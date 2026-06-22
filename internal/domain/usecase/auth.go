// Package usecase holds use-case interfaces (implemented in internal/usecase).
package usecase

import (
	"context"

	"vtv.vn/backend/internal/domain/model"
)

// AuthUseCase covers login / profile / password.
type AuthUseCase interface {
	// Login validates credentials and returns a signed JWT + the user.
	// Errors: model.ErrInvalidCredentials, model.ErrAccountLocked.
	Login(ctx context.Context, username, password string) (token string, user *model.User, err error)
	// Me returns the currently-authenticated user (from ctx user id).
	Me(ctx context.Context) (*model.User, error)
	// ChangePassword changes the current user's password (requires the current one).
	ChangePassword(ctx context.Context, currentPassword, newPassword string) error
	// Logout revokes the current token so it can no longer be used.
	Logout(ctx context.Context) error

	// RequestPasswordReset generates a single-use token for the given
	// username/email if a matching active user exists. Returns the *plain*
	// token (BE caller is responsible for delivery channel — email, log,
	// admin notification). Always returns (token, nil) or ("", nil) so the
	// HTTP layer can hide whether the user exists (anti-enumeration).
	RequestPasswordReset(ctx context.Context, identifier string) (string, error)
	// ResetPassword consumes a reset token and sets a new password.
	// Errors: invalid/expired token, weak password.
	ResetPassword(ctx context.Context, token, newPassword string) error
}
