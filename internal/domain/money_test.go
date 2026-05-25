package domain

import (
	"errors"
	"testing"
)

func TestNewMoney(t *testing.T) {
	tests := []struct {
		name     string
		amount   int64
		currency Currency
		wantErr  error
	}{
		{"valid TRY", 100, CurrencyTRY, nil},
		{"valid USD zero", 0, CurrencyUSD, nil},
		{"valid EUR negative", -50, CurrencyEUR, nil},
		{"unsupported currency", 100, Currency("XYZ"), ErrUnsupportedCurrency},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := NewMoney(tc.amount, tc.currency)
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestMoney_Arithmetic(t *testing.T) {
	tryMoney := func(amount int64) Money { return Money{Amount: amount, Currency: CurrencyTRY} }
	usdMoney := func(amount int64) Money { return Money{Amount: amount, Currency: CurrencyUSD} }

	tests := []struct {
		name    string
		op      func() (Money, error)
		want    int64
		wantErr error
	}{
		{
			name:    "Add same currency",
			op:      func() (Money, error) { return tryMoney(100).Add(tryMoney(50)) },
			want:    150,
			wantErr: nil,
		},
		{
			name:    "Sub same currency",
			op:      func() (Money, error) { return tryMoney(100).Sub(tryMoney(30)) },
			want:    70,
			wantErr: nil,
		},
		{
			name:    "Sub going negative is allowed (caller enforces)",
			op:      func() (Money, error) { return tryMoney(10).Sub(tryMoney(50)) },
			want:    -40,
			wantErr: nil,
		},
		{
			name:    "Add currency mismatch",
			op:      func() (Money, error) { return tryMoney(100).Add(usdMoney(50)) },
			wantErr: ErrCurrencyMismatch,
		},
		{
			name:    "Sub currency mismatch",
			op:      func() (Money, error) { return tryMoney(100).Sub(usdMoney(50)) },
			wantErr: ErrCurrencyMismatch,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := tc.op()
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.Amount != tc.want {
				t.Errorf("amount = %d, want %d", got.Amount, tc.want)
			}
		})
	}
}

func TestMoney_GreaterThanOrEqual(t *testing.T) {
	try100 := Money{Amount: 100, Currency: CurrencyTRY}
	try50 := Money{Amount: 50, Currency: CurrencyTRY}
	usd100 := Money{Amount: 100, Currency: CurrencyUSD}

	if ok, err := try100.GreaterThanOrEqual(try50); err != nil || !ok {
		t.Errorf("100 TRY >= 50 TRY should be true, got ok=%v err=%v", ok, err)
	}
	if ok, err := try50.GreaterThanOrEqual(try100); err != nil || ok {
		t.Errorf("50 TRY >= 100 TRY should be false, got ok=%v err=%v", ok, err)
	}
	if _, err := try100.GreaterThanOrEqual(usd100); !errors.Is(err, ErrCurrencyMismatch) {
		t.Errorf("expected ErrCurrencyMismatch, got %v", err)
	}
}
