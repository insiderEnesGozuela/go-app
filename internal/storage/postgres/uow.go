package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insiderEnesGozuela/go-app/internal/repository"
)

// querier is the subset of methods shared by *pgxpool.Pool and pgx.Tx. By
// programming repositories against this interface, the *same* repository code
// runs both standalone (pool) and inside a transaction (tx). This is what lets
// a transfer debit + credit two wallets atomically without duplicating SQL.
//
// The signatures match pgx exactly (Exec returns pgconn.CommandTag) so both
// *pgxpool.Pool and pgx.Tx satisfy this interface implicitly.
type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// txKey is the unexported context key under which an in-flight pgx.Tx is
// stored by UnitOfWork.Do. Repositories pull it out via querierFrom. Using a
// private struct type as the key guarantees no other package can collide.
type txKey struct{}

// querierFrom returns the transaction bound to ctx if one is active, otherwise
// the fallback pool. Every repository method starts with this call, so the same
// method body works inside and outside a UnitOfWork — the caller decides which
// by whether they're inside Do.
func querierFrom(ctx context.Context, pool *pgxpool.Pool) querier {
	if tx, ok := ctx.Value(txKey{}).(pgx.Tx); ok {
		return tx
	}
	return pool
}

// UnitOfWork is the pgx-backed implementation of repository.UnitOfWork. It owns
// the pool and hands tx-scoped repositories to the callback.
type UnitOfWork struct {
	pool  *pgxpool.Pool
	repos repository.Repositories
}

// NewUnitOfWork wires a UoW around a pool. The repository instances it holds are
// the same pool-backed ones used standalone; inside Do they transparently pick
// up the tx from context, so we don't need a second set of "tx repositories".
func NewUnitOfWork(pool *pgxpool.Pool) *UnitOfWork {
	return &UnitOfWork{
		pool: pool,
		repos: repository.Repositories{
			Users:        NewUserRepository(pool),
			Wallets:      NewWalletRepository(pool),
			Transactions: NewTransactionRepository(pool),
		},
	}
}

// Do runs fn inside a single database transaction. If fn returns nil the tx is
// committed; if it returns an error (or panics) the tx is rolled back and the
// error is propagated. This is the callback pattern that makes it impossible to
// forget a commit/rollback — the transaction boundary is the function boundary.
//
// The financial guarantee: a transfer's Debit(source) and Credit(target) both
// run against repos pulled from a ctx carrying the same tx. Either both persist
// (COMMIT) or neither does (ROLLBACK). No money is ever created or destroyed.
func (u *UnitOfWork) Do(ctx context.Context, fn func(ctx context.Context, repos repository.Repositories) error) (err error) {
	tx, err := u.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}

	// Bind the tx to a derived context so querierFrom routes every repo call
	// inside fn through this tx rather than the bare pool.
	txCtx := context.WithValue(ctx, txKey{}, tx)

	defer func() {
		if p := recover(); p != nil {
			// A panic must not leave a transaction dangling (it would hold locks
			// until the connection is reaped). Roll back, then re-panic so the
			// caller still sees the failure.
			_ = tx.Rollback(ctx)
			panic(p)
		}
		if err != nil {
			// fn failed (or commit failed). Roll back. We ignore the rollback
			// error if it's "already committed/rolled back" — that just means
			// the happy path already finished. Any other rollback error is
			// joined so it isn't lost.
			if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
				err = errors.Join(err, fmt.Errorf("rollback: %w", rbErr))
			}
		}
	}()

	if err = fn(txCtx, u.repos); err != nil {
		return err // deferred rollback handles cleanup
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}
	return nil
}
