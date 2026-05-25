// Package service holds use-case-level interfaces. Concrete implementations
// will be added in Month 2 once domain + repositories are stable. We expose
// the interfaces now so handlers (Month 3) can be designed against them.
package service

import (
	"context"

	"github.com/google/uuid"

	"github.com/insiderEnesGozuela/go-app/internal/domain"
)

type RegisterUserInput struct {
	Email    string
	FullName string
	Password string
}

type UserService interface {
	Register(ctx context.Context, in RegisterUserInput) (*domain.User, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
}

type CreateWalletInput struct {
	UserID   uuid.UUID
	Currency domain.Currency
}

type WalletService interface {
	Create(ctx context.Context, in CreateWalletInput) (*domain.Wallet, error)
	GetBalance(ctx context.Context, walletID uuid.UUID) (domain.Money, error)
}

type DepositInput struct {
	WalletID       uuid.UUID
	Amount         domain.Money
	IdempotencyKey string
}

type WithdrawInput struct {
	WalletID       uuid.UUID
	Amount         domain.Money
	IdempotencyKey string
}

type TransferInput struct {
	SourceWalletID uuid.UUID
	TargetWalletID uuid.UUID
	Amount         domain.Money
	IdempotencyKey string
}

type TransactionService interface {
	Deposit(ctx context.Context, in DepositInput) (*domain.Transaction, error)
	Withdraw(ctx context.Context, in WithdrawInput) (*domain.Transaction, error)
	Transfer(ctx context.Context, in TransferInput) (*domain.Transaction, error)
	GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error)
}
