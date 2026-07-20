// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import (
	"net/http"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
)

// traceRoute wraps handler in otelhttp instrumentation, matching the pattern
// milo-apiserver uses (cmd/milo/apiserver/config.go, otelhttp.NewHandler).
// It uses the process-wide global TracerProvider/Propagator — set one via
// otel.SetTracerProvider before serving traffic if traces should actually
// export anywhere; otherwise this is a documented no-op.
func traceRoute(route string, handler http.HandlerFunc) http.Handler {
	return otelhttp.NewHandler(handler, route)
}
