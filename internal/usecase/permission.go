package usecase

import (
	"context"
	"strconv"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xpostgres"
)

type permissionUseCase struct {
	logger *xlogger.Logger
	tx     xpostgres.TxRunner
	perms  repository.PermissionRepository
	users  repository.UserRepository
	audit  repository.AuditRepository
}

// NewPermissionUseCase builds the user-permission use case.
func NewPermissionUseCase(
	logger *xlogger.Logger,
	tx xpostgres.TxRunner,
	perms repository.PermissionRepository,
	users repository.UserRepository,
	audit repository.AuditRepository,
) usecase.PermissionUseCase {
	return &permissionUseCase{logger: logger, tx: tx, perms: perms, users: users, audit: audit}
}

func (u *permissionUseCase) Create(ctx context.Context, in usecase.CreateUserPermissionInput) (*model.UserPermission, error) {
	if in.UserID <= 0 {
		return nil, xhttp.BadRequestErrorf("userId là bắt buộc")
	}
	var created *model.UserPermission
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		usr, err := u.users.GetByID(ctx, in.UserID)
		if err != nil {
			return err
		}
		if usr == nil {
			return xhttp.NotFoundErrorf("không tìm thấy người dùng %d", in.UserID)
		}
		existing, err := u.perms.GetByUserID(ctx, in.UserID)
		if err != nil {
			return err
		}
		if existing != nil {
			return xhttp.ConflictErrorf("người dùng đã có phân quyền (id=%d)", existing.ID)
		}
		actor := xhttp.UserID(ctx)
		up := &model.UserPermission{
			UserID:          in.UserID,
			Permissions:     model.PermissionMap{},
			PermissionCount: 0,
			CreatedBy:       actor,
			UpdatedBy:       actor,
		}
		if err := u.perms.Create(ctx, up); err != nil {
			return err
		}
		created = up
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserPermissionCreated, EntityType: "user_permission",
			EntityID: up.ID, EntityName: usr.Username,
			Summary: "tạo phân quyền cho người dùng " + usr.Username,
		})
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (u *permissionUseCase) GetByID(ctx context.Context, id int64) (*model.UserPermission, error) {
	up, err := u.perms.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if up == nil {
		return nil, xhttp.NotFoundErrorf("không tìm thấy phân quyền %d", id)
	}
	return up, nil
}

func (u *permissionUseCase) GetByUserID(ctx context.Context, userID int64) (*model.UserPermission, error) {
	up, err := u.perms.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if up == nil {
		return nil, xhttp.NotFoundErrorf("người dùng %d chưa có phân quyền", userID)
	}
	return up, nil
}

func (u *permissionUseCase) List(ctx context.Context, f repository.UserPermissionListFilter) ([]model.UserPermission, int64, error) {
	return u.perms.List(ctx, f)
}

func (u *permissionUseCase) Update(ctx context.Context, id int64, in usecase.UpdateUserPermissionInput) (*model.UserPermission, error) {
	var updated *model.UserPermission
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		up, err := u.perms.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if up == nil {
			return xhttp.NotFoundErrorf("không tìm thấy phân quyền %d", id)
		}
		cleaned := filterValidPermissions(in.Permissions)
		// Explicitly deny role-default permissions that the admin unchecked.
		usr, err := u.users.GetByID(ctx, up.UserID)
		if err == nil && usr != nil {
			defaults := model.RoleDefaultPermissions(string(usr.Role))
			for resource, acts := range defaults {
				for action := range acts {
					if _, exists := cleaned[resource]; !exists {
						cleaned[resource] = map[string]bool{}
					}
					if _, set := cleaned[resource][action]; !set {
						cleaned[resource][action] = false
					}
				}
			}
		}
		up.Permissions = cleaned
		up.PermissionCount = cleaned.Count()
		up.UpdatedBy = xhttp.UserID(ctx)
		if err := u.perms.Update(ctx, up); err != nil {
			return err
		}
		updated = up
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserPermissionUpdated, EntityType: "user_permission",
			EntityID: up.ID,
			Summary:  "cập nhật phân quyền user_id=" + intToString(up.UserID),
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *permissionUseCase) UpdateByUserID(ctx context.Context, userID int64, in usecase.UpdateUserPermissionInput) (*model.UserPermission, error) {
	var updated *model.UserPermission
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		usr, err := u.users.GetByID(ctx, userID)
		if err != nil {
			return err
		}
		if usr == nil {
			return xhttp.NotFoundErrorf("không tìm thấy người dùng %d", userID)
		}
		up, err := u.perms.GetByUserID(ctx, userID)
		if err != nil {
			return err
		}
		actor := xhttp.UserID(ctx)
		cleaned := filterValidPermissions(in.Permissions)
		// Explicitly deny role-default permissions that the admin unchecked.
		// Without this, MergeOver would re-grant them from role defaults.
		defaults := model.RoleDefaultPermissions(string(usr.Role))
		for resource, acts := range defaults {
			for action := range acts {
				if _, exists := cleaned[resource]; !exists {
					cleaned[resource] = map[string]bool{}
				}
				if _, set := cleaned[resource][action]; !set {
					cleaned[resource][action] = false
				}
			}
		}
		if up == nil {
			// Auto-create on first write — matches FE expectation that the
			// "phân quyền người dùng" page can save before a row exists.
			up = &model.UserPermission{
				UserID: userID, Permissions: cleaned, PermissionCount: cleaned.Count(),
				CreatedBy: actor, UpdatedBy: actor,
			}
			if err := u.perms.Create(ctx, up); err != nil {
				return err
			}
		} else {
			up.Permissions = cleaned
			up.PermissionCount = cleaned.Count()
			up.UpdatedBy = actor
			if err := u.perms.Update(ctx, up); err != nil {
				return err
			}
		}
		updated = up
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserPermissionUpdated, EntityType: "user_permission",
			EntityID: up.ID, EntityName: usr.Username,
			Summary: "cập nhật phân quyền cho " + usr.Username,
		})
	})
	if err != nil {
		return nil, err
	}
	return updated, nil
}

