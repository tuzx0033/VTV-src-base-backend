package usecase

import (
	"context"

	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
)

// PermissionUseCase orchestrates user-permission writes + reads.
type PermissionUseCase interface {
	Create(ctx context.Context, in CreateUserPermissionInput) (*model.UserPermission, error)
	GetByID(ctx context.Context, id int64) (*model.UserPermission, error)
	GetByUserID(ctx context.Context, userID int64) (*model.UserPermission, error)
	List(ctx context.Context, f repository.UserPermissionListFilter) ([]model.UserPermission, int64, error)
	Update(ctx context.Context, id int64, in UpdateUserPermissionInput) (*model.UserPermission, error)
	UpdateByUserID(ctx context.Context, userID int64, in UpdateUserPermissionInput) (*model.UserPermission, error)
	Delete(ctx context.Context, id int64) error
	// EffectivePermissions returns the flat grant matrix for a caller.
	// `isAdmin` short-circuits to the full reference matrix.
	EffectivePermissions(ctx context.Context, userID int64, isAdmin bool) (model.PermissionMap, error)
}

// CreateUserPermissionInput creates an empty row for a user.
type CreateUserPermissionInput struct {
	UserID int64
}

// UpdateUserPermissionInput replaces the grant tree.
type UpdateUserPermissionInput struct {
	Permissions model.PermissionMap
}
