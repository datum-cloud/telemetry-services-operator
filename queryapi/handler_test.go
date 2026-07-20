// SPDX-License-Identifier: AGPL-3.0-only

package queryapi_test

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"go.datum.net/o11y/queryapi"
)

// loadSpec parses and validates openapi.yaml, then builds a router that maps
// requests to the documented operations.
func loadSpec(t *testing.T) routers.Router {
	t.Helper()

	loader := openapi3.NewLoader()
	doc, err := loader.LoadFromFile("openapi.yaml")
	if err != nil {
		t.Fatalf("load openapi.yaml: %v", err)
	}
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("openapi.yaml failed its own schema validation: %v", err)
	}
	router, err := gorillamux.NewRouter(doc)
	if err != nil {
		t.Fatalf("build router from openapi.yaml: %v", err)
	}
	return router
}

// TestHandlersConformToSpec drives every documented endpoint against the
// stub handler and validates the response against openapi.yaml. Every
// handler currently returns 501, which isn't listed under any operation's
// explicit status codes — it's covered by the spec's "default" response
// (added for exactly this reason), so this test already checks the one
// thing that's true today: every response, success or not, is Error-shaped
// where the spec says it should be. As handlers grow real behavior, the same
// test starts checking the 200 schemas too, with no changes needed here.
func TestHandlersConformToSpec(t *testing.T) {
	router := loadSpec(t)

	srv := httptest.NewServer(queryapi.NewHandler(slog.New(slog.DiscardHandler)))
	defer srv.Close()

	// The spec's server entry is a template kube-aggregator fills in at
	// runtime (see openapi.yaml's `servers` block); route matching needs a
	// concrete instance of it, not the test server's own URL.
	const specBase = "/projects/test-project/control-plane/apis/telemetry.datumapis.com/v1"

	cases := []struct {
		name  string
		path  string
		query string
	}{
		{"logs", "/logs", `query={service_name="foo"}`},
		{"logs tail", "/logs/tail", `query={service_name="foo"}`},
		{"metrics query", "/metrics/query", "query=cpu_usage"},
		{
			"metrics query_range", "/metrics/query_range",
			"query=cpu_usage&start=2026-01-01T00:00:00Z&end=2026-01-01T01:00:00Z&step=1m",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			realURL := srv.URL + "/v1" + tc.path + "?" + tc.query
			httpReq, err := http.NewRequest(http.MethodGet, realURL, nil)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}

			resp, err := http.DefaultClient.Do(httpReq)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer func() { _ = resp.Body.Close() }()
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}

			specURL := &url.URL{Path: specBase + tc.path, RawQuery: tc.query}
			specReq, err := http.NewRequest(http.MethodGet, specURL.String(), nil)
			if err != nil {
				t.Fatalf("build spec request: %v", err)
			}

			route, pathParams, err := router.FindRoute(specReq)
			if err != nil {
				t.Fatalf("find route for %s: %v", tc.path, err)
			}

			requestValidationInput := &openapi3filter.RequestValidationInput{
				Request:    specReq,
				PathParams: pathParams,
				Route:      route,
			}
			responseValidationInput := &openapi3filter.ResponseValidationInput{
				RequestValidationInput: requestValidationInput,
				Status:                 resp.StatusCode,
				Header:                 resp.Header,
				Body:                   io.NopCloser(bytes.NewReader(body)),
			}
			if err := openapi3filter.ValidateResponse(context.Background(), responseValidationInput); err != nil {
				t.Fatalf("response for %s did not conform to openapi.yaml: %v\nbody: %s", tc.path, err, body)
			}
		})
	}
}
