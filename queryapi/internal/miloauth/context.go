// SPDX-License-Identifier: AGPL-3.0-only

// Package miloauth resolves the project a request is scoped to from the
// identity Milo forwards.
package miloauth

import "context"

type ctxKey struct{}

// WithProject returns ctx carrying the resolved project id.
func WithProject(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// ProjectID returns the project id on ctx, if any.
func ProjectID(ctx context.Context) (string, bool) {
	id, ok := ctx.Value(ctxKey{}).(string)
	return id, ok && id != ""
}
