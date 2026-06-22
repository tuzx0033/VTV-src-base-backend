package dto

import (
	"time"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
)

// PermissionDefinition mirrors consts.Permission for the FE permission picker.
type PermissionDefinition struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Actions     []string               `json:"actions"`
	Children    []PermissionDefinition `json:"children,omitempty"`
}

// ActionDefinition is the Vietnamese-labelled action.
type ActionDefinition struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// ToPermissionDefinitions converts the consts tree to the DTO tree.
func ToPermissionDefinitions(in []consts.Permission) []PermissionDefinition {
	out := make([]PermissionDefinition, 0, len(in))
	for _, p := range in {
		out = append(out, PermissionDefinition{
			ID: p.ID, Name: p.Name, Description: p.Description,
			Actions:  p.Actions,
			Children: ToPermissionDefinitions(p.Children),
		})
	}
	return out
}

// ToActionDefinitions converts the consts action list to the DTO list.
func ToActionDefinitions(in []consts.Action) []ActionDefinition {
	out := make([]ActionDefinition, 0, len(in))
	for _, a := range in {
		out = append(out, ActionDefinition{ID: a.ID, Name: a.Name})
	}
	return out
}

// UserPermissionResponse is the public shape of a user permission row.
type UserPermissionResponse struct {
	ID              int64                      `json:"id"`
	UserID          int64                      `json:"userId"`
	UserFullName    string                     `json:"userFullName,omitempty"`
	UserUsername    string                     `json:"userUsername,omitempty"`
	UserStaffCode   *string                    `json:"userStaffCode,omitempty"`
	PermissionCount int                        `json:"permissionCount"`
	Permissions     map[string]map[string]bool `json:"permissions"`
	// Effective is the merged permission map (role defaults + overrides).
	// Only populated by GetByUser; other endpoints leave it nil (omitted).
	Effective map[string]map[string]bool `json:"effective,omitempty"`
	CreatedAt string                     `json:"createdAt"`
	UpdatedAt string                     `json:"updatedAt"`
	CreatedBy int64                      `json:"createdBy"`
	UpdatedBy int64                      `json:"updatedBy"`
}

// ToUserPermissionResponse maps domain → DTO.
func ToUserPermissionResponse(p *model.UserPermission) UserPermissionResponse {
	perms := map[string]map[string]bool(p.Permissions)
	if perms == nil {
		perms = map[string]map[string]bool{}
	}
	return UserPermissionResponse{
		ID: p.ID, UserID: p.UserID,
		UserFullName: p.UserFullName, UserUsername: p.UserUsername, UserStaffCode: p.UserStaffCode,
		PermissionCount: p.PermissionCount,
		Permissions:     perms,
		CreatedAt:       p.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:       p.UpdatedAt.UTC().Format(time.RFC3339),
		CreatedBy:       p.CreatedBy, UpdatedBy: p.UpdatedBy,
	}
}

// ToUserPermissionResponses maps a slice.
func ToUserPermissionResponses(ps []model.UserPermission) []UserPermissionResponse {
	out := make([]UserPermissionResponse, 0, len(ps))
	for i := range ps {
		out = append(out, ToUserPermissionResponse(&ps[i]))
	}
	return out
}

// CreateUserPermissionRequest creates an empty row for a user (initial grant
// matrix is empty — caller updates afterwards). Idempotent vs. existing row.
type CreateUserPermissionRequest struct {
	UserID int64 `json:"userId" validate:"required,min=1"`
}

// UpdateUserPermissionRequest replaces the JSONB grant tree.
type UpdateUserPermissionRequest struct {
	Permissions map[string]map[string]bool `json:"permissions" validate:"required"`
}

// UpdateUserPermissionsByUserRequest is the FE-compat shape:
// PUT /users/:id/permissions with the full grant tree.
type UpdateUserPermissionsByUserRequest struct {
	Permissions map[string]map[string]bool `json:"permissions" validate:"required"`
}

// ListUserPermissionsQuery is the query for GET /user-permissions.
type ListUserPermissionsQuery struct {
	Page   int    `query:"page"   default:"1"  validate:"omitempty,min=1"`
	Limit  int    `query:"limit"  default:"20" validate:"omitempty,min=1,max=200"`
	Search string `query:"search" validate:"omitempty,max=100"`
	UserID *int64 `query:"userId" validate:"omitempty,min=1"`
}

// MyPermissionsResponse is the effective grant matrix returned to a caller.
// Admins get the entire flattened reference matrix; everyone else gets their
// stored row (or empty if none).
type MyPermissionsResponse struct {
	UserID      int64                      `json:"userId"`
	Role        string                     `json:"role"`
	IsAdmin     bool                       `json:"isAdmin"`
	Permissions map[string]map[string]bool `json:"permissions"`
}
