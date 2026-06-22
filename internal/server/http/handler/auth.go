package handler

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/internal/server/http/dto"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// CookieConfig is the minimal cookie params the auth handler needs to set / clear the session cookie.
type CookieConfig struct {
	Name      string
	Domain    string
	Secure    bool
	MaxAgeSec int // TTL in seconds; derived from jwt_ttl
}

// AuthHandler serves /auth/*.
type AuthHandler struct {
	logger *xlogger.Logger
	authUC usecase.AuthUseCase
	cookie CookieConfig
}

// NewAuthHandler builds the auth handler.
func NewAuthHandler(logger *xlogger.Logger, authUC usecase.AuthUseCase, cookie CookieConfig) *AuthHandler {
	return &AuthHandler{logger: logger, authUC: authUC, cookie: cookie}
}

// Login godoc
// @Summary		Đăng nhập
// @Tags		auth
// @Accept		json
// @Produce		json
// @Param		request	body		dto.LoginRequest	true	"credentials"
// @Success		200		{object}	xhttp.Envelope{data=dto.LoginResponse}
// @Failure		401		{object}	xhttp.Problem
// @Failure		422		{object}	xhttp.Problem
// @Router		/auth/login [post]
func (h *AuthHandler) Login(c echo.Context) error {
	req := &dto.LoginRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	token, user, err := h.authUC.Login(c.Request().Context(), req.Username, req.Password)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	if h.cookie.Name != "" {
		c.SetCookie(&http.Cookie{
			Name:     h.cookie.Name,
			Value:    token,
			Path:     "/",
			Domain:   h.cookie.Domain,
			HttpOnly: true,
			Secure:   h.cookie.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   h.cookie.MaxAgeSec,
		})
	}
	return xhttp.SuccessResponse(c, dto.LoginResponse{
		Token:               token,
		User:                dto.ToUserResponse(user),
		ForcePasswordChange: user.ForcePasswordChange,
	})
}

// Me godoc
// @Summary		Thông tin tài khoản hiện tại
// @Tags		auth
// @Produce		json
// @Success		200	{object}	xhttp.Envelope{data=dto.UserResponse}
// @Failure		401	{object}	xhttp.Problem
// @Security	BearerAuth
// @Router		/auth/me [get]
func (h *AuthHandler) Me(c echo.Context) error {
	user, err := h.authUC.Me(c.Request().Context())
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserResponse(user))
}

// ChangePassword godoc
// @Summary		Đổi mật khẩu
// @Tags		auth
// @Accept		json
// @Produce		json
// @Param		request	body		dto.ChangePasswordRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope
// @Failure		401		{object}	xhttp.Problem
// @Security	BearerAuth
// @Router		/auth/change-password [post]
func (h *AuthHandler) ChangePassword(c echo.Context) error {
	req := &dto.ChangePasswordRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	if err := h.authUC.ChangePassword(c.Request().Context(), req.CurrentPassword, req.NewPassword); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, map[string]string{"message": "đổi mật khẩu thành công"})
}

// ForgotPassword godoc
// @Summary		Yêu cầu reset mật khẩu (gửi token đến channel nội bộ)
// @Tags		auth
// @Accept		json
// @Produce		json
// @Param		request	body		dto.ForgotPasswordRequest	true	"username"
// @Success		200		{object}	xhttp.Envelope
// @Router		/auth/forgot-password [post]
func (h *AuthHandler) ForgotPassword(c echo.Context) error {
	req := &dto.ForgotPasswordRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	_, err := h.authUC.RequestPasswordReset(c.Request().Context(), req.Username)
	if err != nil {
		// Log but don't leak — anti-enumeration.
		h.logger.Error("forgot password", xlogger.Err(err))
	}
	// Always return 200 với cùng message (anti-enumeration).
	return xhttp.SuccessResponse(c, map[string]string{
		"message": "Nếu tài khoản tồn tại, link đặt lại mật khẩu đã được gửi tới email đăng ký. Hãy kiểm tra hộp thư (kể cả thư rác).",
	})
}

// ResetPassword godoc
// @Summary		Đặt lại mật khẩu bằng token
// @Tags		auth
// @Accept		json
// @Produce		json
// @Param		request	body		dto.ResetPasswordByTokenRequest	true	"token + newPassword"
// @Success		200		{object}	xhttp.Envelope
// @Failure		400		{object}	xhttp.Problem
// @Router		/auth/reset-password [post]
func (h *AuthHandler) ResetPassword(c echo.Context) error {
	req := &dto.ResetPasswordByTokenRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	if err := h.authUC.ResetPassword(c.Request().Context(), req.Token, req.NewPassword); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, map[string]string{"message": "đặt lại mật khẩu thành công"})
}

// Logout godoc
// @Summary		Đăng xuất
// @Tags		auth
// @Produce		json
// @Success		200	{object}	xhttp.Envelope
// @Security	BearerAuth
// @Router		/auth/logout [post]
func (h *AuthHandler) Logout(c echo.Context) error {
	if err := h.authUC.Logout(c.Request().Context()); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	if h.cookie.Name != "" {
		// Clear the session cookie
		c.SetCookie(&http.Cookie{
			Name:     h.cookie.Name,
			Value:    "",
			Path:     "/",
			Domain:   h.cookie.Domain,
			HttpOnly: true,
			Secure:   h.cookie.Secure,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   -1,
			Expires:  time.Unix(0, 0),
		})
	}
	return xhttp.SuccessResponse(c, map[string]string{"message": "đã đăng xuất"})
}
