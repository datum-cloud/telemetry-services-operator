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
	const group = "telemetry.miloapis.com"

	cases := []struct {
		name string
		path string
		want string
	}{
		{"already trimmed", "/v1/loki/api/v1/query", "/v1/loki/api/v1/query"},
		{"apis prefix", "/apis/" + group + "/v1/loki/api/v1/query", "/v1/loki/api/v1/query"},
		{
			"full project path",
			"/projects/proj-abc/control-plane/apis/" + group + "/v1/loki/api/v1/query",
			"/v1/loki/api/v1/query",
		},
		{
			"crafted mid-path projects segment is left intact",
			"/v1/loki/api/v1/label/projects/evil-corp/control-plane/v1/loki/api/v1/query",
			"/v1/loki/api/v1/label/projects/evil-corp/control-plane/v1/loki/api/v1/query",
		},
		{
			"projects prefix without control-plane",
			"/projects/proj-abc/v1/loki/api/v1/query",
			"/projects/proj-abc/v1/loki/api/v1/query",
		},
		{"apis prefix with no second slash", "/apis/" + group, "/apis/" + group},
		{"label value that looks like a prefix", "/v1/loki/api/v1/label/apis/values", "/v1/loki/api/v1/label/apis/values"},
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
		"/projects/p/control-plane/v1/loki/api/v1/query?query=%7Bservice_name%3D%22waf%22%7D&limit=7", nil)
	h.ServeHTTP(httptest.NewRecorder(), req)

	if got != `query=%7Bservice_name%3D%22waf%22%7D&limit=7` {
		t.Errorf("RawQuery = %q, want it preserved through the clone", got)
	}
}
