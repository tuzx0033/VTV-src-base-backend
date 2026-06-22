// Package di is the composition root — manual dependency injection (no codegen).
package di

import (
	"context"
	"fmt"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"vtv.vn/backend/internal/config"
	"vtv.vn/backend/internal/repository"
	"vtv.vn/backend/internal/server/http/handler"
	"vtv.vn/backend/internal/server/http/middleware"
	"vtv.vn/backend/internal/usecase"
	"vtv.vn/backend/pkg/xauth"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xmail"
	"vtv.vn/backend/pkg/xnotify"
	"vtv.vn/backend/pkg/xpostgres"
	"vtv.vn/backend/pkg/xstorage"
)

// BackgroundServer is anything with a Start/Stop lifecycle (cron, asynq worker).
type BackgroundServer interface {
	Start() error
	Stop()
}

// AppContainer holds everything cmd/api needs after wiring.
type AppContainer struct {
	HTTPHandler       handler.Handler
	BackgroundServers []BackgroundServer
	TxRunner          xpostgres.TxRunner
}

// NewAppContainer wires the application. Add repositories/services/usecases here
// as modules land — keep cmd/api/main.go free of wiring logic.
func NewAppContainer(
	cfg *config.Config,
	logger *xlogger.Logger,
	db *gorm.DB,
	rdb *redis.Client,
	_ *echo.Echo,
	version string,
) (*AppContainer, func(), error) {
	// ── infra ────────────────────────────────────────────────────────────────
	tx := xpostgres.NewTxRunner(db)
	tokens, err := xauth.NewManager(&cfg.Auth.Config)
	if err != nil {
		return nil, nil, err
	}
	blacklist := xauth.NewBlacklist(rdb)

	// ── storage (optional — only if provider is configured) ──────────────────
	var storage xstorage.Provider
	if cfg.Storage.Provider != "" {
		s, sErr := xstorage.New(context.Background(), &cfg.Storage)
		if sErr != nil {
			return nil, nil, fmt.Errorf("init storage: %w", sErr)
		}
		storage = s
	}
	// storage is wired into usecases that need it (nil when provider is unconfigured).

	// ── repositories ─────────────────────────────────────────────────────────
	userRepo := repository.NewUserRepository(logger, db)
	auditRepo := repository.NewAuditRepository(db)
	settingRepo := repository.NewSettingRepository(db)
	permissionRepo := repository.NewPermissionRepository(logger, db)

	// ── mailer (SMTP or logger fallback) ─────────────────────────────────────
	smtpCfg := xmail.Config{
		Host:     cfg.Integrations.SMTP.Host,
		Port:     cfg.Integrations.SMTP.Port,
		Username: cfg.Integrations.SMTP.Username,
		Password: cfg.Integrations.SMTP.Password,
		From:     cfg.Integrations.SMTP.From,
	}
	var mailer xmail.Sender
	if smtpCfg.IsConfigured() {
		mailer = xmail.NewSMTPSender(smtpCfg)
		logger.Info("mailer: SMTP configured",
			xlogger.String("host", smtpCfg.Host),
			xlogger.Int("port", smtpCfg.Port),
			xlogger.String("from", smtpCfg.From),
		)
	} else {
		mailer = &xmail.LoggerSender{Log: func(m xmail.Message) {
			logger.Warn("mailer: STDOUT fallback (SMTP not configured)",
				xlogger.String("to", strings.Join(m.To, ",")),
				xlogger.String("subject", m.Subject),
			)
		}}
		logger.Warn("mailer: SMTP not configured — using stdout fallback")
	}
	// FE base URL = first CORS-allowed origin. Fallback về BaseURL nếu chưa cấu hình CORS.
	feBase := cfg.HTTP.BaseURL
	for _, o := range cfg.HTTP.CORSAllowOrigins {
		if o != "" {
			feBase = o
			break
		}
	}
	resetURL := strings.TrimRight(feBase, "/") + "/reset-password"
	loginURL := strings.TrimRight(feBase, "/") + "/login"

	// ── notification (real-time push via Redis Pub/Sub hub) ───────────────────
	announcementRepo := repository.NewAnnouncementRepository(db)
	baseNotificationRepo := repository.NewNotificationRepository(db)
	notifHub := xnotify.NewRedisHub(rdb, logger)
	notificationRepo := repository.NewPublishingNotificationRepository(baseNotificationRepo, notifHub, logger)

	// ── usecases ─────────────────────────────────────────────────────────────
	authUC := usecase.NewAuthUseCase(logger, tx, userRepo, auditRepo, tokens, blacklist, mailer, resetURL)
	userUC := usecase.NewUserUseCase(logger, tx, userRepo, permissionRepo, auditRepo, storage, mailer, loginURL)
	settingUC := usecase.NewSettingUseCase(logger, settingRepo)
	notificationUC := usecase.NewNotificationUseCase(logger, tx, announcementRepo, notificationRepo, userRepo, auditRepo)
	permissionUC := usecase.NewPermissionUseCase(logger, tx, permissionRepo, userRepo, auditRepo)

	// ── middlewares ──────────────────────────────────────────────────────────
	authMW := middleware.NewAuth(logger, tokens, cfg.Auth.CookieName, blacklist)
	authMW.SetPermissionRepo(permissionRepo)
	authMW.SetUserRepo(userRepo)
	internalMW := middleware.NewInternal(cfg.InternalAPIKey, logger)

	// ── handlers ─────────────────────────────────────────────────────────────
	healthH := handler.NewHealthHandler(logger, db, rdb, version)
	cookieCfg := handler.CookieConfig{
		Name:      cfg.Auth.CookieName,
		Domain:    cfg.Auth.CookieDomain,
		Secure:    cfg.Auth.CookieSecure,
		MaxAgeSec: int(cfg.Auth.TTL.Seconds()),
	}
	authH := handler.NewAuthHandler(logger, authUC, cookieCfg)
	userH := handler.NewUserHandler(logger, userUC)
	settingH := handler.NewSettingHandler(logger, settingUC)
	notificationH := handler.NewNotificationHandler(logger, notificationUC)
	permissionH := handler.NewPermissionHandler(logger, permissionUC)
	// HMAC secret for storage upload tokens — reuse JWT secret so we don't
	// add a new ops surface for key rotation.
	storageH := handler.NewStorageHandler(logger, storage, cfg.Storage.MaxUploadBytes, []byte(cfg.Auth.Secret), cfg.HTTP.BaseURL)
	sseH := handler.NewSSEHandler(logger, notifHub)
	activityLogH := handler.NewActivityLogHandler(logger, auditRepo)

	httpHandler := handler.New(logger, authMW, internalMW, healthH, authH, userH, permissionH, notificationH, settingH, storageH, sseH, activityLogH)

	// ── background servers (TODO: cron jobs, asynq worker) ───────────────────
	var bg []BackgroundServer

	return &AppContainer{HTTPHandler: httpHandler, BackgroundServers: bg, TxRunner: tx}, func() {
		_ = notifHub.Close()
	}, nil
}
