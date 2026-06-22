// Command api runs the backend HTTP API server.
//
//	@title						Backend API
//	@version					1.0
//	@description				Backend API.
//	@host						localhost:8888
//	@BasePath					/api/v1
//	@securityDefinitions.apikey	BearerAuth
//	@in							header
//	@name						Authorization
//	@description				JWT token. Format: "Bearer <token>"
package main

import (
	"context"
	"errors"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"

	"vtv.vn/backend/internal/config"
	"vtv.vn/backend/internal/di"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xhttp/xmiddleware"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xpostgres"
	"vtv.vn/backend/pkg/xratelimit"
	"vtv.vn/backend/pkg/xredis"
)

// Version is set at build time via -ldflags "-X main.Version=...".
var Version = "dev"

func main() {
	env := flag.String("env", envOr("APP_ENV", "dev"), "environment: dev|staging|prod")
	flag.Parse()

	cfg, err := config.Load(*env)
	if err != nil {
		fatal("load config: " + err.Error())
	}
	logger, err := xlogger.New(&cfg.Logger)
	if err != nil {
		fatal("init logger: " + err.Error())
	}
	logger.Info("starting api", xlogger.String("env", cfg.Env), xlogger.String("version", Version))

	ctx := context.Background()
	db, err := xpostgres.NewDB(ctx, &cfg.Postgres)
	if err != nil {
		logger.Fatal("connect postgres", xlogger.Err(err))
	}
	rdb, err := xredis.New(ctx, &cfg.Redis)
	if err != nil {
		logger.Fatal("connect redis", xlogger.Err(err))
	}

	e := echo.New()
	e.HideBanner = true
	e.HTTPErrorHandler = xhttp.HTTPErrorHandler
	if len(cfg.HTTP.TrustedProxies) == 0 {
		e.IPExtractor = echo.ExtractIPDirect()
	}
	banGuard := xratelimit.NewIPBanGuard(rdb, logger, xratelimit.DefaultConfig())
	e.Use(
		middleware.Recover(),
		middleware.Secure(),
		middleware.CORSWithConfig(middleware.CORSConfig{
			AllowOrigins:     cfg.HTTP.CORSAllowOrigins,
			AllowCredentials: true,
			AllowMethods:     []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete},
		}),
		xmiddleware.RequestLogging(logger),
		// Auto-ban IPs that probe for vulnerabilities (PHP/.env/panel scanners).
		// Runs after logging so probes are still recorded, before routing so
		// banned IPs are rejected early. Redis-backed; fails open on errors.
		banGuard.Middleware(),
	)

	container, cleanup, err := di.NewAppContainer(cfg, logger, db, rdb, e, Version)
	if err != nil {
		logger.Fatal("wire app", xlogger.Err(err))
	}
	defer cleanup()
	container.HTTPHandler.RegisterRoutes(e)

	for _, s := range container.BackgroundServers {
		if err := s.Start(); err != nil {
			logger.Fatal("start background server", xlogger.Err(err))
		}
	}

	go func() {
		if err := e.Start(cfg.HTTP.Addr()); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server stopped", xlogger.Err(err))
		}
	}()
	logger.Info("listening", xlogger.String("addr", cfg.HTTP.Addr()))

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("shutting down…")

	shCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	for _, s := range container.BackgroundServers {
		s.Stop()
	}
	if err := e.Shutdown(shCtx); err != nil {
		logger.Error("graceful shutdown failed", xlogger.Err(err))
	}
	if sqlDB, derr := db.DB(); derr == nil {
		_ = sqlDB.Close()
	}
	_ = rdb.Close()
	logger.Info("stopped")
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func fatal(msg string) {
	_, _ = os.Stderr.WriteString("FATAL: " + msg + "\n")
	os.Exit(1)
}
