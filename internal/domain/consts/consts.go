// Package consts holds domain-level constants: table names, role names,
// permission strings, audit verbs, redis key prefixes.
package consts

// User roles. admin = god-mode (wildcard permissions); manager/staff are the
// default operational roles. Extend per project as needed (the users table
// role CHECK constraint must be updated in tandem — see migrations/0001).
const (
	RoleAdmin   = "admin"
	RoleManager = "manager"
	RoleStaff   = "staff"
)

// Table names (single source of truth — repositories reference these).
const (
	TableUsers           = "users"
	TableAuditLog        = "audit_log"
	TableUserPermissions = "user_permissions"
	TableSettings        = "settings"
	TableAnnouncements   = "announcements"
	TableNotifications   = "notifications"
)

// Audit verbs (taxonomy: <entity>.<verb>).
const (
	AuditUserCreated       = "user.created"
	AuditUserInvited       = "user.invited"
	AuditUserUpdated       = "user.updated"
	AuditUserPasswordReset = "user.password_reset"
	AuditUserStatusToggled = "user.status_toggled"
	AuditUserLocked        = "user.locked"
	AuditUserUnlocked      = "user.unlocked"
	AuditUserDeleted       = "user.deleted"

	AuditAuthLoginSucceeded = "auth.login_succeeded"
	AuditAuthLoginFailed    = "auth.login_failed"

	AuditAnnouncementCreated = "announcement.created"
	AuditAnnouncementUpdated = "announcement.updated"
	AuditAnnouncementDeleted = "announcement.deleted"

	AuditUserPermissionCreated = "user_permission.created"
	AuditUserPermissionUpdated = "user_permission.updated"
	AuditUserPermissionDeleted = "user_permission.deleted"
)

// Redis key prefixes.
const (
	RedisKeyUserProfile    = "app:user_profile:" // + userID
	RedisKeyTokenBlacklist = "app:token_blacklist:"
	RedisKeyUserPerms      = "app:user_perms:" // + userID -> SET of permission strings
)
