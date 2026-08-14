// SPDX-License-Identifier: AGPL-3.0-only

package queryapi_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.datum.net/o11y/queryapi"
	"go.datum.net/o11y/queryapi/internal/storage"
)

// unreachableStore fails Ping so the /readyz 503 branch actually executes.
type unreachableStore struct{ recordingStore }

func (*unreachableStore) Ping(context.Context) error { return errors.New("dial: connection refused") }

func TestReadyzReportsStorageUnavailable(t *testing.T) {
	srv := httptest.NewServer(queryapi.NewHandler(
		slog.New(slog.DiscardHandler), &unreachableStore{}, queryapi.DefaultConfig()))
	defer srv.Close()

	get := func(t *testing.T, path string) int {
		t.Helper()
		resp, err := http.Get(srv.URL + path)
		if err != nil {
			t.Fatalf("get %s: %v", path, err)
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				t.Logf("close response body: %v", err)
			}
		}()
		return resp.StatusCode
	}

	if code := get(t, "/readyz"); code != http.StatusServiceUnavailable {
		t.Errorf("/readyz status = %d, want 503", code)
	}
	// Liveness must not depend on storage, or an outage restarts the pod.
	if code := get(t, "/healthz"); code != http.StatusOK {
		t.Errorf("/healthz status = %d, want 200", code)
	}
}

var _ storage.LogStore = (*unreachableStore)(nil)
