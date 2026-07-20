// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	requestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "queryapi_http_requests_total",
		Help: "Total HTTP requests handled, by route and status code.",
	}, []string{"route", "code"})

	requestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "queryapi_http_request_duration_seconds",
		Help:    "HTTP request latency in seconds, by route.",
		Buckets: prometheus.DefBuckets,
	}, []string{"route"})
)

// instrumentRoute wraps handler with request-count and latency metrics
// labeled by route — the registered mux pattern (e.g. "GET /v1/logs"), not
// the raw request path, so per-project paths don't create unbounded label
// cardinality.
func instrumentRoute(route string, handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		handler(rec, r)
		requestDuration.WithLabelValues(route).Observe(time.Since(start).Seconds())
		requestsTotal.WithLabelValues(route, strconv.Itoa(rec.status)).Inc()
	}
}

// statusRecorder captures the status code written by a handler so it can be
// reported after the fact; http.ResponseWriter has no getter for it.
type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}
