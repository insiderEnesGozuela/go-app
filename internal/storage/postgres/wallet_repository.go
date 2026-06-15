package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insiderEnesGozuela/go-app/internal/domain"
)

// WalletRepository is the Postgres implementation of repository.WalletRepository.
// The wallet is the hot, contended row in a payment system, so this is where the
// concurrency story (FOR UPDATE locking + optimistic version checks) lives.
type WalletRepository struct {
	pool *pgxpool.Pool
}

func NewWalletRepository(pool *pgxpool.Pool) *WalletRepository {
	return &WalletRepository{pool: pool}
}

func (r *WalletRepository) Create(ctx context.Context, w *domain.Wallet) error {
	const q = `
		INSERT INTO wallets (id, user_id, balance, currency, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`
	// Money is flattened into (balance, currency) columns. We pass the minor-unit
	// int64 directly — no float conversion ever touches the money path.
	_, err := querierFrom(ctx, r.pool).Exec(ctx, q,
		w.ID, w.UserID, w.Balance.Amount, w.Balance.Currency, w.Version, w.CreatedAt, w.UpdatedAt)
	if err != nil {
		return fmt.Errorf("insert wallet: %w", mapError(err, domain.ErrWalletNotFound))
	}
	return nil
}

func (r *WalletRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error) {
	const q = `
		SELECT id, user_id, balance, currency, version, created_at, updated_at
		FROM wallets WHERE id = $1`
	row := querierFrom(ctx, r.pool).QueryRow(ctx, q, id)
	return scanWallet(row)
}

func (r *WalletRepository) GetByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.Wallet, error) {
	const q = `
		SELECT id, user_id, balance, currency, version, created_at, updated_at
		FROM wallets WHERE user_id = $1 AND currency = $2`
	row := querierFrom(ctx, r.pool).QueryRow(ctx, q, userID, currency)
	return scanWallet(row)
}

// GetForUpdate acquires a row-level write lock with SELECT ... FOR UPDATE. Any
// other transaction that also runs GetForUpdate (or UPDATE) on the same wallet
// row BLOCKS until this transaction commits or rolls back. That serializes
// concurrent debits on the same wallet and closes the classic balance-check
// TOCTOU race: read-balance → check → write can no longer interleave.
//
// CRITICAL: this is only meaningful inside a transaction. Outside a UnitOfWork
// the lock is taken and released on the pool's autocommit, giving you nothing.
// Callers MUST invoke this from within uow.Do(...). We don't (and can't cheaply)
// assert that here, so it's a documented contract.
func (r *WalletRepository) GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Wallet, error) {
	const q = `
		SELECT id, user_id, balance, currency, version, created_at, updated_at
		FROM wallets WHERE id = $1
		FOR UPDATE`
	row := querierFrom(ctx, r.pool).QueryRow(ctx, q, id)
	return scanWallet(row)
}

// Update persists a new balance + version using optimistic locking. The WHERE
// clause requires the version to still match what we read (w.Version-1, since
// Credit/Debit already incremented it in memory). If a concurrent transaction
// changed the row, 0 rows match and we return ErrAlreadyExists-style conflict.
//
// Belt-and-suspenders: with FOR UPDATE the optimistic check is redundant (the
// lock already serializes writers), but keeping it means Update is also safe
// when called WITHOUT a preceding GetForUpdate — e.g. a read-modify-write that
// only took a plain SELECT. Defense in depth costs one extra predicate.
func (r *WalletRepository) Update(ctx context.Context, w *domain.Wallet) error {
	const q = `
		UPDATE wallets
		SET balance = $1, version = $2, updated_at = $3
		WHERE id = $4 AND version = $5`
	tag, err := querierFrom(ctx, r.pool).Exec(ctx, q,
		w.Balance.Amount, w.Version, w.UpdatedAt, w.ID, w.Version-1)
	if err != nil {
		return fmt.Errorf("update wallet: %w", mapError(err, domain.ErrWalletNotFound))
	}
	if tag.RowsAffected() == 0 {
		// Either the wallet doesn't exist or its version moved under us
		// (optimistic-lock conflict). Surface a distinct error so the service
		// can retry the whole read-modify-write if it chooses.
		return fmt.Errorf("update wallet %s: %w", w.ID, domain.ErrConcurrentUpdate)
	}
	return nil
}

func scanWallet(row rowScanner) (*domain.Wallet, error) {
	var (
		w        domain.Wallet
		amount   int64
		currency domain.Currency
	)
	err := row.Scan(&w.ID, &w.UserID, &amount, &currency, &w.Version, &w.CreatedAt, &w.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan wallet: %w", mapError(err, domain.ErrWalletNotFound))
	}
	// Reassemble the Money value object from its two flattened columns.
	w.Balance = domain.Money{Amount: amount, Currency: currency}
	return &w, nil
}
