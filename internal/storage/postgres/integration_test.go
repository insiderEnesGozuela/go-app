//go:build integration

// Package postgres integration tests. These need a real PostgreSQL and are
// excluded from the default `go test ./...` by the `integration` build tag.
//
// Run them with:
//
//	docker compose up -d
//	TEST_DATABASE_URL="host=localhost port=5432 user=wallet password=change-me dbname=wallet sslmode=disable" \
//	  go test -tags=integration ./internal/storage/postgres/...
//
// Why a build tag instead of just skipping at runtime? A tagged file isn't even
// compiled into the normal test binary, so CI without Docker stays fast and
// green, while the same code is exercised end-to-end where a DB exists.
package postgres

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insiderEnesGozuela/go-app/internal/domain"
	"github.com/insiderEnesGozuela/go-app/internal/repository"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping integration test")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("parse config: %v", err)
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatalf("new pool: %v", err)
	}

	// Probe reachability *before* migrating. If the DB simply isn't up (e.g.
	// Docker not started), skip rather than fail — a connection-refused is an
	// environment gap, not a code defect. A reachable-but-broken migration
	// still fails below, which is what we want.
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		t.Skipf("TEST_DATABASE_URL set but database unreachable, skipping: %v", err)
	}

	if err := Migrate(dsn); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	t.Cleanup(pool.Close)
	return pool
}

// seedUser creates a user + a TRY wallet with the given starting balance and
// returns the wallet. Each call uses a fresh random email so tests don't
// collide on the unique index.
func seedUser(t *testing.T, ctx context.Context, pool *pgxpool.Pool, startBalance int64) *domain.Wallet {
	t.Helper()
	users := NewUserRepository(pool)
	wallets := NewWalletRepository(pool)

	u, err := domain.NewUser(uniqueEmail(), "Test User")
	if err != nil {
		t.Fatalf("new user: %v", err)
	}
	if err := users.Create(ctx, u); err != nil {
		t.Fatalf("create user: %v", err)
	}
	w, err := domain.NewWallet(u.ID, domain.CurrencyTRY)
	if err != nil {
		t.Fatalf("new wallet: %v", err)
	}
	if startBalance > 0 {
		if err := w.Credit(domain.Money{Amount: startBalance, Currency: domain.CurrencyTRY}); err != nil {
			t.Fatalf("seed credit: %v", err)
		}
	}
	if err := wallets.Create(ctx, w); err != nil {
		t.Fatalf("create wallet: %v", err)
	}
	return w
}

func uniqueEmail() string {
	return "u" + time.Now().Format("150405.000000000") + "@example.com"
}

// TestTransactionRepository_IdempotencyConflict proves the unique index turns a
// retried request into a domain.ErrAlreadyExists rather than a duplicate row.
func TestTransactionRepository_IdempotencyConflict(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	w := seedUser(t, ctx, pool, 0)
	txRepo := NewTransactionRepository(pool)

	key := "idem-" + uniqueEmail()
	tx1, err := domain.NewDeposit(w.ID, domain.Money{Amount: 5000, Currency: domain.CurrencyTRY}, key)
	if err != nil {
		t.Fatalf("new deposit: %v", err)
	}
	if err := txRepo.Create(ctx, tx1); err != nil {
		t.Fatalf("first create: %v", err)
	}

	// Same idempotency key, second time → must conflict.
	tx2, _ := domain.NewDeposit(w.ID, domain.Money{Amount: 5000, Currency: domain.CurrencyTRY}, key)
	err = txRepo.Create(ctx, tx2)
	if !errors.Is(err, domain.ErrAlreadyExists) {
		t.Fatalf("want ErrAlreadyExists on duplicate idempotency key, got %v", err)
	}
}

