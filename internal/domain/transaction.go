package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type TransactionType string

const (
	TxTypeDeposit    TransactionType = "DEPOSIT"
	TxTypeWithdrawal TransactionType = "WITHDRAWAL"
	TxTypeTransfer   TransactionType = "TRANSFER"
)

type TransactionStatus string

const (
	TxStatusPending   TransactionStatus = "PENDING"
	TxStatusCompleted TransactionStatus = "COMPLETED"
	TxStatusFailed    TransactionStatus = "FAILED"
	TxStatusReversed  TransactionStatus = "REVERSED"
)

// Transaction is the immutable audit record of a balance change. Storing
// both wallet IDs as nullable lets us represent the three types uniformly:
//   - Deposit:    src=nil,  dst=walletID
//   - Withdrawal: src=walletID, dst=nil
//   - Transfer:   src=walletA, dst=walletB
//
// Idempotency: an external IdempotencyKey lets clients retry a request without
// double-charging. The repository layer enforces uniqueness on (user_id, key).
type Transaction struct {
	ID             uuid.UUID
	Type           TransactionType
	Status         TransactionStatus
	SourceWalletID *uuid.UUID
	TargetWalletID *uuid.UUID
	Amount         Money
	IdempotencyKey string
	Reference      string
	CreatedAt      time.Time
}

func NewDeposit(targetWalletID uuid.UUID, amount Money, idempotencyKey string) (*Transaction, error) {
	return newTx(TxTypeDeposit, nil, &targetWalletID, amount, idempotencyKey)
}

func NewWithdrawal(sourceWalletID uuid.UUID, amount Money, idempotencyKey string) (*Transaction, error) {
	return newTx(TxTypeWithdrawal, &sourceWalletID, nil, amount, idempotencyKey)
}

func NewTransfer(sourceWalletID, targetWalletID uuid.UUID, amount Money, idempotencyKey string) (*Transaction, error) {
	if sourceWalletID == targetWalletID {
		return nil, ErrSameWalletTransfer
	}
	return newTx(TxTypeTransfer, &sourceWalletID, &targetWalletID, amount, idempotencyKey)
}

func newTx(t TransactionType, src, dst *uuid.UUID, amount Money, idem string) (*Transaction, error) {
	if !amount.IsPositive() {
		return nil, fmt.Errorf("%w: transaction amount must be > 0", ErrNonPositiveAmount)
	}
	if !amount.Currency.Valid() {
		return nil, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, amount.Currency)
	}
	if idem == "" {
		return nil, fmt.Errorf("%w: idempotency_key is required", ErrInvalidInput)
	}
	return &Transaction{
		ID:             uuid.New(),
		Type:           t,
		Status:         TxStatusPending,
		SourceWalletID: src,
		TargetWalletID: dst,
		Amount:         amount,
		IdempotencyKey: idem,
		CreatedAt:      time.Now().UTC(),
	}, nil
}

// MarkCompleted / MarkFailed are explicit transitions instead of letting
// callers set Status directly — keeps the state machine in one place.
func (t *Transaction) MarkCompleted() { t.Status = TxStatusCompleted }
func (t *Transaction) MarkFailed()    { t.Status = TxStatusFailed }
