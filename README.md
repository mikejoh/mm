# mm

[![CI](https://github.com/mikejoh/mm/actions/workflows/go.yml/badge.svg)](https://github.com/mikejoh/mm/actions/workflows/go.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

<p align="center">
<img src="https://github.com/mikejoh/mm/assets/899665/0158ee4b-e5b1-4f70-ae04-82a01217de34" alt="mm" />
</p>


`mm` - Metrics middleware for your Go HTTP Clients!

## Install

`go get github.com/mikejoh/mm`

## Usage

This example will output the client metrics at `http://localhost:8080` we get from the chained metric middlewares:
```
func main() {
    pr := prometheus.NewRegistry()

    mm := mm.New(pr, "namespace", "subsystem")

    c := http.DefaultClient
    c.Transport = mm.DefaultMiddlewares(http.DefaultTransport)
    c.Get("http://www.google.com")

    http.Handle("/metrics", promhttp.HandlerFor(mm.Registry, promhttp.HandlerOpts{}))
    http.ListenAndServe(":8080", nil)
}
```

A runnable version of this example is in [`examples/basic`](examples/basic/main.go):

```
go run github.com/mikejoh/mm/examples/basic
curl localhost:8080/metrics
```

Since `mm` registers metrics on the registry you pass in rather than the
global default, `/metrics` only ever contains `mm`'s own metrics - no
`go_*`/`process_*` noise. Here's what you get out of the box after that one
`c.Get(...)` call:

```
# HELP namespace_subsystem_http_client_api_requests_total A counter for requests from the wrapped client.
# TYPE namespace_subsystem_http_client_api_requests_total counter
namespace_subsystem_http_client_api_requests_total{code="200",method="get"} 1
# HELP namespace_subsystem_http_client_dns_duration_seconds Trace dns latency histogram.
# TYPE namespace_subsystem_http_client_dns_duration_seconds histogram
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_done",le="0.005"} 0
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_done",le="0.01"} 0
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_done",le="0.025"} 1
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_done",le="0.05"} 1
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_done",le="+Inf"} 1
namespace_subsystem_http_client_dns_duration_seconds_sum{event="dns_done"} 0.017868298
namespace_subsystem_http_client_dns_duration_seconds_count{event="dns_done"} 1
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_start",le="0.005"} 1
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_start",le="0.01"} 1
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_start",le="0.025"} 1
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_start",le="0.05"} 1
namespace_subsystem_http_client_dns_duration_seconds_bucket{event="dns_start",le="+Inf"} 1
namespace_subsystem_http_client_dns_duration_seconds_sum{event="dns_start"} 5.1302e-05
namespace_subsystem_http_client_dns_duration_seconds_count{event="dns_start"} 1
# HELP namespace_subsystem_http_client_in_flight_requests Total count of in-flight requests for the wrapped http client.
# TYPE namespace_subsystem_http_client_in_flight_requests gauge
namespace_subsystem_http_client_in_flight_requests 0
# HELP namespace_subsystem_http_client_request_duration_seconds Trace http request latencies histogram.
# TYPE namespace_subsystem_http_client_request_duration_seconds histogram
namespace_subsystem_http_client_request_duration_seconds_bucket{le="0.005"} 0
namespace_subsystem_http_client_request_duration_seconds_bucket{le="0.01"} 0
namespace_subsystem_http_client_request_duration_seconds_bucket{le="0.025"} 0
namespace_subsystem_http_client_request_duration_seconds_bucket{le="0.05"} 1
namespace_subsystem_http_client_request_duration_seconds_bucket{le="0.1"} 1
namespace_subsystem_http_client_request_duration_seconds_bucket{le="0.25"} 1
namespace_subsystem_http_client_request_duration_seconds_bucket{le="0.5"} 1
namespace_subsystem_http_client_request_duration_seconds_bucket{le="1"} 1
namespace_subsystem_http_client_request_duration_seconds_bucket{le="2.5"} 1
namespace_subsystem_http_client_request_duration_seconds_bucket{le="5"} 1
namespace_subsystem_http_client_request_duration_seconds_bucket{le="10"} 1
namespace_subsystem_http_client_request_duration_seconds_bucket{le="+Inf"} 1
namespace_subsystem_http_client_request_duration_seconds_sum 0.045058562
namespace_subsystem_http_client_request_duration_seconds_count 1
# HELP namespace_subsystem_http_client_tls_duration_seconds Trace tls latency histogram.
# TYPE namespace_subsystem_http_client_tls_duration_seconds histogram
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_done",le="0.05"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_done",le="0.1"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_done",le="0.25"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_done",le="0.5"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_done",le="+Inf"} 1
namespace_subsystem_http_client_tls_duration_seconds_sum{event="tls_handshake_done"} 0.037116539
namespace_subsystem_http_client_tls_duration_seconds_count{event="tls_handshake_done"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_start",le="0.05"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_start",le="0.1"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_start",le="0.25"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_start",le="0.5"} 1
namespace_subsystem_http_client_tls_duration_seconds_bucket{event="tls_handshake_start",le="+Inf"} 1
namespace_subsystem_http_client_tls_duration_seconds_sum{event="tls_handshake_start"} 0.020124704
namespace_subsystem_http_client_tls_duration_seconds_count{event="tls_handshake_start"} 1
```
