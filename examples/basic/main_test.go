package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

func TestNewMetricsHandler(t *testing.T) {
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	defer target.Close()

	h, reg, err := newMetricsHandler(target.URL)
	if err != nil {
		t.Fatalf("newMetricsHandler(%q) failed: %v", target.URL, err)
	}

	metricsServer := httptest.NewServer(h)
	defer metricsServer.Close()

	resp, err := http.Get(metricsServer.URL)
	if err != nil {
		t.Fatalf("GET %s failed: %v", metricsServer.URL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body failed: %v", err)
	}

	// These three always emit a sample after one request, regardless of
	// target: they don't depend on a real DNS lookup or TLS handshake.
	wantInBody := []string{
		"namespace_subsystem_http_client_in_flight_requests",
		"namespace_subsystem_http_client_api_requests_total",
		"namespace_subsystem_http_client_request_duration_seconds",
	}
	for _, name := range wantInBody {
		if !strings.Contains(string(body), name) {
			t.Errorf("metrics output missing %q, got:\n%s", name, body)
		}
	}

	// DNS/TLS histograms only emit a sample when a real DNS lookup / TLS
	// handshake happens, which a loopback plain-HTTP target does not
	// trigger - so just confirm the collectors are registered, same as
	// mm_test.go's TestAddClientTrace does for the same reason.
	wantRegistered := []string{
		"namespace_subsystem_http_client_dns_duration_seconds",
		"namespace_subsystem_http_client_tls_duration_seconds",
	}
	for _, name := range wantRegistered {
		if _, err := testutil.GatherAndCount(reg, name); err != nil {
			t.Errorf("metric %s not registered: %v", name, err)
		}
	}
}
