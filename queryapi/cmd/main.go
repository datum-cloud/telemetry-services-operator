// SPDX-License-Identifier: AGPL-3.0-only

// Command queryapi serves the telemetry query layer HTTP API. See
// ../openapi.yaml for the request/response contract.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.datum.net/o11y/queryapi"
	"go.datum.net/o11y/queryapi/internal/storage"
	"go.datum.net/o11y/queryapi/internal/storage/fake"
)

func main() {
	var cfg queryapi.Config
	queryapi.RegisterFlags(flag.CommandLine, &cfg)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	store, err := newStore(cfg)
	if err != nil {
		logger.Error("configure storage", "error", err)
		os.Exit(1)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           queryapi.NewHandler(logger, store, cfg),
		ReadHeaderTimeout: cfg.ReadHeaderTimeout,
		IdleTimeout:       cfg.IdleTimeout,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	serveErr := make(chan error, 1)
	go func() {
		logger.Info("starting query api", "addr", cfg.Addr)
		serveErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("query api server exited", "error", err)
			os.Exit(1)
		}
	case <-ctx.Done():
		logger.Info("shutting down query api")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			logger.Error("query api shutdown did not complete cleanly", "error", err)
			os.Exit(1)
		}
	}
}

// newStore builds the configured storage backend. The clickhouse case lands
// in the next change; until then only fake is selectable.
func newStore(cfg queryapi.Config) (storage.LogStore, error) {
	switch cfg.Storage {
	case "fake":
		return fake.New(cfg.FakeRate), nil
	default:
		return nil, fmt.Errorf("unknown storage backend %q", cfg.Storage)
	}
}
