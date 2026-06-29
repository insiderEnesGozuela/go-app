package service

import (
	"context"
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"

	"github.com/google/uuid"
	"github.com/insiderEnesGozuela/go-app/internal/domain"
	"github.com/insiderEnesGozuela/go-app/internal/repository"
)

// userService is the concrete implementation of UserService.
// It owns no state beyond the repositories it needs — stateless by design so
// it's trivially safe to share across goroutines.
type userService struct {
	users repository.UserRepository
}

// NewUserService wires the service against any UserRepository implementation.
// Callers (main, tests) supply the concrete repo; the service never imports postgres.
func NewUserService(users repository.UserRepository) UserService {
	return &userService{users: users}
}

// Register creates a new user, hashing the supplied plaintext password with
// bcrypt before it ever reaches the database. The service never stores or logs
// plaintext passwords; the domain.User it returns already has PasswordHash set.
//
// Why bcrypt cost 12? OWASP recommends >= 10. Cost 12 adds ~200 ms on a modern
// CPU — slow enough to make offline dictionary attacks impractical, fast enough
// for interactive logins. Adjust via config if hardware changes.
func (s *userService) Register(ctx context.Context, in RegisterUserInput) (*domain.User, error) {
	u, err := domain.NewUser(in.Email, in.FullName)
	if err != nil {
		return nil, fmt.Errorf("build user: %w", err)
	}

	// Check for existing email before hashing to avoid the bcrypt cost on a
	// duplicate email. The uniqueness check is advisory here — the DB unique
	// index is the hard guarantee. A TOCTOU between the check and the INSERT
	// results in a duplicate-key error which we translate to ErrUserAlreadyExists.
	_, err = s.users.GetByEmail(ctx, u.Email)
	if err == nil {
		return nil, fmt.Errorf("register user: %w", domain.ErrUserAlreadyExists)
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		return nil, fmt.Errorf("check existing email: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(in.Password), 12)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}
	u.PasswordHash = string(hash)

	if err := s.users.Create(ctx, u); err != nil {
		if errors.Is(err, domain.ErrAlreadyExists) {
			return nil, fmt.Errorf("register user: %w", domain.ErrUserAlreadyExists)
		}
		return nil, fmt.Errorf("persist user: %w", err)
	}
	return u, nil
}

func (s *userService) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	u, err := s.users.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("get user %s: %w", id, err)
	}
	return u, nil
}
