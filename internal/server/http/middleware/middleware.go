// Package middleware holds HTTP middlewares (auth, role gate, internal API key).
package middleware

import (
	"crypto/subtle"
	"strings"

	"github.com/labstack/echo/v4"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/pkg/xauth"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// Auth verifies the JWT (from the configured cookie or `Authorization: Bearer`)
// and puts the user id + role into the request context.
type Auth struct {
	logger     *xlogger.Logger
	tokens     *xauth.Manager
	cookieName string
	blacklist  *xauth.Blacklist                // nil = skip revocation check
	perms      repository.PermissionRepository // nil = RequirePermission denies non-admin
	users      repository.UserRepository       // nil = RequirePasswordRotated() is a no-op
}

// NewAuth builds the auth middleware.
func NewAuth(logger *xlogger.Logger, tokens *xauth.Manager, cookieName string, blacklist *xauth.Blacklist) *Auth {
	return &Auth{logger: logger, tokens: tokens, cookieName: cookieName, blacklist: blacklist}
}

// forcePwExemptSuffix là các route mà user đang ở trạng thái
// force_password_change vẫn được phép gọi (để hoàn tất rotation).
// Match theo SUFFIX để cover cả prefix "/api/v1".
var forcePwExemptSuffix = []string{
	"/auth/me",
	"/auth/logout",
	"/auth/change-password",
}

func isForcePwExempt(path string) bool {
	for _, s := range forcePwExemptSuffix {
		if strings.HasSuffix(path, s) {
			return true
		}
	}
	return false
}

// Middleware authenticates the request. Khi user có cờ force_password_change
// + path không thuộc exempt list (auth/me, auth/logout, auth/change-password)
// → trả 403 FORCE_PASSWORD_CHANGE để FE redirect.
func (a *Auth) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			tok := a.extract(c)
			if tok == "" {
				a.logger.Warn("unauthenticated request", xlogger.String("path", c.Request().URL.Path))
				return xhttp.AppErrorResponse(c, xhttp.UnauthorizedErrorf("không có token"))
			}
			claims, err := a.tokens.Verify(tok)
			if err != nil {
				a.logger.Warn("invalid token", xlogger.String("path", c.Request().URL.Path))
				return xhttp.AppErrorResponse(c, xhttp.UnauthorizedErrorf("token không hợp lệ"))
			}
			if a.blacklist != nil && claims.ID != "" {
				revoked, bErr := a.blacklist.IsRevoked(c.Request().Context(), claims.ID)
				if bErr != nil {
					a.logger.Error("blacklist check failed", xlogger.Err(bErr))
					return xhttp.AppErrorResponse(c, xhttp.InternalErrorf("lỗi xác thực"))
				}
				if revoked {
					return xhttp.AppErrorResponse(c, xhttp.UnauthorizedErrorf("token đã bị thu hồi"))
				}
			}
			ctx := c.Request().Context()
			ctx = xhttp.WithUserID(ctx, claims.UserID)
			ctx = xhttp.WithRole(ctx, claims.Role)
			ctx = xhttp.WithTokenID(ctx, claims.ID)
			if claims.ExpiresAt != nil {
				ctx = xhttp.WithTokenExpiry(ctx, claims.ExpiresAt.Time)
			}
			c.SetRequest(c.Request().WithContext(ctx))

			// Force-password-change gate. Skip cho exempt routes (me, logout,
			// change-password). 1 query mỗi protected request — chấp nhận vì
			// đây là internal tool, traffic thấp; có thể cache vào Redis nếu
			// cần optimize sau.
			if a.users != nil && !isForcePwExempt(c.Request().URL.Path) {
				must, mErr := a.users.MustForcePasswordChange(ctx, claims.UserID)
				if mErr != nil {
					a.logger.Error("force-password-change check failed", xlogger.Err(mErr))
					return xhttp.AppErrorResponse(c, xhttp.InternalErrorf("lỗi kiểm tra trạng thái mật khẩu"))
				}
				if must {
					return xhttp.AppErrorResponse(c, xhttp.ForcePasswordChangeErrorf("vui lòng đổi mật khẩu trước khi tiếp tục"))
				}
			}
			return next(c)
		}
	}
}

