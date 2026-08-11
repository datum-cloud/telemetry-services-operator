// SPDX-License-Identifier: AGPL-3.0-only

package miloauth

import (
	"net/http"
	"net/url"
	"regexp"
	"strings"
)

const (
	extraHeaderPrefix = "x-remote-extra-"
	parentTypeExtra   = "iam.miloapis.com/parent-type"
	parentNameExtra   = "iam.miloapis.com/parent-name"
	projectHeader     = "X-Project-Id"
)

// validProjectID guards the value before it reaches a query parameter or a
// ClickHouse setting.
var validProjectID = regexp.MustCompile(`^[a-z0-9]([-a-z0-9.]*[a-z0-9])?$`)

// Resolve returns the project id for r and its source. queryapi is reached
// only through Milo's authenticating proxy chain, which forwards identity as
// X-Remote-* extras; that delegated identity is the sole production source.
// trustHeader (local dev only) additionally accepts the client-controlled
// X-Project-Id header.
func Resolve(r *http.Request, trustHeader bool) (id string, source string, ok bool) {
	type candidateSource struct {
		source string
		fn     func(*http.Request) string
	}

	candidates := []candidateSource{{"remote-extra", fromUserExtras}}
	if trustHeader {
		candidates = append(candidates, candidateSource{"header", fromProjectHeader})
	}

	for _, candidate := range candidates {
		if id := candidate.fn(r); valid(id) {
			return id, candidate.source, true
		}
	}
	return "", "", false
}

func valid(id string) bool {
	return id != "" && len(id) <= 253 && validProjectID.MatchString(id)
}

// fromUserExtras reads the parent project out of the delegated user extras.
// Keys are lowercased and percent-decoded the way
// k8s.io/apiserver/pkg/authentication/request/headerrequest recovers them, so
// header canonicalisation cannot break the lookup.
func fromUserExtras(r *http.Request) string {
	extras := map[string]string{}
	for name, values := range r.Header {
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, extraHeaderPrefix) || len(values) == 0 {
			continue
		}
		key, err := url.PathUnescape(lower[len(extraHeaderPrefix):])
		if err != nil {
			key = lower[len(extraHeaderPrefix):]
		}
		extras[key] = values[0]
	}

	// Milo sets the same extras for org parents, so the type check keeps an
	// org id from being read as a project id.
	if extras[parentTypeExtra] != "Project" {
		return ""
	}
	return extras[parentNameExtra]
}

func fromProjectHeader(r *http.Request) string {
	return r.Header.Get(projectHeader)
}
