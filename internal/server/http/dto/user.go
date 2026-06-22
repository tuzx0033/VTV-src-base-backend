// Package dto holds HTTP request/response shapes + mappers. The OpenAPI spec is
// generated from these (via swaggo). FE generates its types from the spec.
package dto

import (
	"time"

	"vtv.vn/backend/internal/domain/model"
)

// UserResponse is the public shape of a user.
type UserResponse struct {
	ID                  int64   `json:"id"`
	Username            string  `json:"username"`
	FullName            string  `json:"fullName"`
	Email               *string `json:"email"`
	Phone               *string `json:"phone"`
	Role                string  `json:"role"`
	Status              string  `json:"status"`
	StaffCode           *string `json:"staffCode"`
	AvatarURL           *string `json:"avatarUrl"`
	EmploymentType      string  `json:"employmentType"`
	ForcePasswordChange bool    `json:"forcePasswordChange"`
	CreatedAt           string  `json:"createdAt"` // RFC3339 UTC
	UpdatedAt           string  `json:"updatedAt"` // RFC3339 UTC
}

// ToUserResponse maps a domain user to the response DTO.
func ToUserResponse(u *model.User) UserResponse {
	return UserResponse{
		ID: u.ID, Username: u.Username, FullName: u.FullName, Email: u.Email, Phone: u.Phone,
		Role: string(u.Role), Status: string(u.Status), StaffCode: u.StaffCode, AvatarURL: u.AvatarURL,
		EmploymentType:      string(u.EmploymentType),
		ForcePasswordChange: u.ForcePasswordChange,
		CreatedAt:           u.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: u.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

// ToUserResponses maps a slice.
func ToUserResponses(us []model.User) []UserResponse {
	out := make([]UserResponse, 0, len(us))
	for i := range us {
		out = append(out, ToUserResponse(&us[i]))
	}
	return out
}

// CreateUserRequest is the body of POST /users. Password min=8 matches OWASP
// ASVS L1 minimum for offline-attack-resistant secrets (paired with bcrypt
// cost=12 in pkg/xcrypto).
type CreateUserRequest struct {
	Username       string  `json:"username" validate:"required,min=3,max=100"`
	Password       string  `json:"password" validate:"required,min=8,max=128"`
	FullName       string  `json:"fullName" validate:"required,min=1,max=255"`
	Email          *string `json:"email"    validate:"omitempty,email,max=255"`
	Phone          *string `json:"phone"    validate:"omitempty,max=32"`
	StaffCode      *string `json:"staffCode" validate:"omitempty,max=32"`
	Role           string  `json:"role"     validate:"required,oneof=admin manager staff"`
	EmploymentType string  `json:"employmentType" validate:"omitempty,oneof=fulltime parttime"`
	// Khi true: user vừa được tạo sẽ bị middleware ép đổi mật khẩu trước khi
	// gọi bất kỳ endpoint non-exempt nào (login response trả forcePasswordChange=true
	// → FE redirect /change-password-required). Dùng cho flow admin tạo staff
	// với mật khẩu tạm và muốn họ tự đổi sau.
	ForcePasswordChange bool `json:"forcePasswordChange,omitempty"`
}

// UpdateUserRequest is the body of PUT /users/:id (nil = leave unchanged).
type UpdateUserRequest struct {
	FullName       *string `json:"fullName" validate:"omitempty,min=1,max=255"`
	Email          *string `json:"email"    validate:"omitempty,email,max=255"`
	Phone          *string `json:"phone"    validate:"omitempty,max=32"`
	Role           *string `json:"role"     validate:"omitempty,oneof=admin manager staff"`
	StaffCode      *string `json:"staffCode" validate:"omitempty,max=32"`
	EmploymentType *string `json:"employmentType" validate:"omitempty,oneof=fulltime parttime"`
}

// ResetPasswordRequest is the body of POST /users/:id/reset-password.
type ResetPasswordRequest struct {
	NewPassword string `json:"newPassword" validate:"required,min=8,max=128"`
}

// UpdateProfileRequest is the body of PUT /users/me.
type UpdateProfileRequest struct {
	FullName  *string `json:"fullName" validate:"omitempty,min=1,max=255"`
	Email     *string `json:"email"    validate:"omitempty,email,max=255"`
	Phone     *string `json:"phone"    validate:"omitempty,max=32"`
	AvatarURL *string `json:"avatarUrl" validate:"omitempty,url,max=2048"`
	Password  *string `json:"password" validate:"omitempty,min=8,max=128"`
	// CurrentPassword bắt buộc khi Password đặt — UC verify để đảm bảo chủ tài khoản.
	CurrentPassword *string `json:"currentPassword" validate:"omitempty,min=1,max=128"`
}

// ListUsersQuery is the query for GET /users.
type ListUsersQuery struct {
	Page   int    `query:"page" default:"1" validate:"omitempty,min=1"`
	Limit  int    `query:"limit" default:"20" validate:"omitempty,min=1,max=100"`
	Search string `query:"search"`
	Role   string `query:"role" validate:"omitempty,oneof=admin manager staff"`
	Status string `query:"status" validate:"omitempty,oneof=active locked"`
	// Multi filters — CSV ("admin,staff" / "active,locked"). Ưu tiên hơn Role/Status nếu có.
	// KHÔNG validate oneof ở đây vì là CSV — handler sẽ filter invalid value khi parse.
	Roles    string `query:"roles" validate:"omitempty,max=128"`
	Statuses string `query:"statuses" validate:"omitempty,max=64"`
}

// IDParam is the :id path param shared by single-resource routes.
type IDParam struct {
	ID int64 `param:"id" validate:"required,min=1"`
}

// NextCodeQuery is the query for GET /users/next-code.
type NextCodeQuery struct {
	Role string `query:"role" default:"staff" validate:"omitempty,oneof=admin manager staff"`
}

// InviteUserRequest is the body of POST /users/invite. Username + password are
// derived/auto-generated by the BE — admin chỉ cần email + họ tên + vai trò.
// Role bị giới hạn {manager, staff} (KHÔNG cho invite admin để tránh
// privilege escalation; muốn nâng cấp lên admin phải dùng Edit user).
type InviteUserRequest struct {
	Email     string  `json:"email"     validate:"required,email,max=200"`
	FullName  string  `json:"fullName"  validate:"required,min=1,max=255"`
	Role      string  `json:"role"      validate:"required,oneof=manager staff"`
	StaffCode *string `json:"staffCode" validate:"omitempty,max=32"`
	Phone     *string `json:"phone"     validate:"omitempty,max=32"`
}

// InviteUserResponse là payload trả về cho POST /users/invite. EmailSent=false
// khi SMTP/template fail — BE đính kèm TemporaryPassword plain để admin có thể
// copy/handoff thủ công (chỉ trường hợp này mới expose password ra response).
type InviteUserResponse struct {
	User              UserResponse `json:"user"`
	EmailSent         bool         `json:"emailSent"`
	TemporaryPassword string       `json:"temporaryPassword,omitempty"`
}

// BulkUserIDsRequest is the body of /users/bulk-* endpoints (lock/unlock/delete).
type BulkUserIDsRequest struct {
	UserIDs []int64 `json:"userIds" validate:"required,min=1,max=200,dive,min=1"`
}

// BulkUserFailure documents 1 user that the bulk action could not touch.
type BulkUserFailure struct {
	UserID int64  `json:"userId"`
	Reason string `json:"reason"`
}

// BulkUserActionResponse is returned by /users/bulk-*. OK = số user đổi
// thành công; Failed liệt kê các trường hợp bị skip kèm lý do (self-target,
// admin role, not-found, …).
type BulkUserActionResponse struct {
	OK     int               `json:"ok"`
	Failed []BulkUserFailure `json:"failed"`
}
