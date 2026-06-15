package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/insiderEnesGozuela/go-app/internal/domain"
)

// UserRepository is the Postgres implementation of repository.UserRepository.
// It holds the pool only as a *fallback*; every method routes through
// querierFrom so that when called inside a UnitOfWork it uses the active tx.
type UserRepository struct {
	pool *pgxpool.Pool
}

func NewUserRepository(pool *pgxpool.Pool) *UserRepository {
	return &UserRepository{pool: pool}
}

func (r *UserRepository) Create(ctx context.Context, u *domain.User) error {
	const q = `
		INSERT INTO users (id, email, password_hash, full_name, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6)`
	// Parameterized query ($1..$6) — never string-concatenate user input. This
	// is the SQL-injection defense mandated by CLAUDE.md, enforced uniformly.
	_, err := querierFrom(ctx, r.pool).Exec(ctx, q,
		u.ID, u.Email, u.PasswordHash, u.FullName, u.CreatedAt, u.UpdatedAt)
	if err != nil {
		// A duplicate email hits the unique index → ErrAlreadyExists, which the
		// service layer maps to "user already exists".
		return fmt.Errorf("insert user: %w", mapError(err, domain.ErrUserNotFound))
	}
	return nil
}

func (r *UserRepository) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, created_at, updated_at
		FROM users WHERE id = $1`
	row := querierFrom(ctx, r.pool).QueryRow(ctx, q, id)
	return scanUser(row)
}

func (r *UserRepository) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	const q = `
		SELECT id, email, password_hash, full_name, created_at, updated_at
		FROM users WHERE email = $1`
	row := querierFrom(ctx, r.pool).QueryRow(ctx, q, email)
	return scanUser(row)
}

// rowScanner is satisfied by pgx.Row (single row) — small interface so scanUser
// can be reused from both QueryRow paths and, later, from a Query loop.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanUser(row rowScanner) (*domain.User, error) {
	var u domain.User
	err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.FullName, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, fmt.Errorf("scan user: %w", mapError(err, domain.ErrUserNotFound))
	}
	return &u, nil
}
