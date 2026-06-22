package handler

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"

	"github.com/labstack/echo/v4"

	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/internal/server/http/dto"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// validRoles / validStatuses: gate cho CSV multi-filter (Roles/Statuses).
// Giá trị invalid bị bỏ qua, không trả 400 — UX nhẹ nhàng hơn với param mở rộng.
var validRoles = map[string]bool{"admin": true, "manager": true, "staff": true}
var validStatuses = map[string]bool{"active": true, "locked": true}

func splitCSVWhitelist(csv string, allow map[string]bool) []string {
	if csv == "" {
		return nil
	}
	parts := strings.Split(csv, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" && allow[v] {
			out = append(out, v)
		}
	}
	return out
}

// UserHandler serves /users/*.
type UserHandler struct {
	logger *xlogger.Logger
	userUC usecase.UserUseCase
}

// NewUserHandler builds the user handler.
func NewUserHandler(logger *xlogger.Logger, userUC usecase.UserUseCase) *UserHandler {
	return &UserHandler{logger: logger, userUC: userUC}
}

// GetMe godoc
// @Summary		Hồ sơ của tôi
// @Tags		users
// @Produce		json
// @Success		200	{object}	xhttp.Envelope{data=dto.UserResponse}
// @Security	BearerAuth
// @Router		/users/me [get]
func (h *UserHandler) GetMe(c echo.Context) error {
	u, err := h.userUC.Get(c.Request().Context(), xhttp.UserID(c.Request().Context()))
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserResponse(u))
}

// UpdateProfile godoc
// @Summary		Cập nhật hồ sơ của tôi
// @Tags		users
// @Accept		json
// @Produce		json
// @Param		request	body		dto.UpdateProfileRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope{data=dto.UserResponse}
// @Security	BearerAuth
// @Router		/users/me [put]
func (h *UserHandler) UpdateProfile(c echo.Context) error {
	req := &dto.UpdateProfileRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	u, err := h.userUC.UpdateProfile(c.Request().Context(), usecase.UpdateProfileInput{
		FullName: req.FullName, Email: req.Email, Phone: req.Phone, AvatarURL: req.AvatarURL,
		Password: req.Password, CurrentPassword: req.CurrentPassword,
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserResponse(u))
}

// NextCode godoc
// @Summary		Sinh staff code tiếp theo
// @Tags		users
// @Produce		json
// @Param		role	query		string	false	"admin|manager|staff"
// @Success		200		{object}	xhttp.Envelope
// @Security	BearerAuth
// @Router		/users/next-code [get]
func (h *UserHandler) NextCode(c echo.Context) error {
	q := &dto.NextCodeQuery{}
	if errs := xhttp.ReadAndValidateRequest(c, q); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	code, err := h.userUC.NextStaffCode(c.Request().Context(), q.Role)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, map[string]string{"nextCode": code})
}

