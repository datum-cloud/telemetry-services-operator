// SPDX-License-Identifier: AGPL-3.0-only

package queryapi

import (
	"flag"
	"fmt"
	"time"
)

// Config is the query API server's runtime configuration.
//
// There is deliberately no WriteTimeout: it is an absolute deadline from when
// the request is read, so it would truncate long-lived streaming responses.
// ReadHeaderTimeout bounds slow-header attacks without touching response
// duration.
type Config struct {
	Addr              string
	ReadHeaderTimeout time.Duration
	IdleTimeout       time.Duration

	// Storage selects the backend: "fake" or "clickhouse".
	Storage string

	// FakeRate is the fake backend's lines per second.
	FakeRate float64

	// QueryTimeout bounds a single storage call.
	QueryTimeout time.Duration

	// DefaultLimit and MaxLimit bound returned rows. Requests above MaxLimit
	// are clamped, not rejected, matching Loki.
	DefaultLimit int
	MaxLimit     int
}

// DefaultConfig returns a Config with production-reasonable defaults.
func DefaultConfig() Config {
	return Config{
		Addr:              ":8080",
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       120 * time.Second,
		Storage:           "fake",
		FakeRate:          2,
		QueryTimeout:      30 * time.Second,
		DefaultLimit:      100,
		MaxLimit:          5000,
	}
}

// Validate reports whether the configured limits are usable.
func (c Config) Validate() error {
	if c.DefaultLimit <= 0 {
		return fmt.Errorf("default-limit must be greater than zero, got %d", c.DefaultLimit)
	}
	if c.MaxLimit <= 0 {
		return fmt.Errorf("max-limit must be greater than zero, got %d", c.MaxLimit)
	}
	if c.DefaultLimit > c.MaxLimit {
		return fmt.Errorf("default-limit %d exceeds max-limit %d", c.DefaultLimit, c.MaxLimit)
	}
	return nil
}

// RegisterFlags binds Config's fields to fs.
func RegisterFlags(fs *flag.FlagSet, c *Config) {
	def := DefaultConfig()
	fs.StringVar(&c.Addr, "addr", def.Addr, "address to serve the query API on")
	fs.DurationVar(&c.ReadHeaderTimeout, "read-header-timeout", def.ReadHeaderTimeout,
		"maximum duration for reading request headers")
	fs.DurationVar(&c.IdleTimeout, "idle-timeout", def.IdleTimeout,
		"maximum amount of time to wait for the next request on a keep-alive connection")
	fs.StringVar(&c.Storage, "storage", def.Storage, "storage backend: fake or clickhouse")
	fs.Float64Var(&c.FakeRate, "fake-rate", def.FakeRate, "synthetic log lines per second (fake backend)")
	fs.DurationVar(&c.QueryTimeout, "query-timeout", def.QueryTimeout, "maximum duration of a single storage query")
	fs.IntVar(&c.DefaultLimit, "default-limit", def.DefaultLimit, "default maximum log lines returned")
	fs.IntVar(&c.MaxLimit, "max-limit", def.MaxLimit, "hard ceiling on log lines returned")
}
