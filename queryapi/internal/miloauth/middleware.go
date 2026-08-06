// SPDX-License-Identifier: AGPL-3.0-only

package miloauth

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	projectSourceTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "queryapi_project_source_total",
		Help: "Requests by the source the project id was resolved from.",
	}, []string{"source"})

	loggedSources sync.Map
)

// Middleware resolves the request's project onto the context, rejecting
// requests it cannot scope. Wrap only tenant-facing routes: probes and
// /metrics carry no identity.
func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, source, ok := Resolve(r)
		if !ok {
			projectSourceTotal.WithLabelValues("none").Inc()
			writeUnauthorized(w)
			return
		}

		projectSourceTotal.WithLabelValues(source).Inc()
		if _, seen := loggedSources.LoadOrStore(source, true); !seen {
			logger.Info("resolved project", "source", source, "project", id)
		}

		next.ServeHTTP(w, r.WithContext(WithProject(r.Context(), id)))
	})
}

func writeUnauthorized(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}{Status: "error", Error: "request is not scoped to a project"})
}
