// SPDX-License-Identifier: AGPL-3.0-only

// Package queryapi implements the HTTP handlers for the telemetry query
// layer's tenant-scoped log and metric query API. Handler bodies are stubs:
// this scaffold exists so the API contract (openapi.yaml) can be
// iterated on with UI/UX engineers ahead of the ClickHouse-backed
// implementation.
package queryapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.datum.net/telemetry-services-operator/queryapi/internal/logql"
)

// NewHandler wires the query API routes. project_id scoping is expected to
// arrive via request context, injected upstream by milo-api/kube-aggregator
// per the query-layer design's API registration section — it is not read
// from any parameter here.
//
// /healthz, /readyz, and /metrics are operational endpoints, not part of the
// tenant-facing contract in openapi.yaml, and are deliberately left
// unwrapped by tracing/request metrics to avoid instrumenting the
// instrumentation.
func NewHandler(logger *slog.Logger) http.Handler {
	h := &handler{logger: logger}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz)
	mux.Handle("GET /metrics", promhttp.Handler())

	route(mux, "GET /v1/logs", h.queryLogs)
	route(mux, "GET /v1/logs/tail", h.tailLogs)
	route(mux, "GET /v1/metrics/query", h.queryMetricsInstant)
	route(mux, "GET /v1/metrics/query_range", h.queryMetricsRange)
	return mux
}

// route registers handler for pattern wrapped in request metrics and tracing,
// so every tenant-facing endpoint gets both without repeating the wiring at
// each call site.
func route(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	mux.Handle(pattern, traceRoute(pattern, instrumentRoute(pattern, handler)))
}

type handler struct {
	logger *slog.Logger
}

func (h *handler) queryLogs(w http.ResponseWriter, r *http.Request) {
	query, err := logql.Parse(r.URL.Query().Get("query"))
	if err != nil && !errors.Is(err, logql.ErrNotImplemented) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = query // translation to SQL and ClickHouse execution: not yet implemented
	writeNotImplemented(w, "queryLogs")
}

func (h *handler) tailLogs(w http.ResponseWriter, r *http.Request) {
	query, err := logql.Parse(r.URL.Query().Get("query"))
	if err != nil && !errors.Is(err, logql.ErrNotImplemented) {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	_ = query // streaming ClickHouse cursor poll: not yet implemented
	writeNotImplemented(w, "tailLogs")
}

func (h *handler) queryMetricsInstant(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "queryMetricsInstant")
}

func (h *handler) queryMetricsRange(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "queryMetricsRange")
}

func writeNotImplemented(w http.ResponseWriter, op string) {
	writeError(w, http.StatusNotImplemented, op+" is not yet implemented")
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Message string `json:"message"`
	}{Message: message})
}
