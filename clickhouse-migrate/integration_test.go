package main

import (
	"os"
	"testing"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/stretchr/testify/require"
)

// TestApplyMigrations_FreshDatabase applies the repository's real migration
// set to an empty database, which is what the clickhouse-migrate Job does on
// a brand-new ClickHouse instance.
//
// It needs a live server, so it is skipped unless CLICKHOUSE_TEST_ADDR is set
// (see `task clickhouse-migrate:test-integration`, which starts one).
func TestApplyMigrations_FreshDatabase(t *testing.T) {
	addr := os.Getenv("CLICKHOUSE_TEST_ADDR")
	if addr == "" {
		t.Skip("CLICKHOUSE_TEST_ADDR not set")
	}

	const database = "o11y_migrate_test"
	cfg := config{
		database:      database,
		migrationsDir: "../config/clickhouse-migrations/migrations",
	}

	opts := &clickhousego.Options{
		Addr: []string{addr},
		Auth: clickhousego.Auth{
			Username: envOr("CLICKHOUSE_TEST_USER", "default"),
			Password: os.Getenv("CLICKHOUSE_TEST_PASSWORD"),
		},
	}

	// Bootstrap connection: not scoped to the target database, so it can
	// create it (and drop any leftovers from a previous run).
	bootstrap := clickhousego.OpenDB(opts)
	t.Cleanup(func() { require.NoError(t, bootstrap.Close()) })

	_, err := bootstrap.Exec("DROP DATABASE IF EXISTS " + quoteIdentifier(database))
	require.NoError(t, err)
	_, err = bootstrap.Exec("CREATE DATABASE " + quoteIdentifier(database))
	require.NoError(t, err)

	scoped := *opts
	scoped.Auth.Database = database
	db := clickhousego.OpenDB(&scoped)
	t.Cleanup(func() { require.NoError(t, db.Close()) })

	version, dirty, err := applyMigrations(db, cfg)
	require.NoError(t, err)
	require.False(t, dirty, "migrations left the database dirty at version %d", version)
	require.Equal(t, uint(1), version)

	// Re-running must be a no-op, not an error: the Job retries on failure
	// and gets re-applied on every deploy.
	version, dirty, err = applyMigrations(db, cfg)
	require.NoError(t, err)
	require.False(t, dirty)
	require.Equal(t, uint(1), version)

	var count uint64
	require.NoError(t, db.QueryRow(
		"SELECT count() FROM system.tables WHERE database = ? AND name = 'logs'",
		database,
	).Scan(&count))
	require.Equal(t, uint64(1), count, "expected the logs table to exist")
}
