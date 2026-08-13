package main

import (
	"database/sql"
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

	// The 000001 migration GRANTs and row-policies ops and queryapi, which exist
	// on the real server via ssl_auth.xml certificate mapping. That layer is
	// absent here, so create the users for the migration to apply to.
	createTestUser(t, bootstrap, "ops")
	createTestUser(t, bootstrap, "queryapi")
	// queryapi needs custom-setting rights for the row policy.
	_, err = bootstrap.Exec("GRANT settings_allow_custom_setting_read, settings_allow_custom_setting_write ON *.* TO queryapi")
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

	assertRestrictedQueryapi(t, db, database)
}

// assertRestrictedQueryapi verifies the 000001 migration installed the queryapi
// row policy and read-only grant. Requires the queryapi user to exist.
func assertRestrictedQueryapi(t *testing.T, db *sql.DB, database string) {
	t.Helper()

	var policies uint64
	require.NoError(t, db.QueryRow(
		"SELECT count() FROM system.row_policies WHERE database = ? AND table_name = 'logs' AND policy_name = 'queryapi_project_isolation'",
		database,
	).Scan(&policies))
	require.Equal(t, uint64(1), policies, "expected the queryapi_project_isolation row policy on logs")

	// ops keeps full read; queryapi is confined to SELECT on logs.
	var grants string
	require.NoError(t, db.QueryRow(
		"SELECT access_type FROM system.grants WHERE user_name = 'queryapi' AND (database, table) = (?, 'logs')",
		database,
	).Scan(&grants))
	require.Equal(t, "SELECT", grants, "queryapi must be read-only (SELECT) on logs")
}

// createTestUser creates a throwaway user for integration testing, dropping any
// leftover from a previous run first.
func createTestUser(t *testing.T, db *sql.DB, name string) {
	t.Helper()
	for _, stmt := range []string{
		"DROP USER IF EXISTS " + name,
		"CREATE USER " + name + " IDENTIFIED WITH no_password",
	} {
		_, err := db.Exec(stmt)
		require.NoError(t, err, "exec %q", stmt)
	}
}

