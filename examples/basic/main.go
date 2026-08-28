// Command basic is a runnable version of the README usage example. It wires
// mm's default middlewares into an http.Client, makes one real request so
// the metrics have data, and serves the result on /metrics.
package main

import (
	"log"
	"net/http"

	"github.com/mikejoh/mm"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// newMetricsHandler wires mm's default middlewares into an http.Client,
// makes one request to target so the metrics have data, and returns the
// resulting /metrics handler along with the registry backing it.
func newMetricsHandler(target string) (http.Handler, *prometheus.Registry, error) {
	pr := prometheus.NewRegistry()
	mmw := mm.New(pr, "namespace", "subsystem")

	c := &http.Client{Transport: mmw.DefaultMiddlewares(http.DefaultTransport)}
	if _, err := c.Get(target); err != nil {
		return nil, nil, err
	}

	return promhttp.HandlerFor(mmw.Registry, promhttp.HandlerOpts{}), pr, nil
}

func main() {
	h, _, err := newMetricsHandler("https://example.com")
	if err != nil {
		log.Fatal(err)
	}

	http.Handle("/metrics", h)
	log.Println("serving /metrics on :8080, try: curl localhost:8080/metrics")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
