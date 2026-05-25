package domain

import (
	"fmt"
)

// Currency is a 3-letter ISO 4217 code. Kept as a typed string so the compiler
// catches accidental cross-currency arithmetic at function boundaries.
type Currency string

const (
	CurrencyTRY Currency = "TRY"
	CurrencyUSD Currency = "USD"
	CurrencyEUR Currency = "EUR"
)

func (c Currency) Valid() bool {
	switch c {
	case CurrencyTRY, CurrencyUSD, CurrencyEUR:
		return true
	}
	return false
}

// Money is an integer-minor-unit value (e.g. kuruş for TRY, cents for USD).
// We never use float for money — IEEE 754 rounding silently destroys cents
// at scale. All arithmetic happens in int64 minor units; conversion to a
// human-readable major unit happens only at the presentation boundary.
type Money struct {
	Amount   int64 // minor units, may be negative for internal calculations
	Currency Currency
}

func NewMoney(amount int64, currency Currency) (Money, error) {
	if !currency.Valid() {
		return Money{}, fmt.Errorf("%w: %q", ErrUnsupportedCurrency, currency)
	}
	return Money{Amount: amount, Currency: currency}, nil
}

func (m Money) IsZero() bool     { return m.Amount == 0 }
func (m Money) IsPositive() bool { return m.Amount > 0 }
func (m Money) IsNegative() bool { return m.Amount < 0 }

// Add returns m + other; errors if currencies differ. Returning a new value
// (not mutating) keeps Money safe to pass by value across goroutines.
func (m Money) Add(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.Currency, other.Currency)
	}
	return Money{Amount: m.Amount + other.Amount, Currency: m.Currency}, nil
}

func (m Money) Sub(other Money) (Money, error) {
	if m.Currency != other.Currency {
		return Money{}, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.Currency, other.Currency)
	}
	return Money{Amount: m.Amount - other.Amount, Currency: m.Currency}, nil
}

// GreaterThanOrEqual is used by the balance check in withdrawals/transfers.
func (m Money) GreaterThanOrEqual(other Money) (bool, error) {
	if m.Currency != other.Currency {
		return false, fmt.Errorf("%w: %s vs %s", ErrCurrencyMismatch, m.Currency, other.Currency)
	}
	return m.Amount >= other.Amount, nil
}

func (m Money) String() string {
	return fmt.Sprintf("%d %s", m.Amount, m.Currency)
}
