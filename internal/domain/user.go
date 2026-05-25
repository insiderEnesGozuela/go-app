package domain

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
)

// emailRegex is intentionally lenient — RFC 5322 in full is huge and most
// rejections in finance come from the downstream verification flow anyway.
// We only catch obviously malformed inputs at the API boundary.
var emailRegex = regexp.MustCompile(`^[^\s@]+@[^\s@]+\.[^\s@]+$`)

type User struct {
	ID           uuid.UUID
	Email        string
	PasswordHash string
	FullName     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// NewUser constructs a validated User. PasswordHash is set later by the auth
// service (so this layer never sees plaintext passwords).
func NewUser(email, fullName string) (*User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	fullName = strings.TrimSpace(fullName)

	if !emailRegex.MatchString(email) {
		return nil, fmt.Errorf("%w: %q", ErrInvalidEmail, email)
	}
	if fullName == "" {
		return nil, fmt.Errorf("%w: full_name is required", ErrInvalidInput)
	}

	now := time.Now().UTC()
	return &User{
		ID:        uuid.New(),
		Email:     email,
		FullName:  fullName,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
