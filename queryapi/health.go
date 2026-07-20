// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import "net/http"

// healthz reports whether the process is alive. It never depends on
// downstream state — a liveness probe should only fail when the process
// itself needs restarting.
func healthz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

// readyz reports whether the server is ready to take traffic. Today that is
// equivalent to healthz since there are no downstream dependencies (no
// ClickHouse client, no milo-api call) wired up yet. Once those land, this
// should check them explicitly rather than always report ready.
func readyz(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}
