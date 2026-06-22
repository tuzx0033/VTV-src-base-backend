package model

import (
	"vtv.vn/backend/internal/domain/consts"
)

// Role is a user role.
type Role string

const (
	RoleAdmin   Role = consts.RoleAdmin
	RoleManager Role = consts.RoleManager
	RoleStaff   Role = consts.RoleStaff
)

// Valid reports whether r is a known role.
func (r Role) Valid() bool {
	switch r {
	case RoleAdmin, RoleManager, RoleStaff:
		return true
	}
	return false
}

// UserStatus is account status.
type UserStatus string

const (
	UserStatusActive UserStatus = "active"
	UserStatusLocked UserStatus = "locked"
)

// User is the account aggregate. State transitions are methods on it.
type User struct {
	ID                  int64
	Username            string
	PasswordHash        string
	FullName            string
	Email               *string
	Phone               *string
	Role                Role
	Status              UserStatus
	StaffCode           *string
	AvatarURL           *string
	ForcePasswordChange bool
	// EmploymentType — fulltime|parttime, dùng cho ngưỡng KPI lương booker.
	EmploymentType EmploymentType
	AuditFields
}

// IsLocked reports whether the account is locked.
func (u *User) IsLocked() bool { return u.Status == UserStatusLocked }

// IsActive reports whether the account is active (and not soft-deleted).
func (u *User) IsActive() bool { return u.Status == UserStatusActive && !u.IsDeleted() }

// Lock locks the account.
func (u *User) Lock() error {
	if u.IsDeleted() {
		return ErrUserDeleted
	}
	u.Status = UserStatusLocked
	return nil
}

// Unlock activates the account.
func (u *User) Unlock() error {
	if u.IsDeleted() {
		return ErrUserDeleted
	}
	u.Status = UserStatusActive
	return nil
}

// SetPasswordHash sets a new (already-hashed) password.
func (u *User) SetPasswordHash(hash string) { u.PasswordHash = hash }

// ClearForcePasswordChange marks the user as having completed the mandatory
// first-login password change. Called from /auth/change-password when the
// user finishes the forced rotation flow.
func (u *User) ClearForcePasswordChange() { u.ForcePasswordChange = false }
