package domain

import (
	"errors"
	"testing"
)

func TestNewUser(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		fullName string
		wantErr  error
		wantMail string
	}{
		{"valid lowercase", "alice@example.com", "Alice", nil, "alice@example.com"},
		{"valid normalized", "  Alice@EXAMPLE.com  ", "Alice", nil, "alice@example.com"},
		{"missing at", "aliceexample.com", "Alice", ErrInvalidEmail, ""},
		{"missing dot", "alice@examplecom", "Alice", ErrInvalidEmail, ""},
		{"empty name", "alice@example.com", "  ", ErrInvalidInput, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			u, err := NewUser(tc.email, tc.fullName)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("expected %v, got %v", tc.wantErr, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if u.Email != tc.wantMail {
				t.Errorf("email = %q, want %q", u.Email, tc.wantMail)
			}
		})
	}
}