func (u *permissionUseCase) Delete(ctx context.Context, id int64) error {
	return u.tx.Run(ctx, func(ctx context.Context) error {
		up, err := u.perms.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if up == nil {
			return xhttp.NotFoundErrorf("không tìm thấy phân quyền %d", id)
		}
		if err := u.perms.Delete(ctx, id); err != nil {
			return err
		}
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserPermissionDeleted, EntityType: "user_permission",
			EntityID: id, Summary: "xoá phân quyền user_id=" + intToString(up.UserID),
		})
	})
}

// EffectivePermissions tính permission map effective cho user:
//   - admin → wildcard "*.*" god-mode
//   - non-admin → roleDefaults(role) ⊕ userOverrides (user_permissions row)
//
// userOverrides có thể GRANT (true) hoặc REVOKE (false) đè lên defaults.
// Trả về map đã merge — FE dùng để gate UI; middleware HasPermission dùng để
// gate route (cùng logic = consistent).
func (u *permissionUseCase) EffectivePermissions(ctx context.Context, userID int64, isAdmin bool) (model.PermissionMap, error) {
	if isAdmin {
		return model.PermissionMap{"*": {"*": true}}, nil
	}
	// Cần role để chọn defaults — load user.
	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return model.PermissionMap{}, nil
	}
	defaults := model.RoleDefaultPermissions(string(user.Role))
	up, err := u.perms.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if up == nil {
		return defaults, nil
	}
	return defaults.MergeOver(up.Permissions), nil
}

// filterValidPermissions drops any (resource, action) pair that isn't declared
// in consts.PermissionsDefinitions. Explicit false values (denials) are kept
// so that an admin can revoke a role-default permission for a specific user.
func filterValidPermissions(in model.PermissionMap) model.PermissionMap {
	ref := flatReferenceMatrix()
	out := model.PermissionMap{}
	for resource, actions := range in {
		refActs, ok := ref[resource]
		if !ok {
			continue
		}
		for action, granted := range actions {
			if !refActs[action] {
				continue
			}
			if out[resource] == nil {
				out[resource] = map[string]bool{}
			}
			out[resource][action] = granted
		}
	}
	return out
}

// flatReferenceMatrix flattens PermissionsDefinitions into {resource: {action: true}}.
func flatReferenceMatrix() model.PermissionMap {
	out := model.PermissionMap{}
	var walk func(p consts.Permission)
	walk = func(p consts.Permission) {
		acts := make(map[string]bool, len(p.Actions))
		for _, a := range p.Actions {
			acts[a] = true
		}
		out[p.ID] = acts
		for _, c := range p.Children {
			walk(c)
		}
	}
	for _, p := range consts.PermissionsDefinitions {
		walk(p)
	}
	return out
}

func intToString(v int64) string { return strconv.FormatInt(v, 10) }
