package usecase

import (
	"context"
	"io"

	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
)

// UserUseCase covers admin user management + self profile.
type UserUseCase interface {
	Create(ctx context.Context, in CreateUserInput) (*model.User, error)
	// Invite tạo user mới với username được derive từ email + password được
	// BE sinh ngẫu nhiên, set force_password_change=true, và gửi credentials
	// qua email cho user. Trả về (user, emailSent, tempPassword).
	// tempPassword chỉ non-empty khi emailSent=false (admin handoff thủ công).
	Invite(ctx context.Context, in InviteUserInput) (*model.User, bool, string, error)
	Get(ctx context.Context, id int64) (*model.User, error)
	List(ctx context.Context, f repository.UserListFilter) ([]model.User, int64, error)
	Update(ctx context.Context, id int64, in UpdateUserInput) (*model.User, error)
	ResetPassword(ctx context.Context, id int64, newPassword string) error
	// ToggleStatus flips active⇄locked. Caller must not toggle themselves (enforced in usecase).
	ToggleStatus(ctx context.Context, id int64) (*model.User, error)
	// BulkLock / BulkUnlock / BulkDelete thao tác hàng loạt; trả về (ok, failures, err).
	// Skip: self-target + admin role + đã ở trạng thái đích / đã deleted.
	BulkLock(ctx context.Context, userIDs []int64) (int, []BulkUserFailure, error)
	BulkUnlock(ctx context.Context, userIDs []int64) (int, []BulkUserFailure, error)
	BulkDelete(ctx context.Context, userIDs []int64) (int, []BulkUserFailure, error)
	UpdateProfile(ctx context.Context, in UpdateProfileInput) (*model.User, error)
	// NextStaffCode returns the next auto-generated staff code for the role (admin|manager|staff).
	NextStaffCode(ctx context.Context, role string) (string, error)
	// UploadAvatar uploads a new avatar image for the user and updates avatar_url.
	// Caller must be the user themselves or an admin.
	UploadAvatar(ctx context.Context, userID int64, filename string, size int64, body io.Reader) (*model.User, error)
}

// BulkUserFailure documents 1 user skipped in a bulk operation.
type BulkUserFailure struct {
	UserID int64
	Reason string
}

// InviteUserInput is the input to Invite (admin-only flow).
type InviteUserInput struct {
	Email     string
	FullName  string
	Role      string // "manager" | "staff" — admin role bị từ chối ở DTO layer
	StaffCode *string
	Phone     *string
}

// CreateUserInput is the input to Create.
type CreateUserInput struct {
	Username  string
	Password  string
	FullName  string
	Email     *string
	Phone     *string
	StaffCode *string
	Role      string
	// EmploymentType — "fulltime" | "parttime" (rỗng → mặc định fulltime ở repo).
	EmploymentType string
	// True khi muốn ép user đổi mật khẩu lần đầu sau khi admin tạo.
	ForcePasswordChange bool
}

// UpdateUserInput is the input to Update (nil = leave unchanged).
type UpdateUserInput struct {
	FullName       *string
	Email          *string
	Phone          *string
	Role           *string
	StaffCode      *string
	EmploymentType *string
}

// UpdateProfileInput is the input to UpdateProfile (nil = leave unchanged).
type UpdateProfileInput struct {
	FullName        *string
	Email           *string
	Phone           *string
	AvatarURL       *string
	Password        *string // optional new password
	CurrentPassword *string // required khi Password != nil (xác nhận chủ tài khoản)
}
