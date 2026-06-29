package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/insiderEnesGozuela/go-app/internal/domain"
	"github.com/insiderEnesGozuela/go-app/internal/repository"
)

type walletService struct {
	wallets repository.WalletRepository
	users   repository.UserRepository
}

// NewWalletService wires the service. We inject UserRepository so we can
// validate that the owning user exists before creating a wallet — a wallet
// for a ghost user would be an orphan and is a domain rule violation.
func NewWalletService(wallets repository.WalletRepository, users repository.UserRepository) WalletService {
	return &walletService{wallets: wallets, users: users}
}

// Create opens a new wallet for a user in the requested currency.
// Business rules enforced here:
//   - The user must exist.
//   - A user may only hold one wallet per currency (duplicate → ErrWalletAlreadyExists).
//
// The DB UNIQUE(user_id, currency) constraint is the authoritative guard; the
// advisory check here returns a cleaner error before hitting the DB.
func (s *walletService) Create(ctx context.Context, in CreateWalletInput) (*domain.Wallet, error) {
	if _, err := s.users.GetByID(ctx, in.UserID); err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, fmt.Errorf("create wallet: owner not found: %w", domain.ErrUserNotFound)
		}
		return nil, fmt.Errorf("verify wallet owner: %w", err)
	}

	// Advisory uniqueness check — makes the error message obvious for callers.
	_, err := s.wallets.GetByUserAndCurrency(ctx, in.UserID, in.Currency)
	if err == nil {
		return nil, fmt.Errorf("create wallet: %w", domain.ErrWalletAlreadyExists)
	}
	if !errors.Is(err, domain.ErrWalletNotFound) {
		return nil, fmt.Errorf("check existing wallet: %w", err)
	}

	w, err := domain.NewWallet(in.UserID, in.Currency)
	if err != nil {
		return nil, fmt.Errorf("build wallet: %w", err)
	}

	if err := s.wallets.Create(ctx, w); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return nil, fmt.Errorf("create wallet: %w", domain.ErrWalletAlreadyExists)
		}
		return nil, fmt.Errorf("persist wallet: %w", err)
	}
	return w, nil
}

func (s *walletService) GetBalance(ctx context.Context, walletID uuid.UUID) (domain.Money, error) {
	w, err := s.wallets.GetByID(ctx, walletID)
	if err != nil {
		return domain.Money{}, fmt.Errorf("get balance for wallet %s: %w", walletID, err)
	}
	return w.Balance, nil
}
