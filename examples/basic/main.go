// Command basic is a runnable version of the README usage example. It wires
// mm's default middlewares into http.DefaultClient, makes one real request
// so the metrics have data, and serves the result on /metrics.
package main

import (
	"log"
	"net/http"

	"github.com/mikejoh/mm"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

func main() {
	pr := prometheus.NewRegistry()

	mmw := mm.New(pr, "namespace", "subsystem")

	c := http.DefaultClient
	c.Transport = mmw.DefaultMiddlewares(http.DefaultTransport)
	if _, err := c.Get("https://example.com"); err != nil {
		log.Fatal(err)
	}

	http.Handle("/metrics", promhttp.HandlerFor(mmw.Registry, promhttp.HandlerOpts{}))
	log.Println("serving /metrics on :8080, try: curl localhost:8080/metrics")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
