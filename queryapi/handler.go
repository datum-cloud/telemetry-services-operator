// SPDX-License-Identifier: AGPL-3.0-only

// Package queryapi implements the HTTP handlers for the telemetry query
// layer's tenant-scoped log and metric query API. See openapi.yaml for the
// contract.
package queryapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"go.datum.net/o11y/queryapi/internal/logql"
	"go.datum.net/o11y/queryapi/internal/miloauth"
	"go.datum.net/o11y/queryapi/internal/storage"
)

// NewHandler wires the query API routes. Log endpoints mimic Loki's HTTP API,
// metric endpoints Prometheus's. Probes and /metrics are operational and are
// deliberately left unwrapped by tracing, metrics, and project resolution.
func NewHandler(logger *slog.Logger, store storage.LogStore, cfg Config) http.Handler {
	h := &handler{logger: logger, store: store, cfg: cfg}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", healthz)
	mux.HandleFunc("GET /readyz", readyz(store, cfg.QueryTimeout))
	mux.Handle("GET /metrics", promhttp.Handler())

	// Patterns are /v1-prefixed to match the path suffix a request carries
	// after "apis/<telemetryGroup>"; openapi.yaml's servers block ends in /v1.
	tenant := http.NewServeMux()
	route(tenant, "GET /v1/loki/api/v1/query", h.lokiQuery)
	route(tenant, "GET /v1/loki/api/v1/query_range", h.lokiQueryRange)
	route(tenant, "GET /v1/loki/api/v1/label", h.lokiLabelNames)
	route(tenant, "GET /v1/loki/api/v1/label/{name}/values", h.lokiLabelValues)
	route(tenant, "GET /v1/loki/api/v1/series", h.lokiSeries)

	// POST-only, form-encoded, matching Grafana's Prometheus datasource.
	route(tenant, "POST /v1/api/v1/query", h.promQuery)
	route(tenant, "POST /v1/api/v1/query_range", h.promQueryRange)
	route(tenant, "GET /v1/api/v1/series", h.promSeries)
	route(tenant, "GET /v1/api/v1/labels", h.promLabelNames)
	route(tenant, "GET /v1/api/v1/label/{name}/values", h.promLabelValues)

	mux.Handle("/", miloauth.Middleware(logger, cfg.TrustProjectHeader, stripProxyPrefixes(tenant)))
	return mux
}

const (
	projectsPrefix   = "/projects/"
	controlPlaneMark = "/control-plane"
	apisPrefix       = "/apis/"
)

// stripProxyPrefixes trims the prefixes Milo's proxy chain may leave on the
// path, so the tenant routes match whichever shape actually arrives.
func stripProxyPrefixes(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		if i := strings.Index(path, projectsPrefix); i >= 0 {
			if j := strings.Index(path[i:], controlPlaneMark+"/"); j >= 0 {
				path = path[i+j+len(controlPlaneMark):]
			}
		}
		if strings.HasPrefix(path, apisPrefix) {
			if j := strings.IndexByte(path[len(apisPrefix):], '/'); j >= 0 {
				path = path[len(apisPrefix)+j:]
			}
		}

		if path == r.URL.Path {
			next.ServeHTTP(w, r)
			return
		}

		r2 := r.Clone(r.Context())
		r2.URL.Path = path
		r2.URL.RawPath = ""
		next.ServeHTTP(w, r2)
	})
}

func route(mux *http.ServeMux, pattern string, handler http.HandlerFunc) {
	mux.Handle(pattern, traceRoute(pattern, instrumentRoute(pattern, handler)))
}

type handler struct {
	logger *slog.Logger
	store  storage.LogStore
	cfg    Config
}

// parseLogQL parses raw, writing a 400 and returning false on failure.
func (h *handler) parseLogQL(w http.ResponseWriter, raw string) (*logql.Query, bool) {
	q, err := logql.Parse(raw)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return nil, false
	}
	return q, true
}

// withTimeout bounds a storage call so a slow backend cannot pin a connection.
func (h *handler) withTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), h.cfg.QueryTimeout)
}

// timeRange resolves start/end, defaulting to the last hour ending at end.
func (h *handler) timeRange(w http.ResponseWriter, r *http.Request) (storage.TimeRange, bool) {
	now := time.Now().UTC()
	end, err := parseTime(r.URL.Query().Get("end"), now, now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return storage.TimeRange{}, false
	}
	start, err := parseTime(r.URL.Query().Get("start"), now, end.Add(-time.Hour))
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return storage.TimeRange{}, false
	}
	if !end.After(start) {
		writeError(w, http.StatusBadRequest, "end must be after start")
		return storage.TimeRange{}, false
	}
	return storage.TimeRange{Start: start, End: end}, true
}

func (h *handler) lokiQuery(w http.ResponseWriter, r *http.Request) {
	q, ok := h.parseLogQL(w, r.URL.Query().Get("query"))
	if !ok {
		return
	}

	now := time.Now().UTC()
	at, err := parseTime(r.URL.Query().Get("time"), now, now)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// An instant log query is a range ending at `time`; Loki looks back over
	// a window rather than at a single nanosecond.
	h.serveLogs(w, r, q, storage.TimeRange{Start: at.Add(-time.Hour), End: at})
}

