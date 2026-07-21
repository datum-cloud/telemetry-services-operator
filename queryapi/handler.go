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
// Log endpoints mimic Loki's HTTP API (/loki/api/v1/...); metric endpoints
// mimic vanilla Prometheus's (/api/v1/...) — see openapi.yaml's info
// description for why. Live tail (/loki/api/v1/tail) is a nice-to-have, not
// v1 scope, and isn't wired here yet.
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

	// Route patterns are prefixed with /v1 to match the full path suffix a
	// request has after "apis/<telemetryGroup>" — kube-aggregator forwards
	// the complete original request path unmodified, and openapi.yaml's
	// servers block ends in "/v1" with these as its relative operation
	// paths (so, e.g., "/v1" + "/loki/api/v1/query" below).

	// Loki-mimicking log endpoints.
	route(mux, "GET /v1/loki/api/v1/query", h.lokiQuery)
	route(mux, "GET /v1/loki/api/v1/query_range", h.lokiQueryRange)
	route(mux, "GET /v1/loki/api/v1/label", h.lokiLabelNames)
	route(mux, "GET /v1/loki/api/v1/label/{name}/values", h.lokiLabelValues)
	route(mux, "GET /v1/loki/api/v1/series", h.lokiSeries)

	// Prometheus-mimicking metric endpoints. Query/query_range are POST-only
	// (form-encoded), matching Grafana's Prometheus datasource default
	// request mode — see openapi.yaml.
	route(mux, "POST /v1/api/v1/query", h.promQuery)
	route(mux, "POST /v1/api/v1/query_range", h.promQueryRange)
	route(mux, "GET /v1/api/v1/series", h.promSeries)
	route(mux, "GET /v1/api/v1/labels", h.promLabelNames)
	route(mux, "GET /v1/api/v1/label/{name}/values", h.promLabelValues)

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

// parseLogQL parses raw as the LogQL subset this service accepts. It treats
// the not-yet-implemented parser as distinct from a genuine parse error,
// writing a 400 only for the latter; either way there's no parsed query to
// act on yet, hence the single ok bool rather than returning the query.
func (h *handler) parseLogQL(w http.ResponseWriter, raw string) (ok bool) {
	if raw == "" {
		return true
	}
	if _, err := logql.Parse(raw); err != nil && !errors.Is(err, logql.ErrNotImplemented) {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func (h *handler) lokiQuery(w http.ResponseWriter, r *http.Request) {
	if !h.parseLogQL(w, r.URL.Query().Get("query")) {
		return
	}
	writeNotImplemented(w, "lokiQuery")
}

func (h *handler) lokiQueryRange(w http.ResponseWriter, r *http.Request) {
	if !h.parseLogQL(w, r.URL.Query().Get("query")) {
		return
	}
	writeNotImplemented(w, "lokiQueryRange")
}

func (h *handler) lokiLabelNames(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "lokiLabelNames")
}

func (h *handler) lokiLabelValues(w http.ResponseWriter, r *http.Request) {
	if !h.parseLogQL(w, r.URL.Query().Get("query")) {
		return
	}
	writeNotImplemented(w, "lokiLabelValues")
}

func (h *handler) lokiSeries(w http.ResponseWriter, r *http.Request) {
	for _, match := range r.URL.Query()["match[]"] {
		if !h.parseLogQL(w, match) {
			return
		}
	}
	writeNotImplemented(w, "lokiSeries")
}

func (h *handler) promQuery(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "promQuery")
}

func (h *handler) promQueryRange(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "promQueryRange")
}

func (h *handler) promSeries(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "promSeries")
}

func (h *handler) promLabelNames(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "promLabelNames")
}

func (h *handler) promLabelValues(w http.ResponseWriter, r *http.Request) {
	writeNotImplemented(w, "promLabelValues")
}

func writeNotImplemented(w http.ResponseWriter, op string) {
	writeError(w, http.StatusNotImplemented, op+" is not yet implemented")
}

// writeError writes the shared Loki/Prometheus-style error envelope
// ({"status":"error","error":"..."}) that ErrorResponse in openapi.yaml
// describes.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}{Status: "error", Error: message})
}
