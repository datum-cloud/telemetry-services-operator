// SPDX-License-Identifier: AGPL-3.0-only

// Command queryapi serves the telemetry query layer HTTP API: tenant-scoped
// log and metric queries over ClickHouse for datumctl and the customer cloud
// portal. See ../openapi.yaml for the request/response contract
// and datum-cloud/enhancements, enhancements/telemetry/query-layer/README.md
// for the design.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.datum.net/o11y/queryapi"
)

func main() {
	var cfg queryapi.Config
	queryapi.RegisterFlags(flag.CommandLine, &cfg)
	flag.Parse()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           queryapi.NewHandler(logger),
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