func (h *handler) lokiQueryRange(w http.ResponseWriter, r *http.Request) {
	q, ok := h.parseLogQL(w, r.URL.Query().Get("query"))
	if !ok {
		return
	}
	tr, ok := h.timeRange(w, r)
	if !ok {
		return
	}
	h.serveLogs(w, r, q, tr)
}

func (h *handler) serveLogs(w http.ResponseWriter, r *http.Request, q *logql.Query, tr storage.TimeRange) {
	ctx, cancel := h.withTimeout(r)
	defer cancel()

	iter, err := h.store.QueryLogs(ctx, storage.LogQuery{
		Matchers:  q.Matchers,
		Filters:   q.Filters,
		Range:     tr,
		Limit:     parseLimit(r.URL.Query().Get("limit"), h.cfg),
		Direction: parseDirection(r.URL.Query().Get("direction")),
	})
	if err != nil {
		h.writeStorageError(w, err)
		return
	}

	streams, err := collectStreams(iter)
	if err != nil {
		h.writeStorageError(w, err)
		return
	}
	writeStreams(w, streams)
}

func (h *handler) lokiLabelNames(w http.ResponseWriter, r *http.Request) {
	tr, ok := h.timeRange(w, r)
	if !ok {
		return
	}
	ctx, cancel := h.withTimeout(r)
	defer cancel()

	names, err := h.store.LabelNames(ctx, tr)
	if err != nil {
		h.writeStorageError(w, err)
		return
	}
	writeData(w, orEmpty(names))
}

func (h *handler) lokiLabelValues(w http.ResponseWriter, r *http.Request) {
	// Validation only: the catalogue is not selector-filtered yet.
	if raw := r.URL.Query().Get("query"); raw != "" {
		if _, ok := h.parseLogQL(w, raw); !ok {
			return
		}
	}
	tr, ok := h.timeRange(w, r)
	if !ok {
		return
	}
	ctx, cancel := h.withTimeout(r)
	defer cancel()

	values, err := h.store.LabelValues(ctx, r.PathValue("name"), tr)
	if err != nil {
		h.writeStorageError(w, err)
		return
	}
	writeData(w, orEmpty(values))
}

func (h *handler) lokiSeries(w http.ResponseWriter, r *http.Request) {
	// openapi.yaml declares match[] required, as Loki's own /series does.
	matches := r.URL.Query()["match[]"]
	if len(matches) == 0 {
		writeError(w, http.StatusBadRequest, "at least one match[] selector is required")
		return
	}

	tr, ok := h.timeRange(w, r)
	if !ok {
		return
	}
	ctx, cancel := h.withTimeout(r)
	defer cancel()

	// Repeated match[] selectors are a union, as in Loki -- one store call per
	// selector, deduped on the way out.
	seen := map[string]bool{}
	out := make([]map[string]string, 0)
	for _, match := range matches {
		q, ok := h.parseLogQL(w, match)
		if !ok {
			return
		}
		series, err := h.store.Series(ctx, q.Matchers, tr)
		if err != nil {
			h.writeStorageError(w, err)
			return
		}
		for _, ls := range series {
			if key := ls.Key(); !seen[key] {
				seen[key] = true
				out = append(out, ls)
			}
		}
	}
	writeData(w, out)
}

func (h *handler) promQuery(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w, "promQuery")
}

func (h *handler) promQueryRange(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w, "promQueryRange")
}

func (h *handler) promSeries(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w, "promSeries")
}

func (h *handler) promLabelNames(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w, "promLabelNames")
}

func (h *handler) promLabelValues(w http.ResponseWriter, _ *http.Request) {
	writeNotImplemented(w, "promLabelValues")
}

func orEmpty(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

// writeStorageError maps storage failures to status codes. Detail is logged,
// not returned, except where the client can act on it.
func (h *handler) writeStorageError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, storage.ErrNoProject):
		writeError(w, http.StatusUnauthorized, "request is not scoped to a project")
	case errors.Is(err, storage.ErrInvalidLimit):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, storage.ErrNotImplemented):
		writeError(w, http.StatusNotImplemented, "not implemented for the configured storage backend")
	case errors.Is(err, context.Canceled):
		// Client disconnected; 499 mirrors Loki/nginx. Not an error worth paging on.
		writeError(w, 499, "client cancelled request")
	case errors.Is(err, context.DeadlineExceeded):
		writeError(w, http.StatusGatewayTimeout, "query timed out")
	default:
		h.logger.Error("storage query failed", "error", err)
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func writeNotImplemented(w http.ResponseWriter, op string) {
	writeError(w, http.StatusNotImplemented, op+" is not yet implemented")
}

// writeError writes the Loki/Prometheus-style error envelope openapi.yaml's
// ErrorResponse describes.
func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}{Status: "error", Error: message})
}
