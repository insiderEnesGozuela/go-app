// Package postgres holds the PostgreSQL-backed infrastructure: the connection
// pool, repository implementations, and the Unit-of-Work transaction wrapper.
//
// This is the *only* place in the codebase that imports a database driver.
// The service layer talks to the repository interfaces in internal/repository;
// it never sees pgx. That is the inward-pointing dependency rule from Clean
// Architecture in practice — swap Postgres for another store by rewriting this
// package alone.
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insiderEnesGozuela/go-app/internal/config"
)

// NewPool builds a pgx connection pool from config and verifies connectivity
// with a Ping before returning. We Ping eagerly so a misconfigured DSN fails at
// startup (fast, obvious) instead of on the first request (slow, confusing).
//
// ctx governs the *connection establishment*, not the pool's lifetime — pass a
// short-lived context with a timeout here; the pool itself lives until Close.
func NewPool(ctx context.Context, cfg config.DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN())
	if err != nil {
		return nil, fmt.Errorf("parse pgx config: %w", err)
	}

	// Map our config knobs onto pgxpool's. MaxConns caps total connections;
	// MinConns keeps a warm floor so the first requests after idle don't pay
	// the TCP+TLS+auth handshake. MaxConnLifetime recycles connections so a
	// long-lived pool doesn't pin a connection a load balancer wants to drain.
	poolCfg.MaxConns = cfg.MaxOpenConns
	poolCfg.MinConns = cfg.MaxIdleConns
	poolCfg.MaxConnLifetime = cfg.ConnMaxLifetime

	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return nil, fmt.Errorf("create pgx pool: %w", err)
	}

	if err := pool.Ping(ctx); err != nil {
		// Ping failed → the pool is useless. Close it so we don't leak the
		// background goroutines pgxpool spawns, then surface the error.
		pool.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}

	return pool, nil
}
