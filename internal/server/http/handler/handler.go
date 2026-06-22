// Package handler holds HTTP handlers and route registration.
package handler

import (
	"time"

	"github.com/labstack/echo/v4"
	"golang.org/x/time/rate"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/server/http/middleware"
	"vtv.vn/backend/pkg/xlogger"
)

// loginLimiter throttles /auth/login per client IP: ~5 attempts per minute with
// a small burst, to slow online credential-stuffing without locking real users.
var loginLimiter = middleware.NewIPRateLimiter(rate.Every(12*time.Second), 5)

// Handler wires every resource handler and registers routes on the Echo instance.
type Handler interface {
	RegisterRoutes(e *echo.Echo)
}

type handler struct {
	logger     *xlogger.Logger
	authMW     *middleware.Auth
	internalMW *middleware.Internal

	health       *HealthHandler
	auth         *AuthHandler
	user         *UserHandler
	permission   *PermissionHandler
	notification *NotificationHandler
	setting      *SettingHandler
	storage      *StorageHandler // nil when storage provider not configured
	sse          *SSEHandler
	activityLog  *ActivityLogHandler
}

// New builds the root Handler. Add resource-handler params as modules land.
func New(
	logger *xlogger.Logger,
	authMW *middleware.Auth,
	internalMW *middleware.Internal,
	health *HealthHandler,
	auth *AuthHandler,
	user *UserHandler,
	permission *PermissionHandler,
	notification *NotificationHandler,
	setting *SettingHandler,
	storage *StorageHandler,
	sse *SSEHandler,
	activityLog *ActivityLogHandler,
) Handler {
	return &handler{
		logger: logger, authMW: authMW, internalMW: internalMW,
		health:       health,
		auth:         auth,
		user:         user,
		permission:   permission,
		notification: notification,
		setting:      setting,
		storage:      storage,
		sse:          sse,
		activityLog:  activityLog,
	}
}

func (h *handler) RegisterRoutes(e *echo.Echo) {
	// ── public ───────────────────────────────────────────────────────────────
	e.GET("/healthz", h.health.Healthz)
	e.GET("/readyz", h.health.Readyz)
	e.GET("/version", h.health.Version)

	api := e.Group("/api/v1")
	h.registerAuthRoutes(api)
	h.registerUserRoutes(api)
	h.registerNotificationRoutes(api)
	h.registerSettingRoutes(api)
	h.registerPermissionRoutes(api)
	h.registerStorageRoutes(api)
	h.registerActivityLogRoutes(api)

	// ── internal (service-to-service) ────────────────────────────────────────
	internal := e.Group("/internal/api/v1", h.internalMW.Middleware())
	_ = internal // h.registerInternalRoutes(internal)
}

func (h *handler) registerAuthRoutes(api *echo.Group) {
	g := api.Group("/auth")
	// Brute-force protection: per-IP rate limit on login only. ~5 attempts/min/IP.
	g.POST("/login", h.auth.Login, loginLimiter.Middleware())
	// Password reset flow — public, rate-limited to prevent token-bruteforce.
	g.POST("/forgot-password", h.auth.ForgotPassword, loginLimiter.Middleware())
	g.POST("/reset-password", h.auth.ResetPassword, loginLimiter.Middleware())
	authd := g.Group("", h.authMW.Middleware())
	authd.POST("/logout", h.auth.Logout)
	authd.GET("/me", h.auth.Me)
	authd.POST("/change-password", h.auth.ChangePassword)
}

func (h *handler) registerNotificationRoutes(api *echo.Group) {
	// Announcements
	annG := api.Group("/announcements", h.authMW.Middleware())
	annG.GET("/unread-count", h.notification.AnnouncementUnreadCount) // before /:id
	annG.GET("", h.notification.ListAnnouncements)
	annG.GET("/:id", h.notification.GetAnnouncement)
	annG.GET("/:id/reads", h.notification.AnnouncementReads)
	annG.POST("/:id/read", h.notification.MarkRead) // reuse notification mark-read
	annG.POST("/:id/reply", h.notification.AnnouncementReply)
	adminMgr := annG.Group("", h.authMW.RequireRole(consts.RoleAdmin, consts.RoleManager))
	adminMgr.POST("", h.notification.CreateAnnouncement)
	adminMgr.PUT("/:id", h.notification.UpdateAnnouncement)
	adminMgr.DELETE("/:id", h.notification.DeleteAnnouncement)
	adminMgr.POST("/:id/publish", h.notification.PublishToAll)
	// Per-user notifications
	notifG := api.Group("/notifications", h.authMW.Middleware())
	notifG.GET("", h.notification.ListMyNotifications)
	notifG.GET("/unread-count", h.notification.UnreadCount)
	notifG.GET("/stream", h.sse.Stream)
	notifG.GET("/:id", h.notification.GetMyNotification)
	notifG.POST("/:id/read", h.notification.MarkRead)
	notifG.POST("/:id/unread", h.notification.MarkUnread)
	notifG.POST("/:id/important", h.notification.SetImportant)
	notifG.POST("/read-all", h.notification.MarkAllRead)
}

