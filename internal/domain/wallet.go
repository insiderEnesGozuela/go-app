package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

// Wallet is the canonical balance container for a user in a given currency.
// One user may have multiple wallets (one per currency). The `Version` field
// supports optimistic locking later when we add concurrent updates without
// row-level FOR UPDATE locks.
type Wallet struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Balance   Money
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewWallet(userID uuid.UUID, currency Currency) (*Wallet, error) {
	if userID == uuid.Nil {
		return nil, fmt.Errorf("%w: user_id is required", ErrInvalidInput)
	}
	if !currency.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, currency)
	}
	now := time.Now().UTC()
	return &Wallet{
		ID:        uuid.New(),
		UserID:    userID,
		Balance:   Money{Amount: 0, Currency: currency},
		Version:   1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

// Credit increases the balance. Validation lives on the entity (not the
// service) so any code path that mutates a Wallet enforces the same rules.
func (w *Wallet) Credit(amount Money) error {
	if !amount.IsPositive() {
		return fmt.Errorf("%w: credit amount must be > 0", ErrNonPositiveAmount)
	}
	next, err := w.Balance.Add(amount)
	if err != nil {
		return err
	}
	w.Balance = next
	w.Version++
	w.UpdatedAt = time.Now().UTC()
	return nil
}

// Debit decreases the balance, refusing to go negative. Callers in financial
// flows MUST wrap Debit + persistence in a DB transaction with a FOR UPDATE
// lock on the wallet row to avoid TOCTOU races between balance check and
// commit. The in-memory check here is defense in depth, not the primary
// safeguard.
func (w *Wallet) Debit(amount Money) error {
	if !amount.IsPositive() {
		return fmt.Errorf("%w: debit amount must be > 0", ErrNonPositiveAmount)
	}
	ok, err := w.Balance.GreaterThanOrEqual(amount)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("%w: have %s, need %s", ErrInsufficientBalance, w.Balance, amount)
	}
	next, err := w.Balance.Sub(amount)
	if err != nil {
		return err
	}
	w.Balance = next
	w.Version++
	w.UpdatedAt = time.Now().UTC()
	return nil
}
