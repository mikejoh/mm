package mm

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNormalizeString(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "lowercasing", in: "Namespace", want: "namespace"},
		{name: "hyphen becomes underscore", in: "my-namespace", want: "my_namespace"},
		{name: "empty string", in: "", want: ""},
		{name: "already valid is unchanged", in: "already_ok", want: "already_ok"},
		{name: "space and dot are replaced", in: "my namespace.foo", want: "my_namespace_foo"},
		{name: "run of invalid chars collapses to one underscore", in: "UPPER--DASH", want: "upper_dash"},
		{name: "leading digit gets an underscore prefix", in: "123abc", want: "_123abc"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeString(tt.in); got != tt.want {
				t.Errorf("normalizeString(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestNew(t *testing.T) {
	t.Run("nil registry defaults to a working registry", func(t *testing.T) {
		m := New(nil, "Namespace", "Sub-System")

		if m.Namespace != "namespace" {
			t.Errorf("Namespace = %q, want %q", m.Namespace, "namespace")
		}
		if m.Subsystem != "sub_system" {
			t.Errorf("Subsystem = %q, want %q", m.Subsystem, "sub_system")
		}
		if m.Registry == nil {
			t.Fatal("Registry is nil, want a default registry")
		}
		if err := m.Registry.Register(prometheus.NewGauge(prometheus.GaugeOpts{Name: "probe"})); err != nil {
			t.Errorf("Registry is not usable: %v", err)
		}
	})

	t.Run("custom registry is stored by identity", func(t *testing.T) {
		reg := prometheus.NewRegistry()
		m := New(reg, "ns", "sub")

		if m.Registry != reg {
			t.Error("Registry does not match the registry passed to New")
		}
	})

	t.Run("empty namespace and subsystem do not panic", func(t *testing.T) {
		m := New(nil, "", "")

		if m.Namespace != "" || m.Subsystem != "" {
			t.Errorf("Namespace/Subsystem = %q/%q, want empty/empty", m.Namespace, m.Subsystem)
		}
	})
}

// roundTripperFunc records its own name into calls before delegating to
// next, used to verify Build's composition order.
type roundTripperFunc struct {
	name  string
	calls *[]string
	next  http.RoundTripper
}

func (r roundTripperFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	*r.calls = append(*r.calls, r.name)
	return r.next.RoundTrip(req)
}

func namedMiddleware(name string, calls *[]string) func(http.RoundTripper) http.RoundTripper {
	return func(next http.RoundTripper) http.RoundTripper {
		return roundTripperFunc{name: name, calls: calls, next: next}
	}
}

func TestBuild(t *testing.T) {
	t.Run("zero middlewares returns base unchanged", func(t *testing.T) {
		if got := Build(http.DefaultTransport); got != http.RoundTripper(http.DefaultTransport) {
			t.Errorf("Build(base) = %v, want base unchanged", got)
		}
	})

	t.Run("nil base with zero middlewares returns nil", func(t *testing.T) {
		if got := Build(nil); got != nil {
			t.Errorf("Build(nil) = %v, want nil", got)
		}
	})

	t.Run("last-listed middleware runs first", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
		defer server.Close()

		var calls []string
		base := roundTripperFunc{name: "base", calls: &calls, next: http.DefaultTransport}
		transport := Build(base, namedMiddleware("A", &calls), namedMiddleware("B", &calls))

		client := &http.Client{Transport: transport}
		if _, err := client.Get(server.URL); err != nil {
			t.Fatalf("request failed: %v", err)
		}

		want := []string{"B", "A", "base"}
		if len(calls) != len(want) {
			t.Fatalf("calls = %v, want %v", calls, want)
		}
		for i := range want {
			if calls[i] != want[i] {
				t.Errorf("calls = %v, want %v", calls, want)
				break
			}
		}
	})
}

func TestAddClientRequestsCounter(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/fail") {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer server.Close()

	reg := prometheus.NewRegistry()
	m := New(reg, "testns", "testsub")
	client := &http.Client{Transport: m.AddClientRequestsCounter(http.DefaultTransport)}

	if _, err := client.Get(server.URL); err != nil {
		t.Fatalf("request failed: %v", err)
	}
	if _, err := client.Get(server.URL + "/fail"); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	want := `
# HELP testns_testsub_http_client_api_requests_total A counter for requests from the wrapped client.
# TYPE testns_testsub_http_client_api_requests_total counter
testns_testsub_http_client_api_requests_total{code="200",method="get"} 1
testns_testsub_http_client_api_requests_total{code="500",method="get"} 1
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "testns_testsub_http_client_api_requests_total"); err != nil {
		t.Error(err)
	}
}

func TestAddClientDuration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	reg := prometheus.NewRegistry()
	m := New(reg, "testns", "testsub")
	client := &http.Client{Transport: m.AddClientDuration(http.DefaultTransport)}

	if _, err := client.Get(server.URL); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	count, err := testutil.GatherAndCount(reg, "testns_testsub_http_client_request_duration_seconds")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("GatherAndCount = %d, want 1 (a single zero-label histogram)", count)
	}
}

func TestAddClientRequestsInFlight(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	reg := prometheus.NewRegistry()
	m := New(reg, "testns", "testsub")
	client := &http.Client{Transport: m.AddClientRequestsInFlight(http.DefaultTransport)}

	// A same-goroutine request always completes before Gather runs, so this
	// only verifies the gauge is registered and settles back to 0 - it
	// cannot observe the in-flight value of 1 mid-request.
	if _, err := client.Get(server.URL); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	want := `
# HELP testns_testsub_http_client_in_flight_requests Total count of in-flight requests for the wrapped http client.
# TYPE testns_testsub_http_client_in_flight_requests gauge
testns_testsub_http_client_in_flight_requests 0
`
	if err := testutil.GatherAndCompare(reg, strings.NewReader(want), "testns_testsub_http_client_in_flight_requests"); err != nil {
		t.Error(err)
	}
}

func TestAddClientTrace(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	reg := prometheus.NewRegistry()
	m := New(reg, "testns", "testsub")
	client := &http.Client{Transport: m.AddClientTrace(http.DefaultTransport)}

	if _, err := client.Get(server.URL); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	// The server URL is a bare IP literal, so the client makes no real DNS
	// lookup or TLS handshake - both histograms register but may
	// legitimately gather zero observations here. This only asserts they
	// are registered under the expected names.
	for _, name := range []string{
		"testns_testsub_http_client_dns_duration_seconds",
		"testns_testsub_http_client_tls_duration_seconds",
	} {
		if _, err := testutil.GatherAndCount(reg, name); err != nil {
			t.Errorf("metric %s not registered: %v", name, err)
		}
	}
}

func TestDefaultMiddlewares(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	reg := prometheus.NewRegistry()
	m := New(reg, "testns", "testsub")
	client := &http.Client{Transport: m.DefaultMiddlewares(http.DefaultTransport)}

	if _, err := client.Get(server.URL); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	for _, name := range []string{
		"testns_testsub_http_client_in_flight_requests",
		"testns_testsub_http_client_api_requests_total",
		"testns_testsub_http_client_dns_duration_seconds",
		"testns_testsub_http_client_tls_duration_seconds",
		"testns_testsub_http_client_request_duration_seconds",
	} {
		if _, err := testutil.GatherAndCount(reg, name); err != nil {
			t.Errorf("metric %s not registered: %v", name, err)
		}
	}
}

func TestAddClientMethods_ReusableAcrossRoundTrippers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer server.Close()

	reg := prometheus.NewRegistry()
	m := New(reg, "testns", "testsub")

	// Instrumenting two different RoundTrippers from the same MM used to
	// panic on the second call via MustRegister's duplicate-collector
	// panic. It must now succeed and share the same underlying metrics.
	clientA := &http.Client{Transport: m.DefaultMiddlewares(http.DefaultTransport)}
	clientB := &http.Client{Transport: m.DefaultMiddlewares(http.DefaultTransport)}

	if _, err := clientA.Get(server.URL); err != nil {
		t.Fatalf("request via clientA failed: %v", err)
	}
	if _, err := clientB.Get(server.URL); err != nil {
		t.Fatalf("request via clientB failed: %v", err)
	}

	count, err := testutil.GatherAndCount(reg, "testns_testsub_http_client_api_requests_total")
	if err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("GatherAndCount = %d, want 1 (both clients share one counter collector)", count)
	}
}
