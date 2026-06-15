package postgres

import (
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/insiderEnesGozuela/go-app/internal/domain"
)

// PostgreSQL error codes we care about. Full list: https://www.postgresql.org/docs/current/errcodes-appendix.html
const (
	pgUniqueViolation     = "23505" // duplicate key
	pgForeignKeyViolation = "23503" // referenced row missing
	pgCheckViolation      = "23514" // CHECK constraint failed
)

// mapError translates raw pgx/Postgres errors into domain sentinel errors so
// upper layers can branch on domain.ErrX with errors.Is and never need to know
// what database is underneath. This is the seam that keeps the service layer
// database-agnostic.
//
// notFound is the domain error to return for pgx.ErrNoRows (it differs per
// table: ErrUserNotFound vs ErrWalletNotFound), so the caller supplies it.
func mapError(err error, notFound error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return notFound
	}

	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case pgUniqueViolation:
			return domain.ErrAlreadyExists
		case pgForeignKeyViolation:
			// A referenced row (e.g. user for a wallet, wallet for a tx) is
			// missing. From the caller's perspective this is invalid input.
			return domain.ErrInvalidInput
		case pgCheckViolation:
			// A DB-level invariant was violated (e.g. balance < 0). The domain
			// should have caught this first; surfacing it as invalid input
			// rather than a raw 500 keeps the contract honest.
			return domain.ErrInvalidInput
		}
	}
	// Unknown error: return as-is so we don't mask infrastructure failures
	// (connection reset, deadlock, etc.) as domain errors.
	return err
}
