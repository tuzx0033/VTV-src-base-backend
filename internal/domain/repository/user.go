// Package repository holds port interfaces (implemented in internal/repository).
package repository

import (
	"context"
	"time"

	"vtv.vn/backend/internal/domain/model"
)

// UserListFilter is the input for listing users.
type UserListFilter struct {
	Search string // matches username / full_name / email
	Role   string // "" = any
	Status string // "" = any
	// Multi filters — OR semantics. Nếu set, ưu tiên hơn Role/Status single.
	Roles    []string
	Statuses []string
	Page     model.Page
}

// UserRepository persists User aggregates.
type UserRepository interface {
	// GetByID returns the user or (nil, nil) if not found.
	GetByID(ctx context.Context, id int64) (*model.User, error)
	// GetByUsername matches case-insensitively; returns (nil, nil) if not found.
	GetByUsername(ctx context.Context, username string) (*model.User, error)
	List(ctx context.Context, f UserListFilter) ([]model.User, int64, error)
	ExistsByUsername(ctx context.Context, username string) (bool, error)
	// Create inserts and sets u.ID.
	Create(ctx context.Context, u *model.User) error
	Update(ctx context.Context, u *model.User) error
	// StaffCodesWithPrefix returns all non-null staff codes matching '<prefix>%' (for next-code generation).
	StaffCodesWithPrefix(ctx context.Context, prefix string) ([]string, error)

	// SoftDelete marks the user as deleted (deleted_at = now). Idempotent: a
	// no-op on an already-deleted user. The caller is responsible for audit.
	SoftDelete(ctx context.Context, id int64, actorID int64) error
	// MustForcePasswordChange returns the value of users.force_password_change
	// for the user. Used by the auth middleware to gate every non-exempt
	// route when the user is still on a server-generated invite password.
	MustForcePasswordChange(ctx context.Context, id int64) (bool, error)

	// ── password reset tokens ──────────────────────────────────────────
	// SetResetToken stores the SHA-256 hash + expiry for a user. Pass empty
	// hash to clear the token.
	SetResetToken(ctx context.Context, userID int64, tokenHash string, expiresAt *time.Time) error
	// FindByResetTokenHash returns the user whose hash matches AND expiry
	// is still in the future. Returns (nil, nil) if no match.
	FindByResetTokenHash(ctx context.Context, tokenHash string) (*model.User, error)
}
