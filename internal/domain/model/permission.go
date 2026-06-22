package model

import (
	"time"

	"vtv.vn/backend/internal/domain/consts"
)

// PermissionMap is the nested {resource: {action: true}} grant tree.
//
// Stored as JSONB in `user_permissions.permissions`. The middleware path reads
// PermissionMap["users"]["write"] etc. — denying when missing or false.
type PermissionMap map[string]map[string]bool

// HasAction returns true when the map grants `action` on `resource`. Missing
// keys count as "no" — defaults to deny. Wildcard "*"."*" short-circuits to
// allow (admin god-mode).
func (p PermissionMap) HasAction(resource, action string) bool {
	if p == nil {
		return false
	}
	if wild, ok := p["*"]; ok && wild["*"] {
		return true
	}
	acts, ok := p[resource]
	if !ok {
		return false
	}
	return acts[action]
}

// MergeOver layers `override` on top of `p` — true grants, false revokes.
// Original maps untouched (deep-cloned). Use for layering user_permissions
// overrides on top of role defaults.
func (p PermissionMap) MergeOver(override PermissionMap) PermissionMap {
	out := PermissionMap{}
	for r, acts := range p {
		copyActs := make(map[string]bool, len(acts))
		for a, v := range acts {
			copyActs[a] = v
		}
		out[r] = copyActs
	}
	for r, acts := range override {
		if _, ok := out[r]; !ok {
			out[r] = map[string]bool{}
		}
		for a, v := range acts {
			out[r][a] = v
		}
	}
	return out
}

// Count returns the total number of granted (resource, action) pairs.
func (p PermissionMap) Count() int {
	n := 0
	for _, acts := range p {
		for _, granted := range acts {
			if granted {
				n++
			}
		}
	}
	return n
}

// RoleDefaultPermissions trả về default grants theo role. Mirror với FE
// roleToPermissions() để consistent giữa BE middleware (RequirePermission) và
// FE gate (sidebar / button visibility). Sửa CẢ 2 file khi đổi role model.
//
//   - admin   → wildcard (caller xử lý riêng; hàm này trả {} cho admin)
//   - manager → read+write+delete trên operational resources
//   - staff   → read trên hầu hết, write trên resource staff trực tiếp tác động
//
// Hàm sống ở model package thay vì usecase để middleware/repository import
// được mà không reverse layering.
func RoleDefaultPermissions(role string) PermissionMap {
	rw := map[string]bool{consts.ActionRead: true, consts.ActionWrite: true}
	rwd := map[string]bool{consts.ActionRead: true, consts.ActionWrite: true, consts.ActionDelete: true}
	r := map[string]bool{consts.ActionRead: true}

	switch role {
	case consts.RoleManager:
		// Manager: full quản lý user + phân quyền, đọc dashboard, gửi thông báo.
		// Tách bạch khỏi admin (admin = wildcard); admin có thể thu hồi per-user.
		return PermissionMap{
			consts.ResourceDashboard:       r,
			consts.ResourceUsers:           rwd,
			consts.ResourceUserPermissions: rwd,
			consts.ResourceAnnouncements:   rwd,
			consts.ResourceNotifications:   rw,
			consts.ResourceSettings:        r,
		}
	case consts.RoleStaff:
		// Staff: đọc dashboard + thông báo, không quản trị.
		return PermissionMap{
			consts.ResourceDashboard:     r,
			consts.ResourceAnnouncements: r,
			consts.ResourceNotifications: rw,
		}
	}
	return PermissionMap{}
}

// UserPermission is the aggregate stored in `user_permissions`. One per user.
type UserPermission struct {
	ID              int64
	UserID          int64
	PermissionCount int
	Permissions     PermissionMap
	CreatedAt       time.Time // UTC
	UpdatedAt       time.Time // UTC
	CreatedBy       int64
	UpdatedBy       int64

	// View-model enrichment (filled by repository on List/Get).
	UserFullName  string
	UserUsername  string
	UserStaffCode *string
}
