// SPDX-License-Identifier: AGPL-3.0-only

package miloauth_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"go.datum.net/o11y/queryapi/internal/miloauth"
)

func TestResolve(t *testing.T) {
	cases := []struct {
		name       string
		path       string
		headers    map[string]string
		wantID     string
		wantSource string
		wantOK     bool
	}{
		{
			name: "user extras with project parent",
			path: "/v1/loki/api/v1/query",
			headers: map[string]string{
				"X-Remote-Extra-Iam.miloapis.com%2Fparent-type": "Project",
				"X-Remote-Extra-Iam.miloapis.com%2Fparent-name": "proj-abc",
			},
			wantID: "proj-abc", wantSource: "remote-extra", wantOK: true,
		},
		{
			name: "user extras ignored when parent is an organization",
			path: "/v1/loki/api/v1/query",
			headers: map[string]string{
				"X-Remote-Extra-Iam.miloapis.com%2Fparent-type": "Organization",
				"X-Remote-Extra-Iam.miloapis.com%2Fparent-name": "org-abc",
			},
			wantOK: false,
		},
		{
			name:    "plain project header",
			path:    "/v1/loki/api/v1/query",
			headers: map[string]string{"X-Project-Id": "proj-def"},
			wantID:  "proj-def", wantSource: "header", wantOK: true,
		},
		{
			name:   "path",
			path:   "/projects/proj-ghi/control-plane/apis/telemetry.miloapis.com/v1/loki/api/v1/query",
			wantID: "proj-ghi", wantSource: "path", wantOK: true,
		},
		{
			name:   "nothing to resolve",
			path:   "/v1/loki/api/v1/query",
			wantOK: false,
		},
		{
			name:    "invalid id rejected",
			path:    "/v1/loki/api/v1/query",
			headers: map[string]string{"X-Project-Id": "Not A Project!"},
			wantOK:  false,
		},
		{
			name: "extras take precedence over header",
			path: "/v1/loki/api/v1/query",
			headers: map[string]string{
				"X-Remote-Extra-Iam.miloapis.com%2Fparent-type": "Project",
				"X-Remote-Extra-Iam.miloapis.com%2Fparent-name": "from-extras",
				"X-Project-Id":                                  "from-header",
			},
			wantID: "from-extras", wantSource: "remote-extra", wantOK: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, tc.path, nil)
			for k, v := range tc.headers {
				// Set canonicalises; the escaped extras keys must survive as-is.
				r.Header[k] = []string{v}
			}

			id, source, ok := miloauth.Resolve(r)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v (id=%q source=%q)", ok, tc.wantOK, id, source)
			}
			if !tc.wantOK {
				return
			}
			if id != tc.wantID || source != tc.wantSource {
				t.Errorf("got (%q, %q), want (%q, %q)", id, source, tc.wantID, tc.wantSource)
			}
		})
	}
}
