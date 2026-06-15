package postgres

import (
	"errors"
	"fmt"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/insiderEnesGozuela/go-app/internal/domain"
)

// TestMapError covers the translation seam between raw Postgres errors and
// domain sentinel errors. This logic is pure (no DB needed), so it's the
// highest-value unit test in the package: every repository method funnels its
// errors through mapError, and getting a code→sentinel mapping wrong would
// silently turn "duplicate email" into a 500. Table-driven per CLAUDE.md.
func TestMapError(t *testing.T) {
	notFound := domain.ErrUserNotFound

	tests := []struct {
		name string
		in   error
		want error // checked with errors.Is
	}{
		{
			name: "nil passes through",
			in:   nil,
			want: nil,
		},
		{
			name: "no rows maps to supplied not-found",
			in:   pgx.ErrNoRows,
			want: notFound,
		},
		{
			name: "no rows survives wrapping",
			in:   fmt.Errorf("scan: %w", pgx.ErrNoRows),
			want: notFound,
		},
		{
			name: "unique violation maps to already-exists",
			in:   &pgconn.PgError{Code: pgUniqueViolation},
			want: domain.ErrAlreadyExists,
		},
		{
			name: "foreign key violation maps to invalid input",
			in:   &pgconn.PgError{Code: pgForeignKeyViolation},
			want: domain.ErrInvalidInput,
		},
		{
			name: "check violation maps to invalid input",
			in:   &pgconn.PgError{Code: pgCheckViolation},
			want: domain.ErrInvalidInput,
		},
		{
			name: "unique violation survives wrapping",
			in:   fmt.Errorf("insert: %w", &pgconn.PgError{Code: pgUniqueViolation}),
			want: domain.ErrAlreadyExists,
		},
		{
			name: "unknown pg code returns original error",
			in:   &pgconn.PgError{Code: "40001"}, // serialization failure
			want: nil,                            // expect the original, not a domain sentinel
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := mapError(tc.in, notFound)

			if tc.in == nil {
				if got != nil {
					t.Fatalf("expected nil, got %v", got)
				}
				return
			}

			if tc.want == nil {
				// "unknown" case: must return the original error untouched so we
				// don't mask infra failures as domain errors.
				if !errors.Is(got, tc.in) {
					t.Fatalf("expected original error %v to pass through, got %v", tc.in, got)
				}
				return
			}

			if !errors.Is(got, tc.want) {
				t.Fatalf("expected errors.Is(%v, %v), got false", got, tc.want)
			}
		})
	}
}