func (h *handler) registerSettingRoutes(api *echo.Group) {
	g := api.Group("/settings", h.authMW.Middleware(), h.authMW.RequireRole(consts.RoleAdmin))
	g.GET("", h.setting.List)
	g.GET("/:key", h.setting.Get)
	g.PUT("/:key", h.setting.Set)
}

// registerStorageRoutes wires /storage/uploads/request-url (auth required to
// avoid anonymous abuse) + /storage/* (public — FE renders <img> from these
// without sending Authorization header, and the redirected presigned URL
// is the actual capability). Skips registration when storage provider is nil.
func (h *handler) registerStorageRoutes(api *echo.Group) {
	if h.storage == nil {
		return
	}
	g := api.Group("/storage")
	// Auth-only: request a signed upload URL. FE PUTs file bytes to that URL.
	g.POST("/uploads/request-url", h.storage.RequestUploadURL, h.authMW.Middleware())
	// HMAC-token-protected (not JWT) — token in querystring is the capability,
	// scoped to one objectPath + content-type, valid 15m.
	g.PUT("/uploads/direct", h.storage.DirectUpload)
	// Public read — random suffix in key prevents enumeration.
	g.GET("/*", h.storage.Serve)
}

func (h *handler) registerActivityLogRoutes(api *echo.Group) {
	g := api.Group("", h.authMW.Middleware())
	g.GET("/activity-logs", h.activityLog.List)
	g.GET("/activity-logs/users", h.activityLog.ListUsers)
}

func (h *handler) registerPermissionRoutes(api *echo.Group) {
	// Permission tree (everyone authenticated can read the definitions to render
	// the picker UI; effective permissions of self are always readable).
	defs := api.Group("/permissions", h.authMW.Middleware())
	defs.GET("/definitions", h.permission.Definitions)
	defs.GET("/actions", h.permission.Actions)
	defs.GET("/me", h.permission.Me)

	// CRUD on the user_permissions table — only admins or users with
	// "users.permissions" write access can mutate.
	g := api.Group("/user-permissions", h.authMW.Middleware())
	g.GET("", h.permission.List,
		h.authMW.RequirePermission(consts.ResourceUserPermissions, consts.ActionRead))
	g.POST("", h.permission.Create,
		h.authMW.RequirePermission(consts.ResourceUserPermissions, consts.ActionWrite))
	g.GET("/:id", h.permission.Get,
		h.authMW.RequirePermission(consts.ResourceUserPermissions, consts.ActionRead))
	g.PUT("/:id", h.permission.Update,
		h.authMW.RequirePermission(consts.ResourceUserPermissions, consts.ActionWrite))
	g.DELETE("/:id", h.permission.Delete,
		h.authMW.RequirePermission(consts.ResourceUserPermissions, consts.ActionDelete))

	// FE-compat shortcut routes under /users/:id/permissions — read can be
	// done by users.read, write requires users.permissions.write.
	ug := api.Group("/users", h.authMW.Middleware())
	ug.GET("/:id/permissions", h.permission.GetByUser,
		h.authMW.RequirePermission(consts.ResourceUserPermissions, consts.ActionRead))
	ug.PUT("/:id/permissions", h.permission.UpdateByUser,
		h.authMW.RequirePermission(consts.ResourceUserPermissions, consts.ActionWrite))
}

func (h *handler) registerUserRoutes(api *echo.Group) {
	g := api.Group("/users", h.authMW.Middleware())
	// self
	g.GET("/me", h.user.GetMe)
	g.PUT("/me", h.user.UpdateProfile)
	g.POST("/me/avatar", h.user.UploadAvatarMe)
	// Read-only user list/detail — admin + manager.
	adminMgr := g.Group("", h.authMW.RequireRole(consts.RoleAdmin, consts.RoleManager))
	adminMgr.GET("", h.user.List)
	adminMgr.GET("/:id", h.user.Get)
	// Write operations — gate bằng quyền users.write.
	uw := g.Group("", h.authMW.RequirePermission(consts.ResourceUsers, consts.ActionWrite))
	uw.GET("/next-code", h.user.NextCode)
	uw.POST("", h.user.Create)
	uw.PUT("/:id", h.user.Update)
	uw.POST("/:id/reset-password", h.user.ResetPassword)
	uw.POST("/:id/toggle-status", h.user.ToggleStatus)
	uw.POST("/:id/avatar", h.user.UploadAvatarByID)
	// Invite + bulk actions — admin/manager.
	adminMgr.POST("/invite", h.user.Invite)
	adminMgr.POST("/bulk-lock", h.user.BulkLock)
	adminMgr.POST("/bulk-unlock", h.user.BulkUnlock)
	adminMgr.DELETE("/bulk", h.user.BulkDelete)
}
