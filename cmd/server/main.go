package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/insiderEnesGozuela/go-app/internal/config"
	"github.com/insiderEnesGozuela/go-app/internal/handler"
	"github.com/insiderEnesGozuela/go-app/internal/logger"
	"github.com/insiderEnesGozuela/go-app/internal/storage/postgres"
)

func main() {
	if err := run(); err != nil {
		// Logger may not exist if config load failed — fall back to stderr.
		fmt.Fprintf(os.Stderr, "fatal: %v\n", err)
		os.Exit(1)
	}
}

// run owns the application lifecycle and returns errors instead of calling
// os.Exit / log.Fatal directly. Easier to test, and defers in main() still
// run before exit (log.Fatal skips them).
func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	log := logger.New(logger.Options{
		Level:   cfg.Log.Level,
		Pretty:  cfg.Log.Pretty,
		Service: cfg.App.Name,
	})
	log.Info().
		Str("env", cfg.App.Env).
		Str("port", cfg.HTTP.Port).
		Msg("starting server")

	// Run migrations before opening the long-lived pool. If the schema can't be
	// brought up to date, the binary the code expects can't run safely — fail
	// fast at startup rather than serving against a stale schema.
	if err := postgres.Migrate(cfg.Database.DSN()); err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}
	log.Info().Msg("migrations up to date")

	// Open the connection pool with a bounded context so a wrong DSN fails fast
	// instead of hanging the whole boot on a TCP timeout.
	poolCtx, poolCancel := context.WithTimeout(context.Background(), 10*time.Second)
	pool, err := postgres.NewPool(poolCtx, cfg.Database)
	poolCancel()
	if err != nil {
		return fmt.Errorf("connect database: %w", err)
	}
	// Close drains in-flight queries and releases every connection. Deferred so
	// it runs on every return path — this is exactly why run() returns an error
	// instead of calling log.Fatal (which would skip defers and leak the pool).
	defer pool.Close()
	log.Info().
		Int32("max_conns", cfg.Database.MaxOpenConns).
		Msg("database pool ready")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.Health)
	// /readyz pings the DB so k8s only routes traffic once the pool is usable.
	mux.HandleFunc("/readyz", handler.Readiness(pool))

	server := &http.Server{
		Addr:         ":" + cfg.HTTP.Port,
		Handler:      mux,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
	}

	// signal.NotifyContext gives us a context that fires Done() on SIGINT/SIGTERM.
	// Using a context (vs a signal channel) lets us pass shutdown intent down
	// to any goroutine that takes a context — uniform cancellation surface.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	// serverErr is buffered so the goroutine never leaks if main() returns
	// before we receive from it (e.g. signal arrives at the same instant).
	serverErr := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErr <- err
			return
		}
		serverErr <- nil
	}()

	select {
	case err := <-serverErr:
		return fmt.Errorf("server failed: %w", err)
	case <-ctx.Done():
		log.Info().Msg("shutdown signal received")
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		// Shutdown returned non-nil — force-close so we don't leak sockets,
		// then surface the original error.
		_ = server.Close()
		return fmt.Errorf("graceful shutdown failed after %s: %w", cfg.HTTP.ShutdownTimeout, err)
	}

	log.Info().Msg("server stopped cleanly")
	return nil
}
