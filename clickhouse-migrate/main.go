// Command clickhouse-migrate applies golang-migrate SQL migrations to a
// ClickHouse database, authenticating via a client TLS certificate rather
// than a password (see users.d/ssl_auth.xml on the ClickHouse side).
package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"

	clickhousego "github.com/ClickHouse/clickhouse-go/v2"
	"github.com/golang-migrate/migrate/v4"
	chmigrate "github.com/golang-migrate/migrate/v4/database/clickhouse"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

type config struct {
	host            string
	port            string
	username        string
	database        string
	migrationsDir   string
	tlsCertFile     string
	tlsKeyFile      string
	tlsCAFile       string
	migrationsTable string
}

func configFromEnv() (config, error) {
	c := config{
		host:            os.Getenv("CLICKHOUSE_HOST"),
		port:            envOr("CLICKHOUSE_PORT", "9440"),
		username:        os.Getenv("CLICKHOUSE_USER"),
		database:        os.Getenv("CLICKHOUSE_DATABASE"),
		migrationsDir:   envOr("MIGRATIONS_DIR", "/migrations"),
		tlsCertFile:     envOr("CLICKHOUSE_TLS_CERT_FILE", "/etc/clickhouse-client/certs/tls.crt"),
		tlsKeyFile:      envOr("CLICKHOUSE_TLS_KEY_FILE", "/etc/clickhouse-client/certs/tls.key"),
		tlsCAFile:       envOr("CLICKHOUSE_TLS_CA_FILE", "/etc/clickhouse-client/certs/ca.crt"),
		migrationsTable: envOr("MIGRATIONS_TABLE", chmigrate.DefaultMigrationsTable),
	}

	var missing []string
	for name, v := range map[string]string{
		"CLICKHOUSE_HOST":     c.host,
		"CLICKHOUSE_USER":     c.username,
		"CLICKHOUSE_DATABASE": c.database,
	} {
		if v == "" {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return config{}, fmt.Errorf("missing required env vars: %v", missing)
	}
	return c, nil
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func loadClientTLSConfig(certFile, keyFile, caFile string) (*tls.Config, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("loading client cert/key: %w", err)
	}

	caBytes, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("reading CA file: %w", err)
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(caBytes) {
		return nil, fmt.Errorf("no certificates found in CA file %s", caFile)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
	}, nil
}

func run() error {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	cfg, err := configFromEnv()
	if err != nil {
		return err
	}

	tlsConfig, err := loadClientTLSConfig(cfg.tlsCertFile, cfg.tlsKeyFile, cfg.tlsCAFile)
	if err != nil {
		return fmt.Errorf("loading TLS config: %w", err)
	}

	db := clickhousego.OpenDB(&clickhousego.Options{
		Addr: []string{fmt.Sprintf("%s:%s", cfg.host, cfg.port)},
		Auth: clickhousego.Auth{
			Database: cfg.database,
			Username: cfg.username,
		},
		TLS: tlsConfig,
	})
	defer func() {
		if err := db.Close(); err != nil {
			logger.Error("closing clickhouse connection", "error", err)
		}
	}()

	driver, err := chmigrate.WithInstance(db, &chmigrate.Config{
		DatabaseName:    cfg.database,
		MigrationsTable: cfg.migrationsTable,
	})
	if err != nil {
		return fmt.Errorf("initializing clickhouse migrate driver: %w", err)
	}

	m, err := migrate.NewWithDatabaseInstance("file://"+cfg.migrationsDir, "clickhouse", driver)
	if err != nil {
		return fmt.Errorf("initializing migrate instance: %w", err)
	}

	logger.Info("applying migrations", "database", cfg.database, "migrations_dir", cfg.migrationsDir)
	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			logger.Info("no new migrations to apply")
			return nil
		}
		return fmt.Errorf("applying migrations: %w", err)
	}

	version, dirty, err := m.Version()
	if err != nil {
		return fmt.Errorf("reading migration version: %w", err)
	}
	logger.Info("migrations applied", "version", version, "dirty", dirty)
	return nil
}

func main() {
	if err := run(); err != nil {
		slog.New(slog.NewTextHandler(os.Stderr, nil)).Error("clickhouse-migrate failed", "error", err)
		os.Exit(1)
	}
}