// List godoc
// @Summary		Danh sách người dùng
// @Tags		users
// @Produce		json
// @Param		page	query		int		false	"page (default 1)"
// @Param		limit	query		int		false	"limit (default 20, max 100)"
// @Param		search	query		string	false	"tìm theo username/họ tên/email"
// @Param		role	query		string	false	"admin|manager|staff"
// @Param		status	query		string	false	"active|locked"
// @Success		200		{object}	xhttp.PagedEnvelope{data=[]dto.UserResponse}
// @Security	BearerAuth
// @Router		/users [get]
func (h *UserHandler) List(c echo.Context) error {
	q := &dto.ListUsersQuery{}
	if errs := xhttp.ReadAndValidateRequest(c, q); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	users, total, err := h.userUC.List(c.Request().Context(), repository.UserListFilter{
		Search:   q.Search,
		Role:     q.Role,
		Status:   q.Status,
		Roles:    splitCSVWhitelist(q.Roles, validRoles),
		Statuses: splitCSVWhitelist(q.Statuses, validStatuses),
		Page:     model.NewPage(q.Page, q.Limit),
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.PaginatedResponse(c, dto.ToUserResponses(users), q.Page, q.Limit, total)
}

// Create godoc
// @Summary		Tạo người dùng
// @Tags		users
// @Accept		json
// @Produce		json
// @Param		request	body		dto.CreateUserRequest	true	"payload"
// @Success		201		{object}	xhttp.Envelope{data=dto.UserResponse}
// @Failure		409		{object}	xhttp.Problem
// @Security	BearerAuth
// @Router		/users [post]
func (h *UserHandler) Create(c echo.Context) error {
	req := &dto.CreateUserRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	u, err := h.userUC.Create(c.Request().Context(), usecase.CreateUserInput{
		Username: req.Username, Password: req.Password, FullName: req.FullName,
		Email: req.Email, Phone: req.Phone, StaffCode: req.StaffCode, Role: req.Role,
		EmploymentType:      req.EmploymentType,
		ForcePasswordChange: req.ForcePasswordChange,
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.CreatedResponse(c, dto.ToUserResponse(u))
}

// Get godoc
// @Summary		Chi tiết người dùng
// @Tags		users
// @Produce		json
// @Param		id	path		int	true	"user id"
// @Success		200	{object}	xhttp.Envelope{data=dto.UserResponse}
// @Failure		404	{object}	xhttp.Problem
// @Security	BearerAuth
// @Router		/users/{id} [get]
func (h *UserHandler) Get(c echo.Context) error {
	p := &dto.IDParam{}
	if errs := xhttp.ReadAndValidateRequest(c, p); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	u, err := h.userUC.Get(c.Request().Context(), p.ID)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserResponse(u))
}

// Update godoc
// @Summary		Cập nhật người dùng
// @Tags		users
// @Accept		json
// @Produce		json
// @Param		id		path		int						true	"user id"
// @Param		request	body		dto.UpdateUserRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope{data=dto.UserResponse}
// @Security	BearerAuth
// @Router		/users/{id} [put]
func (h *UserHandler) Update(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	req := &dto.UpdateUserRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	u, err := h.userUC.Update(c.Request().Context(), id, usecase.UpdateUserInput{
		FullName: req.FullName, Email: req.Email, Phone: req.Phone, Role: req.Role, StaffCode: req.StaffCode,
		EmploymentType: req.EmploymentType,
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserResponse(u))
}

// ResetPassword godoc
// @Summary		Đặt lại mật khẩu người dùng
// @Tags		users
// @Accept		json
// @Produce		json
// @Param		id		path		int							true	"user id"
// @Param		request	body		dto.ResetPasswordRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope
// @Security	BearerAuth
// @Router		/users/{id}/reset-password [post]
func (h *UserHandler) ResetPassword(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	req := &dto.ResetPasswordRequest{}
	_ = c.Bind(req) // ignore bind error — password may be empty
	newPw := req.NewPassword
	if newPw == "" {
		// Auto-generate random password
		b := make([]byte, 8)
		_, _ = rand.Read(b)
		newPw = hex.EncodeToString(b)[:12]
	}
	if err := h.userUC.ResetPassword(c.Request().Context(), id, newPw); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, map[string]string{
		"message":           "đặt lại mật khẩu thành công",
		"temporaryPassword": newPw,
	})
}

// UploadAvatarMe godoc
// @Summary		Upload avatar của tôi
// @Tags		users
// @Accept		multipart/form-data
// @Produce		json
// @Param		avatar  formData  file  true  "file ảnh (JPEG/PNG/WebP, tối đa 5MB)"
// @Success		200  {object}  xhttp.Envelope{data=dto.UserResponse}
// @Failure		400  {object}  xhttp.Problem
// @Failure		401  {object}  xhttp.Problem
// @Failure		503  {object}  xhttp.Problem
// @Security	BearerAuth
// @Router		/users/me/avatar [post]
func (h *UserHandler) UploadAvatarMe(c echo.Context) error {
	callerID := xhttp.UserID(c.Request().Context())
	fh, err := c.FormFile("avatar")
	if err != nil {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("thiếu file avatar"))
	}
	f, err := fh.Open()
	if err != nil {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("không thể mở file avatar"))
	}
	defer f.Close()

	u, err := h.userUC.UploadAvatar(c.Request().Context(), callerID, fh.Filename, fh.Size, f)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserResponse(u))
}

// UploadAvatarByID godoc
// @Summary		Upload avatar cho người dùng bất kỳ (admin only)
// @Tags		users
// @Accept		multipart/form-data
// @Produce		json
// @Param		id      path      int   true  "User ID"
// @Param		avatar  formData  file  true  "file ảnh (JPEG/PNG/WebP, tối đa 5MB)"
// @Success		200  {object}  xhttp.Envelope{data=dto.UserResponse}
// @Failure		400  {object}  xhttp.Problem
// @Failure		401  {object}  xhttp.Problem
// @Failure		403  {object}  xhttp.Problem
// @Failure		404  {object}  xhttp.Problem
// @Failure		503  {object}  xhttp.Problem
// @Security	BearerAuth
// @Router		/users/{id}/avatar [post]
func (h *UserHandler) UploadAvatarByID(c echo.Context) error {
	p := &dto.IDParam{}
	if errs := xhttp.ReadAndValidateRequest(c, p); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	fh, err := c.FormFile("avatar")
	if err != nil {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("thiếu file avatar"))
	}
	f, err := fh.Open()
	if err != nil {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("không thể mở file avatar"))
	}
	defer f.Close()

	u, err := h.userUC.UploadAvatar(c.Request().Context(), p.ID, fh.Filename, fh.Size, f)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserResponse(u))
}

