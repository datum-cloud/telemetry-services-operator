// SPDX-License-Identifier: AGPL-3.0-only

// Package clickhouse is the ClickHouse-backed storage.LogStore. It connects
// and reports health; LogQL-to-SQL translation is not implemented yet.
package clickhouse

import (
	"context"
	"fmt"

	ch "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"

	"go.datum.net/o11y/queryapi/internal/logql"
	"go.datum.net/o11y/queryapi/internal/miloauth"
	"go.datum.net/o11y/queryapi/internal/storage"
)

// Store queries logs from ClickHouse.
type Store struct {
	conn driver.Conn
}

// New opens a lazy connection; nothing is dialled until Ping or a query. TLS
// is mandatory: an unloadable certificate is an error, never a silent
// downgrade to an unencrypted connection.
func New(cfg Config) (*Store, error) {
	tlsCfg, err := cfg.tlsConfig()
	if err != nil {
		return nil, err
	}

	conn, err := ch.Open(&ch.Options{
		Addr: []string{fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)},
		Auth: ch.Auth{Database: cfg.Database, Username: cfg.User},
		TLS:  tlsCfg,
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse: open: %w", err)
	}
	return &Store{conn: conn}, nil
}

// newStoreWithConn exists for tests, which cannot call New without real
// certificate files on disk.
func newStoreWithConn(conn driver.Conn) *Store {
	return &Store{conn: conn}
}

// Close releases the connection pool.
func (s *Store) Close() error { return s.conn.Close() }

func (s *Store) Ping(ctx context.Context) error { return s.conn.Ping(ctx) }

// requireProject enforces tenancy at the backend, so a handler bug cannot
// issue an unscoped query.
func requireProject(ctx context.Context) error {
	if _, ok := miloauth.ProjectID(ctx); !ok {
		return storage.ErrNoProject
	}
	return nil
}

func (s *Store) QueryLogs(ctx context.Context, q storage.LogQuery) (storage.LogIterator, error) {
	if err := requireProject(ctx); err != nil {
		return nil, err
	}
	if q.Limit <= 0 {
		return nil, storage.ErrInvalidLimit
	}
	return nil, storage.ErrNotImplemented
}

func (s *Store) LabelNames(ctx context.Context, _ storage.TimeRange) ([]string, error) {
	if err := requireProject(ctx); err != nil {
		return nil, err
	}
	return nil, storage.ErrNotImplemented
}

func (s *Store) LabelValues(ctx context.Context, _ string, _ storage.TimeRange) ([]string, error) {
	if err := requireProject(ctx); err != nil {
		return nil, err
	}
	return nil, storage.ErrNotImplemented
}

func (s *Store) Series(ctx context.Context, _ []logql.LabelMatcher, _ storage.TimeRange) ([]storage.LabelSet, error) {
	if err := requireProject(ctx); err != nil {
		return nil, err
	}
	return nil, storage.ErrNotImplemented
}