// TestWalletRepository_OptimisticLock proves a stale-version Update is rejected
// with ErrConcurrentUpdate (0 rows affected), the safety net that stops a
// lost-update from silently overwriting a concurrent balance change.
func TestWalletRepository_OptimisticLock(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	w := seedUser(t, ctx, pool, 10000)
	wallets := NewWalletRepository(pool)

	// Load the wallet twice — two readers with the same version.
	a, err := wallets.GetByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("get a: %v", err)
	}
	b, err := wallets.GetByID(ctx, w.ID)
	if err != nil {
		t.Fatalf("get b: %v", err)
	}

	// First writer wins.
	if err := a.Debit(domain.Money{Amount: 1000, Currency: domain.CurrencyTRY}); err != nil {
		t.Fatalf("debit a: %v", err)
	}
	if err := wallets.Update(ctx, a); err != nil {
		t.Fatalf("update a: %v", err)
	}

	// Second writer holds a stale version → must be rejected.
	if err := b.Debit(domain.Money{Amount: 2000, Currency: domain.CurrencyTRY}); err != nil {
		t.Fatalf("debit b: %v", err)
	}
	err = wallets.Update(ctx, b)
	if !errors.Is(err, domain.ErrConcurrentUpdate) {
		t.Fatalf("want ErrConcurrentUpdate on stale version, got %v", err)
	}
}

// TestUnitOfWork_TransferAtomic proves the money-conservation invariant: a
// transfer inside Do either fully applies (both balances change) or, on error,
// rolls back entirely (neither changes). We force the failure path by debiting
// more than the source holds AFTER a successful credit step.
func TestUnitOfWork_TransferAtomic(t *testing.T) {
	pool := newTestPool(t)
	ctx := context.Background()
	src := seedUser(t, ctx, pool, 10000) // 100.00 TRY
	dst := seedUser(t, ctx, pool, 0)
	uow := NewUnitOfWork(pool)

	amount := domain.Money{Amount: 3000, Currency: domain.CurrencyTRY}

	// Happy path: transfer 30.00 TRY src -> dst, atomically.
	err := uow.Do(ctx, func(ctx context.Context, repos repository.Repositories) error {
		s, err := repos.Wallets.GetForUpdate(ctx, src.ID)
		if err != nil {
			return err
		}
		d, err := repos.Wallets.GetForUpdate(ctx, dst.ID)
		if err != nil {
			return err
		}
		if err := s.Debit(amount); err != nil {
			return err
		}
		if err := d.Credit(amount); err != nil {
			return err
		}
		if err := repos.Wallets.Update(ctx, s); err != nil {
			return err
		}
		return repos.Wallets.Update(ctx, d)
	})
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}

	wallets := NewWalletRepository(pool)
	gotSrc, _ := wallets.GetByID(ctx, src.ID)
	gotDst, _ := wallets.GetByID(ctx, dst.ID)
	if gotSrc.Balance.Amount != 7000 {
		t.Errorf("src balance: want 7000, got %d", gotSrc.Balance.Amount)
	}
	if gotDst.Balance.Amount != 3000 {
		t.Errorf("dst balance: want 3000, got %d", gotDst.Balance.Amount)
	}

	// Rollback path: a transfer that errors mid-flight must leave BOTH balances
	// untouched. We return an error after mutating src in memory + persisting,
	// proving the COMMIT never happened.
	before, _ := wallets.GetByID(ctx, src.ID)
	wantErr := errors.New("boom")
	err = uow.Do(ctx, func(ctx context.Context, repos repository.Repositories) error {
		s, err := repos.Wallets.GetForUpdate(ctx, src.ID)
		if err != nil {
			return err
		}
		if err := s.Debit(amount); err != nil {
			return err
		}
		if err := repos.Wallets.Update(ctx, s); err != nil {
			return err
		}
		return wantErr // triggers ROLLBACK
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("want boom error, got %v", err)
	}
	after, _ := wallets.GetByID(ctx, src.ID)
	if before.Balance.Amount != after.Balance.Amount {
		t.Errorf("rollback failed: balance changed from %d to %d",
			before.Balance.Amount, after.Balance.Amount)
	}
}
