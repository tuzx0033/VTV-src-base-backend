package handler

import (
	"github.com/labstack/echo/v4"
	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// HealthHandler serves liveness/readiness/version.
type HealthHandler struct {
	logger  *xlogger.Logger
	db      *gorm.DB
	rdb     *redis.Client
	version string
}

// NewHealthHandler builds the health handler.
func NewHealthHandler(logger *xlogger.Logger, db *gorm.DB, rdb *redis.Client, version string) *HealthHandler {
	return &HealthHandler{logger: logger, db: db, rdb: rdb, version: version}
}

// Healthz reports process liveness.
// @Summary Liveness probe
// @Tags health
// @Produce json
// @Success 200 {object} xhttp.Envelope
// @Router /healthz [get]
func (h *HealthHandler) Healthz(c echo.Context) error {
	return xhttp.SuccessResponse(c, map[string]string{"status": "ok"})
}

// Readyz reports readiness (Postgres + Redis reachable).
// @Summary Readiness probe
// @Tags health
// @Produce json
// @Success 200 {object} xhttp.Envelope
// @Failure 503 {object} xhttp.Problem
// @Router /readyz [get]
func (h *HealthHandler) Readyz(c echo.Context) error {
	ctx := c.Request().Context()
	if sqlDB, err := h.db.DB(); err != nil {
		return h.unavailable(c, "postgres", err)
	} else if err := sqlDB.PingContext(ctx); err != nil {
		return h.unavailable(c, "postgres", err)
	}
	if err := h.rdb.Ping(ctx).Err(); err != nil {
		return h.unavailable(c, "redis", err)
	}
	return xhttp.SuccessResponse(c, map[string]string{"status": "ready"})
}

func (h *HealthHandler) unavailable(c echo.Context, dep string, err error) error {
	h.logger.Error("readiness check failed", xlogger.String("dependency", dep), xlogger.Err(err))
	return xhttp.AppErrorResponse(c, xhttp.ServiceUnavailableErrorf("%s not ready", dep).Wrap(err))
}

// Version returns the build version.
// @Summary Build version
// @Tags health
// @Produce json
// @Success 200 {object} xhttp.Envelope
// @Router /version [get]
func (h *HealthHandler) Version(c echo.Context) error {
	return xhttp.SuccessResponse(c, map[string]string{"version": h.version, "service": "backend"})
}
