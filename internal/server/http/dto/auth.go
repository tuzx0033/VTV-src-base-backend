package dto

// LoginRequest is the body of POST /auth/login. Max bounds prevent an attacker
// from streaming megabytes of credentials per attempt.
type LoginRequest struct {
	Username string `json:"username" validate:"required,min=1,max=100"`
	Password string `json:"password" validate:"required,min=1,max=128"`
}

// LoginResponse is returned by POST /auth/login. ForcePasswordChange = true
// khi user vẫn còn mật khẩu khởi tạo (invite flow) — FE phải redirect sang
// /change-password-required và block các API khác cho tới khi đổi xong.
type LoginResponse struct {
	Token               string       `json:"token"`
	User                UserResponse `json:"user"`
	ForcePasswordChange bool         `json:"forcePasswordChange"`
}

// ChangePasswordRequest is the body of POST /auth/change-password. NewPassword
// min=8 aligns with OWASP ASVS L1 (paired with bcrypt cost=12).
type ChangePasswordRequest struct {
	CurrentPassword string `json:"currentPassword" validate:"required,min=1,max=128"`
	NewPassword     string `json:"newPassword"     validate:"required,min=8,max=128"`
}

// ForgotPasswordRequest is the body of POST /auth/forgot-password.
type ForgotPasswordRequest struct {
	Username string `json:"username" validate:"required,min=1,max=100"`
}

// ResetPasswordByTokenRequest is the body of POST /auth/reset-password.
type ResetPasswordByTokenRequest struct {
	Token       string `json:"token"       validate:"required,min=8,max=128"`
	NewPassword string `json:"newPassword" validate:"required,min=8,max=128"`
}
