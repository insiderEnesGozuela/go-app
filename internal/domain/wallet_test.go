package domain

import (
	"errors"
	"testing"

	"github.com/google/uuid"
)

func TestNewWallet(t *testing.T) {
	uid := uuid.New()
	tests := []struct {
		name     string
		userID   uuid.UUID
		currency Currency
		wantErr  error
	}{
		{"valid TRY wallet", uid, CurrencyTRY, nil},
		{"nil user id", uuid.Nil, CurrencyTRY, ErrInvalidInput},
		{"unsupported currency", uid, Currency("XYZ"), ErrUnsupportedCurrency},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w, err := NewWallet(tc.userID, tc.currency)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if w.Balance.Amount != 0 {
				t.Errorf("new wallet should have zero balance, got %d", w.Balance.Amount)
			}
			if w.Version != 1 {
				t.Errorf("new wallet should have version 1, got %d", w.Version)
			}
		})
	}
}

func TestWallet_Credit(t *testing.T) {
	w, err := NewWallet(uuid.New(), CurrencyTRY)
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	tests := []struct {
		name    string
		amount  Money
		wantErr error
		wantBal int64
		wantVer int64
	}{
		{
			name:    "credit 100",
			amount:  Money{Amount: 100, Currency: CurrencyTRY},
			wantBal: 100, wantVer: 2,
		},
		{
			name:    "credit zero rejected",
			amount:  Money{Amount: 0, Currency: CurrencyTRY},
			wantErr: ErrNonPositiveAmount,
			wantBal: 100, wantVer: 2,
		},
		{
			name:    "credit negative rejected",
			amount:  Money{Amount: -10, Currency: CurrencyTRY},
			wantErr: ErrNonPositiveAmount,
			wantBal: 100, wantVer: 2,
		},
		{
			name:    "credit wrong currency",
			amount:  Money{Amount: 50, Currency: CurrencyUSD},
			wantErr: ErrCurrencyMismatch,
			wantBal: 100, wantVer: 2,
		},
		{
			name:    "credit 50 more",
			amount:  Money{Amount: 50, Currency: CurrencyTRY},
			wantBal: 150, wantVer: 3,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := w.Credit(tc.amount)
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if w.Balance.Amount != tc.wantBal {
				t.Errorf("balance = %d, want %d", w.Balance.Amount, tc.wantBal)
			}
			if w.Version != tc.wantVer {
				t.Errorf("version = %d, want %d", w.Version, tc.wantVer)
			}
		})
	}
}

func TestWallet_Debit(t *testing.T) {
	w, _ := NewWallet(uuid.New(), CurrencyTRY)
	_ = w.Credit(Money{Amount: 100, Currency: CurrencyTRY}) // start with 100

	tests := []struct {
		name    string
		amount  Money
		wantErr error
		wantBal int64
	}{
		{"debit 40 ok", Money{Amount: 40, Currency: CurrencyTRY}, nil, 60},
		{"debit exceeding balance", Money{Amount: 1000, Currency: CurrencyTRY}, ErrInsufficientBalance, 60},
		{"debit zero", Money{Amount: 0, Currency: CurrencyTRY}, ErrNonPositiveAmount, 60},
		{"debit negative", Money{Amount: -5, Currency: CurrencyTRY}, ErrNonPositiveAmount, 60},
		{"debit currency mismatch", Money{Amount: 10, Currency: CurrencyUSD}, ErrCurrencyMismatch, 60},
		{"debit remaining 60", Money{Amount: 60, Currency: CurrencyTRY}, nil, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := w.Debit(tc.amount)
			if tc.wantErr != nil && !errors.Is(err, tc.wantErr) {
				t.Fatalf("expected %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr == nil && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if w.Balance.Amount != tc.wantBal {
				t.Errorf("balance = %d, want %d", w.Balance.Amount, tc.wantBal)
			}
		})
	}
}
