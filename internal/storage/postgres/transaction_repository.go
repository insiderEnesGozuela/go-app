package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insiderEnesGozuela/go-app/internal/domain"
)

// TransactionRepository is the Postgres implementation of
// repository.TransactionRepository. Transactions are an append-only audit log:
// we INSERT and we update status, but we never mutate amounts or wallets on an
// existing row.
type TransactionRepository struct {
	pool *pgxpool.Pool
}

func NewTransactionRepository(pool *pgxpool.Pool) *TransactionRepository {
	return &TransactionRepository{pool: pool}
}

func (r *TransactionRepository) Create(ctx context.Context, t *domain.Transaction) error {
	const q = `
		INSERT INTO transactions
			(id, type, status, source_wallet_id, target_wallet_id, amount, currency, idempotency_key, reference, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
	// source/target are *uuid.UUID — pgx encodes a nil pointer as SQL NULL,
	// which is exactly how DEPOSIT (no source) / WITHDRAWAL (no target) are
	// represented. No special-casing needed.
	_, err := querierFrom(ctx, r.pool).Exec(ctx, q,
		t.ID, t.Type, t.Status, t.SourceWalletID, t.TargetWalletID,
		t.Amount.Amount, t.Amount.Currency, t.IdempotencyKey, t.Reference, t.CreatedAt)
	if err != nil {
		// A repeated idempotency_key hits the unique index → ErrAlreadyExists.
		// The service layer treats that as "this request was already processed"
		// and returns the original transaction instead of double-charging.
		return fmt.Errorf("insert transaction: %w", mapError(err, domain.ErrTransactionNotFound))
	}
	return nil
}

func (r *TransactionRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	const q = `
		SELECT id, type, status, source_wallet_id, target_wallet_id, amount, currency, idempotency_key, reference, created_at
		FROM transactions WHERE id = $1`
	row := querierFrom(ctx, r.pool).QueryRow(ctx, q, id)
	return scanTransaction(row)
}

func (r *TransactionRepository) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	const q = `
		SELECT id, type, status, source_wallet_id, target_wallet_id, amount, currency, idempotency_key, reference, created_at
		FROM transactions WHERE idempotency_key = $1`
	row := querierFrom(ctx, r.pool).QueryRow(ctx, q, key)
	return scanTransaction(row)
}

func (r *TransactionRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TransactionStatus) error {
	const q = `UPDATE transactions SET status = $1 WHERE id = $2`
	tag, err := querierFrom(ctx, r.pool).Exec(ctx, q, status, id)
	if err != nil {
		return fmt.Errorf("update transaction status: %w", mapError(err, domain.ErrTransactionNotFound))
	}
	if tag.RowsAffected() == 0 {
		return fmt.Errorf("update transaction %s status: %w", id, domain.ErrTransactionNotFound)
	}
	return nil
}

func scanTransaction(row rowScanner) (*domain.Transaction, error) {
	var (
		t        domain.Transaction
		amount   int64
		currency domain.Currency
	)
	err := row.Scan(
		&t.ID, &t.Type, &t.Status, &t.SourceWalletID, &t.TargetWalletID,
		&amount, &currency, &t.IdempotencyKey, &t.Reference, &t.CreatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan transaction: %w", mapError(err, domain.ErrTransactionNotFound))
	}
	t.Amount = domain.Money{Amount: amount, Currency: currency}
	return &t, nil
}