// SetPermissionRepo attaches the permission repository so RequirePermission can
// query the JSONB grant tree. Optional — leaving it nil disables permission
// checks (RequirePermission then only honours role-based fallback).
func (a *Auth) SetPermissionRepo(repo repository.PermissionRepository) {
	a.perms = repo
}

// SetUserRepo wires the user repository so RequirePasswordRotated() can read
// the `force_password_change` flag. Without it, the middleware is a no-op.
func (a *Auth) SetUserRepo(repo repository.UserRepository) {
	a.users = repo
}

// RequirePermission gates a route on (resource, action). MUST be chained after
// Middleware(). Logic giống EffectivePermissions ở usecase:
//
//   - admin → bypass (god-mode)
//   - non-admin → role defaults (model.RoleDefaultPermissions) ⊕ user_permissions
//     overrides. true = grant, false = revoke.
//
// Keep this in sync với usecase/permission.go::EffectivePermissions — cùng dùng
// model.RoleDefaultPermissions nên consistent.
func (a *Auth) RequirePermission(resource, action string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			role := xhttp.Role(ctx)
			if role == consts.RoleAdmin {
				return next(c)
			}
			userID := xhttp.UserID(ctx)

			// Start with role defaults — manager/staff có grants ngay cả khi
			// user_permissions row chưa tồn tại. Tránh phải seed row cho mỗi
			// user mới khi tạo.
			effective := model.RoleDefaultPermissions(role)

			// Layer explicit overrides on top (admin có thể grant/revoke cho
			// từng user qua user_permissions). nil → giữ nguyên defaults.
			if a.perms != nil {
				up, err := a.perms.GetByUserID(ctx, userID)
				if err != nil {
					a.logger.Error("permission lookup failed", xlogger.Err(err),
						xlogger.Int64("user_id", userID))
					return xhttp.AppErrorResponse(c, xhttp.InternalErrorf("lỗi kiểm tra quyền"))
				}
				if up != nil {
					effective = effective.MergeOver(up.Permissions)
				}
			}

			if !effective.HasAction(resource, action) {
				return xhttp.AppErrorResponse(c, xhttp.ForbiddenErrorf("không đủ quyền"))
			}
			return next(c)
		}
	}
}

// RequireRole gates a route to one of the given roles. MUST be chained after Middleware().
func (a *Auth) RequireRole(roles ...string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			role := xhttp.Role(c.Request().Context())
			for _, r := range roles {
				if role == r {
					return next(c)
				}
			}
			return xhttp.AppErrorResponse(c, xhttp.ForbiddenErrorf("không đủ quyền"))
		}
	}
}

func (a *Auth) extract(c echo.Context) string {
	if a.cookieName != "" {
		if ck, err := c.Cookie(a.cookieName); err == nil && ck.Value != "" {
			return ck.Value
		}
	}
	h := c.Request().Header.Get(echo.HeaderAuthorization)
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimSpace(h[7:])
	}
	return ""
}

// Internal authenticates service-to-service calls via a static API key.
type Internal struct {
	apiKey string
	logger *xlogger.Logger
}

// NewInternal builds the internal-auth middleware.
func NewInternal(apiKey string, logger *xlogger.Logger) *Internal {
	return &Internal{apiKey: apiKey, logger: logger}
}

// Middleware enforces the X-Internal-Api-Key header. The comparison is constant
// time to mitigate timing oracles.
func (i *Internal) Middleware() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			got := c.Request().Header.Get("X-Internal-Api-Key")
			if i.apiKey == "" || subtle.ConstantTimeCompare([]byte(got), []byte(i.apiKey)) != 1 {
				i.logger.Warn("internal auth rejected", xlogger.String("path", c.Request().URL.Path))
				return xhttp.AppErrorResponse(c, xhttp.UnauthorizedErrorf("missing or invalid internal api key"))
			}
			return next(c)
		}
	}
}
