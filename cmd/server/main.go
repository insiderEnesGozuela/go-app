package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/insiderEnesGozuela/go-app/internal/config"
	"github.com/insiderEnesGozuela/go-app/internal/handler"
	"github.com/insiderEnesGozuela/go-app/internal/logger"
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

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handler.Health)

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
