package handler

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"vtv.vn/backend/internal/domain/model"
	domainRepo "vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/internal/server/http/dto"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// ActivityLogHandler handles /api/v1/activity-logs endpoints.
// It reads directly from AuditRepository (read-only, no usecase needed).
type ActivityLogHandler struct {
	logger *xlogger.Logger
	repo   domainRepo.AuditRepository
}

// NewActivityLogHandler constructs an ActivityLogHandler.
func NewActivityLogHandler(logger *xlogger.Logger, repo domainRepo.AuditRepository) *ActivityLogHandler {
	return &ActivityLogHandler{logger: logger, repo: repo}
}

// List returns paginated, filtered activity log entries.
// GET /api/v1/activity-logs -> {"data": [...], "meta": {...}}
func (h *ActivityLogHandler) List(c echo.Context) error {
	page, _ := strconv.Atoi(c.QueryParam("page"))
	limit, _ := strconv.Atoi(c.QueryParam("limit"))

	filter := domainRepo.AuditFilter{
		Search:     c.QueryParam("search"),
		Action:     c.QueryParam("action"),
		EntityType: c.QueryParam("entityType"),
		DateFrom:   c.QueryParam("dateFrom"),
		DateTo:     c.QueryParam("dateTo"),
		Page:       page,
		Limit:      limit,
	}
	if s := c.QueryParam("userId"); s != "" {
		if uid, err := strconv.ParseInt(s, 10, 64); err == nil {
			filter.UserID = &uid
		}
	}

	rows, total, err := h.repo.ListLogs(c.Request().Context(), filter)
	if err != nil {
		return xhttp.AppErrorResponse(c, xhttp.InternalErrorf("không thể truy vấn activity logs"))
	}

	out := dto.ToAuditLogRowResponses(rows)

	type listResponse struct {
		Data  []model.AuditLogRowResponse `json:"data"`
		Total int64                       `json:"total"`
	}
	return c.JSON(http.StatusOK, listResponse{Data: out, Total: total})
}

// ListUsers returns distinct users who have audit log entries.
// GET /api/v1/activity-logs/users -> {"data": [...]}
func (h *ActivityLogHandler) ListUsers(c echo.Context) error {
	actors, err := h.repo.ListActors(c.Request().Context())
	if err != nil {
		return xhttp.AppErrorResponse(c, xhttp.InternalErrorf("không thể truy vấn users"))
	}
	if actors == nil {
		actors = []domainRepo.AuditActor{}
	}

	return xhttp.SuccessResponse(c, dto.ToAuditActorResponses(actors))
}
