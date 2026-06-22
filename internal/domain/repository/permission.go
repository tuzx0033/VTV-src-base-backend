package repository

import (
	"context"

	"vtv.vn/backend/internal/domain/model"
)

// UserPermissionListFilter is the input for listing user-permission rows.
type UserPermissionListFilter struct {
	Search string // matches user username / full_name / staff_code
	UserID *int64 // exact match
	Page   model.Page
}

// PermissionRepository persists per-user permission grants and answers the
// hot-path "does X have action Y on resource R" question for middleware.
type PermissionRepository interface {
	// GetByID returns the row or (nil, nil) if not found.
	GetByID(ctx context.Context, id int64) (*model.UserPermission, error)
	// GetByUserID returns the row or (nil, nil) if not found.
	GetByUserID(ctx context.Context, userID int64) (*model.UserPermission, error)
	// List returns rows + total matching the filter, with user fields joined.
	List(ctx context.Context, f UserPermissionListFilter) ([]model.UserPermission, int64, error)
	// Create inserts a row and sets up.ID.
	Create(ctx context.Context, up *model.UserPermission) error
	// Update replaces permissions / permission_count / updated_* for up.ID.
	Update(ctx context.Context, up *model.UserPermission) error
	// Delete removes the row. Idempotent (no-op if missing).
	Delete(ctx context.Context, id int64) error
	// HasPermission is the hot path used by the auth middleware. Returns true
	// when the user has explicitly been granted (resource, action).
	HasPermission(ctx context.Context, userID int64, resource, action string) (bool, error)
	// CountPermissionsAvailable returns the total number of (resource, action)
	// pairs declared in PermissionsDefinitions — used by /permissions/me admin
	// callers to render "X/Y permissions granted".
	CountPermissionsAvailable() int
}
