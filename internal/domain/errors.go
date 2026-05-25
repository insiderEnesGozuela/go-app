// Package domain holds the pure business model: entities, value objects, and
// the errors that describe domain rule violations. It has no dependencies on
// infra (DB drivers, HTTP libraries) — that's enforced by Clean Architecture's
// inward-pointing dependency rule.
package domain

import "errors"

// Sentinel errors are stable identifiers that upper layers can match with
// errors.Is. They MUST stay free of dynamic data — when context is needed,
// wrap with fmt.Errorf("...: %w", ErrX) at the call site.
var (
	// Generic
	ErrNotFound      = errors.New("resource not found")
	ErrAlreadyExists = errors.New("resource already exists")
	ErrInvalidInput  = errors.New("invalid input")

	// User
	ErrUserNotFound      = errors.New("user not found")
	ErrUserAlreadyExists = errors.New("user already exists")
	ErrInvalidEmail      = errors.New("invalid email")

	// Wallet
	ErrWalletNotFound       = errors.New("wallet not found")
	ErrWalletAlreadyExists  = errors.New("wallet already exists for user/currency")
	ErrInsufficientBalance  = errors.New("insufficient balance")
	ErrCurrencyMismatch     = errors.New("currency mismatch between wallets")
	ErrUnsupportedCurrency  = errors.New("unsupported currency")
	ErrNonPositiveAmount    = errors.New("amount must be positive")

	// Transaction
	ErrTransactionNotFound = errors.New("transaction not found")
	ErrSameWalletTransfer  = errors.New("source and destination wallets must differ")
)
