// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import (
	"context"
	"net/http"
	"time"

	"go.datum.net/o11y/queryapi/internal/storage"
)

// healthz reports process liveness and never depends on downstream state.
func healthz(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// readyz reports whether the storage backend is reachable.
func readyz(store storage.LogStore, timeout time.Duration) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), timeout)
		defer cancel()

		if err := store.Ping(ctx); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte("storage unavailable"))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
