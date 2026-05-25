// Package repository defines the persistence contracts used by service layer.
// Concrete implementations (e.g. PostgreSQL) live in subpackages such as
// repository/postgres. The service layer depends only on these interfaces,
// never on a concrete driver — that's how Clean Architecture keeps the
// business logic testable without spinning up a database.
package repository

import (
	"context"

	"github.com/google/uuid"

	"github.com/insiderEnesGozuela/go-app/internal/domain"
)

// UnitOfWork groups multiple repository calls into a single DB transaction.
// Critical for wallet operations: a transfer that debits source and credits
// target MUST commit atomically or roll back entirely. Implementations should
// pass a tx-scoped context so all repos inside the callback share one tx.
type UnitOfWork interface {
	Do(ctx context.Context, fn func(ctx context.Context, repos Repositories) error) error
}

// Repositories is the bundle handed to a UnitOfWork callback. Holds tx-scoped
// repository instances; outside a UoW callers use the standalone repos below.
type Repositories struct {
	Users        UserRepository
	Wallets      WalletRepository
	Transactions TransactionRepository
}

type UserRepository interface {
	Create(ctx context.Context, u *domain.User) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetByEmail(ctx context.Context, email string) (*domain.User, error)
}

type WalletRepository interface {
	Create(ctx context.Context, w *domain.Wallet) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Wallet, error)
	GetByUserAndCurrency(ctx context.Context, userID uuid.UUID, currency domain.Currency) (*domain.Wallet, error)

	// GetForUpdate must acquire a row-level lock (SELECT ... FOR UPDATE) so a
	// concurrent debit on the same wallet within another tx blocks until commit.
	// Returning the locked wallet without an error guarantees the caller has
	// exclusive write access for the remainder of the transaction.
	GetForUpdate(ctx context.Context, id uuid.UUID) (*domain.Wallet, error)

	// Update applies a new balance + version. Implementations should check the
	// version column for optimistic-lock failures when not using FOR UPDATE.
	Update(ctx context.Context, w *domain.Wallet) error
}

type TransactionRepository interface {
	Create(ctx context.Context, t *domain.Transaction) error
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error)
	GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status domain.TransactionStatus) error
}
