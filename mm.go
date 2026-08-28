// Package mm provides Prometheus instrumentation middleware for
// http.RoundTrippers, so outgoing HTTP client requests can be observed
// without hand-wiring promhttp's instrumentation helpers at each call site.
package mm

import (
	"net/http"
	"regexp"
	"strings"
	"unicode"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// invalidMetricChars matches any rune not allowed in a Prometheus metric
// name component ([a-zA-Z0-9_]), per the naming rules described at
// https://prometheus.io/docs/concepts/data_model/#metric-names-and-labels.
var invalidMetricChars = regexp.MustCompile(`[^a-zA-Z0-9_]+`)

// RegistererGatherer combines the two capabilities MM needs from a
// Prometheus registry: registering collectors and gathering their metrics.
// *prometheus.Registry satisfies this interface.
type RegistererGatherer interface {
	prometheus.Registerer
	prometheus.Gatherer
}

// MM holds the namespace, subsystem, and registry used to build and
// register the metrics collected by its middleware constructors.
type MM struct {
	// Namespace is prepended to every metric name registered by MM.
	Namespace string
	// Subsystem is inserted between Namespace and each metric's base name.
	Subsystem string
	// Registry is where all metrics created by MM are registered.
	Registry RegistererGatherer
}

// New returns an MM that registers metrics under the given namespace and
// subsystem (both normalized to valid Prometheus name components).
//
// If metricsRegistry is nil, a fresh prometheus.NewRegistry() is used.
// Note this check only catches an untyped nil literal: passing a typed nil
// pointer (e.g. a nil *prometheus.Registry) boxed into the interface will
// bypass it and panic on first use, so always pass a literal nil to opt
// into the default registry.
func New(metricsRegistry RegistererGatherer, namespace, subsystem string) *MM {
	if metricsRegistry == nil {
		metricsRegistry = prometheus.NewRegistry()
	}

	return &MM{
		Namespace: normalizeString(namespace),
		Subsystem: normalizeString(subsystem),
		Registry:  metricsRegistry,
	}
}

// registerOrReuse registers c with mm.Registry. If an equivalent collector
// is already registered (e.g. because an AddClient* method was already
// called on this MM), the existing collector is returned instead of
// panicking, so repeated calls stay idempotent. Any other registration
// error still panics, matching MustRegister's behavior.
func registerOrReuse[T prometheus.Collector](mm *MM, c T) T {
	if err := mm.Registry.Register(c); err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector.(T)
		}
		panic(err)
	}
	return c
}

// AddClientRequestsInFlight wraps next with a gauge tracking the number of
// in-flight requests, registered as "<namespace>_<subsystem>_http_client_in_flight_requests".
func (mm *MM) AddClientRequestsInFlight(next http.RoundTripper) http.RoundTripper {
	metric := registerOrReuse(mm, prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: mm.Namespace,
		Subsystem: mm.Subsystem,
		Name:      "http_client_in_flight_requests",
		Help:      "Total count of in-flight requests for the wrapped http client.",
	}))

	return promhttp.InstrumentRoundTripperInFlight(metric, next)
}

// AddClientRequestsCounter wraps next with a "code"/"method"-labeled
// counter, registered as "<namespace>_<subsystem>_http_client_api_requests_total".
func (mm *MM) AddClientRequestsCounter(next http.RoundTripper) http.RoundTripper {
	metric := registerOrReuse(mm, prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: mm.Namespace,
			Subsystem: mm.Subsystem,
			Name:      "http_client_api_requests_total",
			Help:      "A counter for requests from the wrapped client.",
		},
		[]string{"code", "method"},
	))

	return promhttp.InstrumentRoundTripperCounter(metric, next)
}

// AddClientTrace wraps next with DNS and TLS handshake latency histograms,
// registered as "<namespace>_<subsystem>_http_client_dns_duration_seconds"
// and "<namespace>_<subsystem>_http_client_tls_duration_seconds".
func (mm *MM) AddClientTrace(next http.RoundTripper) http.RoundTripper {
	clientDNSLatencyVec := registerOrReuse(mm, prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mm.Namespace,
			Subsystem: mm.Subsystem,
			Name:      "http_client_dns_duration_seconds",
			Help:      "Trace dns latency histogram.",
			Buckets:   []float64{.005, .01, .025, .05},
		},
		[]string{"event"},
	))

	clientTLSLatencyVec := registerOrReuse(mm, prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mm.Namespace,
			Subsystem: mm.Subsystem,
			Name:      "http_client_tls_duration_seconds",
			Help:      "Trace tls latency histogram.",
			Buckets:   []float64{.05, .1, .25, .5},
		},
		[]string{"event"},
	))

	clientTrace := &promhttp.InstrumentTrace{
		DNSStart: func(t float64) {
			clientDNSLatencyVec.WithLabelValues("dns_start").Observe(t)
		},
		DNSDone: func(t float64) {
			clientDNSLatencyVec.WithLabelValues("dns_done").Observe(t)
		},
		TLSHandshakeStart: func(t float64) {
			clientTLSLatencyVec.WithLabelValues("tls_handshake_start").Observe(t)
		},
		TLSHandshakeDone: func(t float64) {
			clientTLSLatencyVec.WithLabelValues("tls_handshake_done").Observe(t)
		},
	}

	return promhttp.InstrumentRoundTripperTrace(clientTrace, next)
}

// AddClientDuration wraps next with a request duration histogram,
// registered as "<namespace>_<subsystem>_http_client_request_duration_seconds".
func (mm *MM) AddClientDuration(next http.RoundTripper) http.RoundTripper {
	// histVec has no labels, making it a zero-dimensional ObserverVec, as
	// required by promhttp.InstrumentRoundTripperDuration's signature: a
	// plain prometheus.Histogram doesn't implement ObserverVec.
	metric := registerOrReuse(mm, prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: mm.Namespace,
			Subsystem: mm.Subsystem,
			Name:      "http_client_request_duration_seconds",
			Help:      "Trace http request latencies histogram.",
			Buckets:   prometheus.DefBuckets,
		},
		[]string{},
	))

	return promhttp.InstrumentRoundTripperDuration(metric, next)
}

// Build chains middlewares around base, applying them in order such that
// the last-listed middleware ends up outermost and runs first on each
// request (each middleware wraps the result of the previous fold step).
func Build(base http.RoundTripper, middlewares ...func(http.RoundTripper) http.RoundTripper) http.RoundTripper {
	chain := base
	for _, middleware := range middlewares {
		chain = middleware(chain)
	}

	return chain
}

// DefaultMiddlewares chains all of MM's middlewares around baseTransport,
// in this fixed order (outermost first): AddClientDuration, AddClientTrace,
// AddClientRequestsCounter, AddClientRequestsInFlight. Because
// AddClientDuration ends up outermost, its histogram measures total
// latency including the other instrumentation's own overhead.
func (mm *MM) DefaultMiddlewares(baseTransport http.RoundTripper) http.RoundTripper {
	finalMiddleware := Build(
		baseTransport,
		mm.AddClientRequestsInFlight,
		mm.AddClientRequestsCounter,
		mm.AddClientTrace,
		mm.AddClientDuration,
	)
	return finalMiddleware
}

// normalizeString lowercases s and replaces every run of characters
// invalid in a Prometheus metric name component with a single underscore,
// prefixing an underscore if the result would otherwise start with a digit.
func normalizeString(s string) string {
	s = invalidMetricChars.ReplaceAllString(strings.ToLower(s), "_")
	if s != "" && unicode.IsDigit(rune(s[0])) {
		s = "_" + s
	}
	return s
}
