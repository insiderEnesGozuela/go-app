package service

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/insiderEnesGozuela/go-app/internal/domain"
	"github.com/insiderEnesGozuela/go-app/internal/repository"
)

type transactionService struct {
	uow repository.UnitOfWork
	txs repository.TransactionRepository
}

// NewTransactionService wires the service with a UnitOfWork (for write paths
// that must be atomic) and a bare TransactionRepository (for read-only lookups
// that don't need a tx boundary).
func NewTransactionService(uow repository.UnitOfWork, txs repository.TransactionRepository) TransactionService {
	return &transactionService{uow: uow, txs: txs}
}

// Deposit credits a wallet and records the transaction atomically.
//
// Idempotency: if IdempotencyKey already exists in the DB we return the
// original transaction without re-applying the credit. This makes the endpoint
// safe for client retries.
//
// Why UoW here? Deposit touches two tables (wallets + transactions). If the
// wallet update commits but the transaction INSERT fails (or vice-versa), the
// ledger and the audit log diverge. The UoW guarantees both succeed or both fail.
func (s *transactionService) Deposit(ctx context.Context, in DepositInput) (*domain.Transaction, error) {
	if existing, err := s.txs.GetByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrTransactionNotFound) {
		return nil, fmt.Errorf("idempotency check: %w", err)
	}

	tx, err := domain.NewDeposit(in.WalletID, in.Amount, in.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("build deposit: %w", err)
	}

	err = s.uow.Do(ctx, func(ctx context.Context, repos repository.Repositories) error {
		// FOR UPDATE acquires a row-level write lock on the wallet, preventing
		// concurrent debits/credits from reading a stale balance until this tx commits.
		wallet, err := repos.Wallets.GetForUpdate(ctx, in.WalletID)
		if err != nil {
			return fmt.Errorf("lock wallet: %w", err)
		}

		if err := wallet.Credit(in.Amount); err != nil {
			return fmt.Errorf("credit wallet: %w", err)
		}

		if err := repos.Wallets.Update(ctx, wallet); err != nil {
			return fmt.Errorf("persist wallet: %w", err)
		}

		if err := repos.Transactions.Create(ctx, tx); err != nil {
			return fmt.Errorf("persist transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		tx.MarkFailed()
		return nil, fmt.Errorf("deposit: %w", err)
	}

	tx.MarkCompleted()
	return tx, nil
}

// Withdraw debits a wallet and records the transaction atomically.
//
// The FOR UPDATE lock + the in-memory Debit check together prevent
// overdrafts: no two concurrent withdrawals can both see the same positive
// balance and both succeed.
func (s *transactionService) Withdraw(ctx context.Context, in WithdrawInput) (*domain.Transaction, error) {
	if existing, err := s.txs.GetByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrTransactionNotFound) {
		return nil, fmt.Errorf("idempotency check: %w", err)
	}

	tx, err := domain.NewWithdrawal(in.WalletID, in.Amount, in.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("build withdrawal: %w", err)
	}

	err = s.uow.Do(ctx, func(ctx context.Context, repos repository.Repositories) error {
		wallet, err := repos.Wallets.GetForUpdate(ctx, in.WalletID)
		if err != nil {
			return fmt.Errorf("lock wallet: %w", err)
		}

		if err := wallet.Debit(in.Amount); err != nil {
			return fmt.Errorf("debit wallet: %w", err)
		}

		if err := repos.Wallets.Update(ctx, wallet); err != nil {
			return fmt.Errorf("persist wallet: %w", err)
		}

		if err := repos.Transactions.Create(ctx, tx); err != nil {
			return fmt.Errorf("persist transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		tx.MarkFailed()
		return nil, fmt.Errorf("withdraw: %w", err)
	}

	tx.MarkCompleted()
	return tx, nil
}

// Transfer moves money from source to target wallet atomically.
//
// Lock ordering matters for deadlock prevention: we always lock the wallet
// with the smaller UUID first, regardless of which is source or target.
// If two concurrent transfers go in opposite directions (A→B and B→A) and
// both try to lock in wallet order, they'll queue behind the same first lock
// instead of forming a deadlock cycle.
func (s *transactionService) Transfer(ctx context.Context, in TransferInput) (*domain.Transaction, error) {
	if existing, err := s.txs.GetByIdempotencyKey(ctx, in.IdempotencyKey); err == nil {
		return existing, nil
	} else if !errors.Is(err, domain.ErrTransactionNotFound) {
		return nil, fmt.Errorf("idempotency check: %w", err)
	}

	tx, err := domain.NewTransfer(in.SourceWalletID, in.TargetWalletID, in.Amount, in.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("build transfer: %w", err)
	}

	err = s.uow.Do(ctx, func(ctx context.Context, repos repository.Repositories) error {
		first, second := lockOrder(in.SourceWalletID, in.TargetWalletID)

		firstWallet, err := repos.Wallets.GetForUpdate(ctx, first)
		if err != nil {
			return fmt.Errorf("lock wallet %s: %w", first, err)
		}
		secondWallet, err := repos.Wallets.GetForUpdate(ctx, second)
		if err != nil {
			return fmt.Errorf("lock wallet %s: %w", second, err)
		}

		// Re-identify which locked wallet is source and which is target.
		src, dst := firstWallet, secondWallet
		if first == in.TargetWalletID {
			src, dst = secondWallet, firstWallet
		}

		if err := src.Debit(in.Amount); err != nil {
			return fmt.Errorf("debit source: %w", err)
		}
		if err := dst.Credit(in.Amount); err != nil {
			return fmt.Errorf("credit target: %w", err)
		}

		if err := repos.Wallets.Update(ctx, src); err != nil {
			return fmt.Errorf("persist source wallet: %w", err)
		}
		if err := repos.Wallets.Update(ctx, dst); err != nil {
			return fmt.Errorf("persist target wallet: %w", err)
		}

		if err := repos.Transactions.Create(ctx, tx); err != nil {
			return fmt.Errorf("persist transaction: %w", err)
		}
		return nil
	})
	if err != nil {
		tx.MarkFailed()
		return nil, fmt.Errorf("transfer: %w", err)
	}

	tx.MarkCompleted()
	return tx, nil
}

func (s *transactionService) GetByID(ctx context.Context, id uuid.UUID) (*domain.Transaction, error) {
	t, err := s.txs.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get transaction %s: %w", id, err)
	}
	return t, nil
}

// lockOrder returns two wallet IDs in a deterministic sequence (smaller UUID
// string first) to enforce a global lock ordering across goroutines/connections.
// This makes A→B and B→A transfers take locks in the same order, eliminating
// the classic deadlock cycle between two concurrent opposite-direction transfers.
func lockOrder(a, b uuid.UUID) (uuid.UUID, uuid.UUID) {
	if a.String() < b.String() {
		return a, b
	}
	return b, a
}
