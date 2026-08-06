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
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers"
	"github.com/getkin/kin-openapi/routers/gorillamux"

	"go.datum.net/o11y/queryapi"
	"go.datum.net/o11y/queryapi/internal/storage/fake"
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

// TestHandlersConformToSpec drives every documented endpoint and validates the response against openapi.yaml.
func TestHandlersConformToSpec(t *testing.T) {
	router := loadSpec(t)

	cfg := queryapi.DefaultConfig()
	srv := httptest.NewServer(queryapi.NewHandler(slog.New(slog.DiscardHandler), fake.New(2), cfg))
	defer srv.Close()

	// The spec's server entry is a template kube-aggregator fills in at
	// runtime (see openapi.yaml's `servers` block); route matching needs a
	// concrete instance of it, not the test server's own URL.
	const specBase = "/projects/test-project/control-plane/apis/telemetry.datumapis.com/v1"

	cases := []struct {
		name   string
		method string
		path   string
		query  string
		form   url.Values
	}{
		{"loki query", http.MethodGet, "/loki/api/v1/query", `query={service_name="foo"}`, nil},
		{
			"loki query_range", http.MethodGet, "/loki/api/v1/query_range",
			`query={service_name="foo"}&start=2026-01-01T00:00:00Z&end=2026-01-01T01:00:00Z`, nil,
		},
		{"loki label names", http.MethodGet, "/loki/api/v1/label", "", nil},
		{"loki label values", http.MethodGet, "/loki/api/v1/label/severity/values", "", nil},
		{"loki series", http.MethodGet, "/loki/api/v1/series", `match[]={service_name="foo"}`, nil},
		{"prom query", http.MethodPost, "/api/v1/query", "", url.Values{"query": {"cpu_usage"}}},
		{
			"prom query_range", http.MethodPost, "/api/v1/query_range", "",
			url.Values{"query": {"cpu_usage"}, "start": {"2026-01-01T00:00:00Z"}, "end": {"2026-01-01T01:00:00Z"}, "step": {"1m"}},
		},
		{"prom series", http.MethodGet, "/api/v1/series", `match[]=cpu_usage{service="foo"}`, nil},
		{"prom label names", http.MethodGet, "/api/v1/labels", "", nil},
		{"prom label values", http.MethodGet, "/api/v1/label/service/values", "", nil},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			realURL := srv.URL + "/v1" + tc.path
			if tc.query != "" {
				realURL += "?" + tc.query
			}

			var body io.Reader
			if tc.form != nil {
				body = strings.NewReader(tc.form.Encode())
			}
			httpReq, err := http.NewRequest(tc.method, realURL, body)
			if err != nil {
				t.Fatalf("build request: %v", err)
			}
			if tc.form != nil {
				httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
			}
			httpReq.Header.Set("X-Project-Id", "test-project")

			resp, err := http.DefaultClient.Do(httpReq)
			if err != nil {
				t.Fatalf("do request: %v", err)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					t.Logf("close response body: %v", err)
				}
			}()
			respBody, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}

			specURL := &url.URL{Path: specBase + tc.path, RawQuery: tc.query}
			var specBody io.Reader
			if tc.form != nil {
				specBody = strings.NewReader(tc.form.Encode())
			}
			specReq, err := http.NewRequest(tc.method, specURL.String(), specBody)
			if err != nil {
				t.Fatalf("build spec request: %v", err)
			}
			if tc.form != nil {
				specReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
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
				Body:                   io.NopCloser(bytes.NewReader(respBody)),
			}
			if err := openapi3filter.ValidateResponse(context.Background(), responseValidationInput); err != nil {
				t.Fatalf("response for %s did not conform to openapi.yaml: %v\nbody: %s", tc.path, err, respBody)
			}
		})
	}
}

func TestUnscopedRequestIsUnauthorized(t *testing.T) {
	cfg := queryapi.DefaultConfig()
	srv := httptest.NewServer(queryapi.NewHandler(slog.New(slog.DiscardHandler), fake.New(2), cfg))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "/v1/loki/api/v1/query?query=%7Bservice_name%3D%22waf%22%7D")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Logf("close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", resp.StatusCode)
	}
}
