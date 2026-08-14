// SPDX-License-Identifier: AGPL-3.0-only

package miloauth_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.datum.net/o11y/queryapi/internal/miloauth"
)

func TestMiddlewarePassesProjectThrough(t *testing.T) {
	var got string
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id, ok := miloauth.ProjectID(r.Context())
		if !ok {
			t.Error("no project on context inside handler")
		}
		got = id
		w.WriteHeader(http.StatusOK)
	})

	r := httptest.NewRequest(http.MethodGet, "/v1/loki/api/v1/query", nil)
	r.Header["X-Project-Id"] = []string{"proj-abc"}
	rec := httptest.NewRecorder()

	miloauth.Middleware(slog.New(slog.DiscardHandler), true, next).ServeHTTP(rec, r)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got != "proj-abc" {
		t.Errorf("project = %q, want %q", got, "proj-abc")
	}
}

func TestMiddlewareRejectsWithoutProject(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	r := httptest.NewRequest(http.MethodGet, "/v1/loki/api/v1/query", nil)
	rec := httptest.NewRecorder()

	miloauth.Middleware(slog.New(slog.DiscardHandler), true, next).ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Error("next handler ran despite an unresolved project")
	}

	var body struct {
		Status string `json:"status"`
		Error  string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body.Status != "error" || body.Error == "" {
		t.Errorf("body = %+v, want the Loki error envelope", body)
	}
}

// TestMiddlewareRejectsUntrustedProjectHeader pins the gate: X-Project-Id is
// client-controlled, so it must not scope a request unless opted in.
func TestMiddlewareRejectsUntrustedProjectHeader(t *testing.T) {
	called := false
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { called = true })

	r := httptest.NewRequest(http.MethodGet, "/v1/loki/api/v1/query", nil)
	r.Header["X-Project-Id"] = []string{"proj-abc"}
	rec := httptest.NewRecorder()

	miloauth.Middleware(slog.New(slog.DiscardHandler), false, next).ServeHTTP(rec, r)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
	if called {
		t.Error("next handler ran despite an untrusted project header")
	}
}