// Invite godoc
// @Summary		Mời người dùng qua email
// @Description	Sinh username từ email + password ngẫu nhiên, set force_password_change=true,
// @Description	gửi credentials qua email cho user. Admin/manager only.
// @Tags		users
// @Accept		json
// @Produce		json
// @Param		request	body		dto.InviteUserRequest	true	"payload"
// @Success		201		{object}	xhttp.Envelope{data=dto.InviteUserResponse}
// @Failure		400		{object}	xhttp.Problem
// @Security	BearerAuth
// @Router		/users/invite [post]
func (h *UserHandler) Invite(c echo.Context) error {
	req := &dto.InviteUserRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	user, emailSent, tempPw, err := h.userUC.Invite(c.Request().Context(), usecase.InviteUserInput{
		Email: req.Email, FullName: req.FullName, Role: req.Role,
		StaffCode: req.StaffCode, Phone: req.Phone,
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	resp := dto.InviteUserResponse{
		User:      dto.ToUserResponse(user),
		EmailSent: emailSent,
	}
	if !emailSent {
		// Chỉ expose plain password khi email gửi fail — admin cần handoff
		// thủ công. Email success → password chỉ tồn tại trong hộp thư user.
		resp.TemporaryPassword = tempPw
	}
	return xhttp.CreatedResponse(c, resp)
}

// bulkOp signature shared by the 3 bulk endpoints below.
type bulkUserOp func(ctx context.Context, ids []int64) (int, []usecase.BulkUserFailure, error)

func (h *UserHandler) runBulk(c echo.Context, op bulkUserOp) error {
	req := &dto.BulkUserIDsRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	ok, failed, err := op(c.Request().Context(), req.UserIDs)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	out := dto.BulkUserActionResponse{OK: ok, Failed: make([]dto.BulkUserFailure, 0, len(failed))}
	for _, f := range failed {
		out.Failed = append(out.Failed, dto.BulkUserFailure{UserID: f.UserID, Reason: f.Reason})
	}
	return xhttp.SuccessResponse(c, out)
}

// BulkLock godoc
// @Summary		Khoá nhiều người dùng cùng lúc
// @Tags		users
// @Accept		json
// @Produce		json
// @Param		request	body		dto.BulkUserIDsRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope{data=dto.BulkUserActionResponse}
// @Security	BearerAuth
// @Router		/users/bulk-lock [post]
func (h *UserHandler) BulkLock(c echo.Context) error { return h.runBulk(c, h.userUC.BulkLock) }

// BulkUnlock godoc
// @Summary		Mở khoá nhiều người dùng cùng lúc
// @Tags		users
// @Accept		json
// @Produce		json
// @Param		request	body		dto.BulkUserIDsRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope{data=dto.BulkUserActionResponse}
// @Security	BearerAuth
// @Router		/users/bulk-unlock [post]
func (h *UserHandler) BulkUnlock(c echo.Context) error { return h.runBulk(c, h.userUC.BulkUnlock) }

// BulkDelete godoc
// @Summary		Xoá nhiều người dùng cùng lúc (soft delete)
// @Tags		users
// @Accept		json
// @Produce		json
// @Param		request	body		dto.BulkUserIDsRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope{data=dto.BulkUserActionResponse}
// @Security	BearerAuth
// @Router		/users/bulk [delete]
func (h *UserHandler) BulkDelete(c echo.Context) error { return h.runBulk(c, h.userUC.BulkDelete) }

// ToggleStatus godoc
// @Summary		Khoá/mở khoá người dùng
// @Tags		users
// @Produce		json
// @Param		id	path		int	true	"user id"
// @Success		200	{object}	xhttp.Envelope{data=dto.UserResponse}
// @Failure		400	{object}	xhttp.Problem
// @Security	BearerAuth
// @Router		/users/{id}/toggle-status [post]
func (h *UserHandler) ToggleStatus(c echo.Context) error {
	p := &dto.IDParam{}
	if errs := xhttp.ReadAndValidateRequest(c, p); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	u, err := h.userUC.ToggleStatus(c.Request().Context(), p.ID)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserResponse(u))
}
