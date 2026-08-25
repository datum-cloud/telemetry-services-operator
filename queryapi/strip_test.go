// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestStripProxyPrefixes pins the anchoring directly. An end-to-end test cannot
// cover this: miloauth's own anchored check rejects a crafted path first, so a
// regression here would stay invisible at the HTTP layer.
func TestStripProxyPrefixes(t *testing.T) {
	cases := []struct {
		name string
		path string
		want string
	}{
		{"already trimmed", "/v1alpha1/loki/api/v1/query", "/v1alpha1/loki/api/v1/query"},

		// Grafana omits the datasource URL's subpath, so the bare Loki form
		// must normalise onto the /v1alpha1 routes.
		{"bare loki path gains /v1alpha1", "/loki/api/v1/query", "/v1alpha1/loki/api/v1/query"},
		{"bare loki labels gains /v1alpha1", "/loki/api/v1/labels", "/v1alpha1/loki/api/v1/labels"},
		{
			"bare loki label values gains /v1alpha1",
			"/loki/api/v1/label/severity/values",
			"/v1alpha1/loki/api/v1/label/severity/values",
		},
		{"already /v1alpha1-prefixed is untouched", "/v1alpha1/loki/api/v1/query", "/v1alpha1/loki/api/v1/query"},
		{"non-loki path is untouched", "/v1alpha1/api/v1/query", "/v1alpha1/api/v1/query"},

		// Anchoring: a bare /loki/api/v1 segment is only a route marker when it
		// leads the path. As a later label-name segment it is client-controlled
		// data and must survive untouched.
		{
			"mid-path bare loki segment is not normalised",
			"/v1alpha1/loki/api/v1/label/loki/api/v1/query/values",
			"/v1alpha1/loki/api/v1/label/loki/api/v1/query/values",
		},

		// The /projects/.../control-plane and /apis/{group} prefixes are
		// consumed by Milo's ProjectRouter before the request reaches queryapi,
		// so they are not rewritten here; a stray leading prefix is left alone
		// rather than half-normalised.
		{
			"leading project prefix is left intact",
			"/projects/p/control-plane/loki/api/v1/query",
			"/projects/p/control-plane/loki/api/v1/query",
		},
		{
			"leading apis prefix is left intact",
			"/apis/telemetry.miloapis.com/loki/api/v1/query",
			"/apis/telemetry.miloapis.com/loki/api/v1/query",
		},
		{
			"mid-path projects segment is left intact",
			"/v1alpha1/loki/api/v1/label/projects/evil-corp/control-plane/v1alpha1/loki/api/v1/query",
			"/v1alpha1/loki/api/v1/label/projects/evil-corp/control-plane/v1alpha1/loki/api/v1/query",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got string
			h := stripProxyPrefixes(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				got = r.URL.Path
			}))

			req := httptest.NewRequest(http.MethodGet, tc.path+"?query=x&limit=1", nil)
			h.ServeHTTP(httptest.NewRecorder(), req)

			if got != tc.want {
				t.Errorf("path = %q, want %q", got, tc.want)
			}
			if req.URL.Path != tc.path {
				t.Errorf("original request mutated: %q, want %q", req.URL.Path, tc.path)
			}
		})
	}
}

// TestStripProxyPrefixesPreservesQuery guards the clone: rewriting the path
// must not drop the query string the handlers parse.
func TestStripProxyPrefixesPreservesQuery(t *testing.T) {
	var got string
	h := stripProxyPrefixes(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		got = r.URL.RawQuery
	}))

	req := httptest.NewRequest(http.MethodGet,
		"/loki/api/v1/query?query=%7Bservice_name%3D%22waf%22%7D&limit=7", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != `query=%7Bservice_name%3D%22waf%22%7D&limit=7` {
		t.Errorf("RawQuery = %q, want it preserved through the clone", got)
	}
}
