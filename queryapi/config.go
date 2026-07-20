// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import (
	"flag"
	"time"
)

// Config holds the query API server's runtime configuration. It is
// deliberately small today — the addr and timeouts needed to run an
// http.Server — and is expected to grow (ClickHouse DSN, milo-api trust
// settings, tracing exporter endpoint) as the stub handlers gain real
// implementations.
//
// There is deliberately no WriteTimeout here. http.Server.WriteTimeout is an
// absolute deadline from when the request is read, applied to every write on
// the connection — it would silently truncate /v1/logs/tail, which is a
// long-lived stream by design. ReadHeaderTimeout bounds the slow-header
// attack surface without touching response duration.
type Config struct {
	// Addr is the address the HTTP server listens on, e.g. ":8080".
	Addr string

	// ReadHeaderTimeout bounds how long reading request headers may take.
	ReadHeaderTimeout time.Duration

	// IdleTimeout bounds how long a keep-alive connection may sit idle
	// between requests.
	IdleTimeout time.Duration
}

// DefaultConfig returns a Config with production-reasonable defaults.
func DefaultConfig() Config {
	return Config{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
}

// RegisterFlags binds Config's fields to fs, defaulting to DefaultConfig's
// values where c is the zero value.
func RegisterFlags(fs *flag.FlagSet, c *Config) {
	def := DefaultConfig()
	fs.StringVar(&c.Addr, "addr", def.Addr, "address to serve the query API on")
	fs.DurationVar(&c.ReadHeaderTimeout, "read-header-timeout", def.ReadHeaderTimeout,
		"maximum duration for reading request headers")
	fs.DurationVar(&c.IdleTimeout, "idle-timeout", def.IdleTimeout,
		"maximum amount of time to wait for the next request on a keep-alive connection")
}
