package handler

import (
	"github.com/labstack/echo/v4"

	"vtv.vn/backend/internal/domain/model"
	domainUC "vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/internal/server/http/dto"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// NotificationHandler handles /api/v1/announcements and /api/v1/notifications routes.
type NotificationHandler struct {
	logger *xlogger.Logger
	uc     domainUC.NotificationUseCase
}

// NewNotificationHandler constructs a NotificationHandler.
func NewNotificationHandler(logger *xlogger.Logger, uc domainUC.NotificationUseCase) *NotificationHandler {
	return &NotificationHandler{logger: logger, uc: uc}
}

// ── announcements ─────────────────────────────────────────────────────────────

// ListAnnouncements godoc
// @Summary  Danh sách thông báo hệ thống
// @Tags     notifications
// @Produce  json
// @Param    all    query  bool  false  "Bao gồm cả bản nháp (admin)"
// @Param    page   query  int   false  "Trang"
// @Param    limit  query  int   false  "Số dòng/trang"
// @Success  200  {object}  xhttp.PagedEnvelope{data=[]dto.AnnouncementResponse}
// @Security BearerAuth
// @Router   /announcements [get]
func (h *NotificationHandler) ListAnnouncements(c echo.Context) error {
	includeAll := c.QueryParam("all") == "true"
	page, limit := parsePage(c, 20)
	items, total, err := h.uc.ListAnnouncements(c.Request().Context(), includeAll, model.NewPage(page, limit))
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	out := make([]dto.AnnouncementResponse, len(items))
	for i, a := range items {
		out[i] = dto.ToAnnouncementResponse(a)
	}
	return xhttp.PaginatedResponse(c, out, page, limit, total)
}

// GetAnnouncement godoc
// @Summary  Chi tiết thông báo hệ thống
// @Tags     notifications
// @Produce  json
// @Param    id  path  int  true  "Announcement ID"
// @Success  200  {object}  xhttp.Envelope{data=dto.AnnouncementResponse}
// @Failure  404  {object}  xhttp.Problem
// @Security BearerAuth
// @Router   /announcements/{id} [get]
func (h *NotificationHandler) GetAnnouncement(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	a, err := h.uc.GetAnnouncement(c.Request().Context(), id)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToAnnouncementResponse(*a))
}

// CreateAnnouncement godoc
// @Summary  Tạo thông báo hệ thống
// @Tags     notifications
// @Accept   json
// @Produce  json
// @Param    body  body  dto.AnnouncementRequest  true  "Nội dung thông báo"
// @Success  201  {object}  xhttp.Envelope{data=dto.AnnouncementResponse}
// @Security BearerAuth
// @Router   /announcements [post]
func (h *NotificationHandler) CreateAnnouncement(c echo.Context) error {
	var req dto.AnnouncementRequest
	if errs := xhttp.ReadAndValidateRequest(c, &req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	a, err := h.uc.CreateAnnouncement(c.Request().Context(), domainUC.AnnouncementInput{
		Title: req.Title, Body: req.Body, Priority: req.Priority,
		PublishedAt: req.PublishedAt, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.CreatedResponse(c, dto.ToAnnouncementResponse(*a))
}

// UpdateAnnouncement godoc
// @Summary  Cập nhật thông báo hệ thống
// @Tags     notifications
// @Accept   json
// @Produce  json
// @Param    id    path  int                      true  "Announcement ID"
// @Param    body  body  dto.AnnouncementRequest  true  "Nội dung cập nhật"
// @Success  200  {object}  xhttp.Envelope{data=dto.AnnouncementResponse}
// @Security BearerAuth
// @Router   /announcements/{id} [put]
func (h *NotificationHandler) UpdateAnnouncement(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	var req dto.AnnouncementRequest
	if errs := xhttp.ReadAndValidateRequest(c, &req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	a, err := h.uc.UpdateAnnouncement(c.Request().Context(), id, domainUC.AnnouncementInput{
		Title: req.Title, Body: req.Body, Priority: req.Priority,
		PublishedAt: req.PublishedAt, ExpiresAt: req.ExpiresAt,
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToAnnouncementResponse(*a))
}

// DeleteAnnouncement godoc
// @Summary  Xoá thông báo hệ thống
// @Tags     notifications
// @Produce  json
// @Param    id  path  int  true  "Announcement ID"
// @Success  204
// @Security BearerAuth
// @Router   /announcements/{id} [delete]
func (h *NotificationHandler) DeleteAnnouncement(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	if err := h.uc.DeleteAnnouncement(c.Request().Context(), id); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.NoContentResponse(c)
}

// PublishToAll godoc
// @Summary  Gửi thông báo đến tất cả người dùng
// @Tags     notifications
// @Produce  json
// @Param    id  path  int  true  "Announcement ID"
// @Success  200  {object}  xhttp.Envelope{data=dto.AnnouncementResponse}
// @Security BearerAuth
// @Router   /announcements/{id}/publish [post]
func (h *NotificationHandler) PublishToAll(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	a, err := h.uc.GetAnnouncement(c.Request().Context(), id)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	if err := h.uc.PublishToAll(c.Request().Context(), a.Title, a.Body, "announcement", a.ID); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToAnnouncementResponse(*a))
}

// ── announcement convenience endpoints (FE generated client expects these) ────

// AnnouncementUnreadCount returns unread announcement count for current user.
// GET /announcements/unread-count
func (h *NotificationHandler) AnnouncementUnreadCount(c echo.Context) error {
	userID := xhttp.UserID(c.Request().Context())
	count, err := h.uc.UnreadCount(c.Request().Context(), userID)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, map[string]int64{"count": count, "unread": count})
}

// AnnouncementReply is a stub for POST /announcements/:id/reply.
// Reply feature not implemented yet — returns 200 with empty data.
func (h *NotificationHandler) AnnouncementReply(c echo.Context) error {
	return xhttp.SuccessResponse(c, model.OkResponse{Ok: true})
}

// AnnouncementReads returns read/unread status for an announcement.
// GET /announcements/:id/reads — stub returning empty lists.
func (h *NotificationHandler) AnnouncementReads(c echo.Context) error {
	type reads struct {
		Reads  []any `json:"reads"`
		Unread []any `json:"unread"`
	}
	return xhttp.SuccessResponse(c, reads{Reads: []any{}, Unread: []any{}})
}

// ── notifications ─────────────────────────────────────────────────────────────

// ListMyNotifications godoc
// @Summary  Thông báo cá nhân của tôi
// @Tags     notifications
// @Produce  json
// @Param    unread      query  bool    false  "Chỉ lấy chưa đọc (alias: unreadOnly)"
// @Param    unreadOnly  query  bool    false  "Chỉ lấy chưa đọc"
// @Param    important   query  bool    false  "Chỉ lấy thông báo quan trọng"
// @Param    category    query  string  false  "Phân loại: task | announcement | system"
// @Param    dateFrom    query  string  false  "RFC3339 từ ngày"
// @Param    dateTo      query  string  false  "RFC3339 đến ngày"
// @Param    page        query  int     false  "Trang"
// @Param    limit       query  int     false  "Số dòng/trang"
// @Success  200  {object}  xhttp.PagedEnvelope{data=[]dto.NotificationResponse}
// @Security BearerAuth
// @Router   /notifications [get]
func (h *NotificationHandler) ListMyNotifications(c echo.Context) error {
	ctx := c.Request().Context()
	userID := xhttp.UserID(ctx)

	unreadOnly := c.QueryParam("unread") == "true" || c.QueryParam("unreadOnly") == "true"
	importantOnly := c.QueryParam("important") == "true" || c.QueryParam("importantOnly") == "true"
	category := c.QueryParam("category")
	var dateFrom, dateTo *string
	if v := c.QueryParam("dateFrom"); v != "" {
		dateFrom = &v
	}
	if v := c.QueryParam("dateTo"); v != "" {
		dateTo = &v
	}
	page, limit := parsePage(c, 20)

	filter := domainUC.NotificationListFilter{
		UnreadOnly:    unreadOnly,
		ImportantOnly: importantOnly,
		Category:      category,
		DateFrom:      dateFrom,
		DateTo:        dateTo,
	}
	items, total, err := h.uc.ListForUserFiltered(ctx, userID, filter, model.NewPage(page, limit))
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	// Always include unreadCount in the response (FE NotificationBell expects it).
	unread, _ := h.uc.UnreadCount(ctx, userID)
	out := dto.ToNotificationResponses(items)
	return xhttp.PaginatedResponseWithMeta(c, out, page, limit, total, map[string]any{"unreadCount": unread})
}

// UnreadCount godoc
// @Summary  Số thông báo chưa đọc
// @Tags     notifications
// @Produce  json
// @Success  200  {object}  xhttp.Envelope
// @Security BearerAuth
// @Router   /notifications/unread-count [get]
func (h *NotificationHandler) UnreadCount(c echo.Context) error {
	userID := xhttp.UserID(c.Request().Context())
	count, err := h.uc.UnreadCount(c.Request().Context(), userID)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, map[string]int64{"count": count, "unread": count})
}

// MarkRead godoc
// @Summary  Đánh dấu thông báo đã đọc
// @Tags     notifications
// @Produce  json
// @Param    id  path  int  true  "Notification ID"
// @Success  204
// @Security BearerAuth
// @Router   /notifications/{id}/read [post]
func (h *NotificationHandler) MarkRead(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	userID := xhttp.UserID(c.Request().Context())
	if err := h.uc.MarkRead(c.Request().Context(), id, userID); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.NoContentResponse(c)
}

// MarkAllRead godoc
// @Summary  Đánh dấu tất cả thông báo đã đọc
// @Tags     notifications
// @Produce  json
// @Success  204
// @Security BearerAuth
// @Router   /notifications/read-all [post]
func (h *NotificationHandler) MarkAllRead(c echo.Context) error {
	userID := xhttp.UserID(c.Request().Context())
	if err := h.uc.MarkAllRead(c.Request().Context(), userID); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.NoContentResponse(c)
}

// GetMyNotification godoc
// @Summary  Chi tiết một thông báo cá nhân (đồng thời mark read)
// @Tags     notifications
// @Produce  json
// @Param    id  path  int  true  "Notification ID"
// @Success  200  {object}  xhttp.Envelope{data=dto.NotificationResponse}
// @Security BearerAuth
// @Router   /notifications/{id} [get]
func (h *NotificationHandler) GetMyNotification(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	ctx := c.Request().Context()
	userID := xhttp.UserID(ctx)
	n, err := h.uc.GetForUser(ctx, id, userID)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	// Auto-mark as read on detail view (UX convenience).
	if !n.IsRead {
		_ = h.uc.MarkRead(ctx, id, userID)
		n.IsRead = true
	}
	return xhttp.SuccessResponse(c, dto.ToNotificationResponse(*n))
}

// MarkUnread godoc
// @Summary  Đánh dấu thông báo chưa đọc
// @Tags     notifications
// @Produce  json
// @Param    id  path  int  true  "Notification ID"
// @Success  204
// @Security BearerAuth
// @Router   /notifications/{id}/unread [post]
func (h *NotificationHandler) MarkUnread(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	userID := xhttp.UserID(c.Request().Context())
	if err := h.uc.MarkUnread(c.Request().Context(), id, userID); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.NoContentResponse(c)
}

// SetImportantRequest toggles the important flag on a notification.
type SetImportantRequest struct {
	Important bool `json:"important"`
}

// SetImportant godoc
// @Summary  Cập nhật cờ "quan trọng" cho thông báo
// @Tags     notifications
// @Accept   json
// @Produce  json
// @Param    id    path  int                     true  "Notification ID"
// @Param    body  body  SetImportantRequest     true  "{important: bool}"
// @Success  204
// @Security BearerAuth
// @Router   /notifications/{id}/important [post]
func (h *NotificationHandler) SetImportant(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return err
	}
	var req SetImportantRequest
	// Default to toggle ON if body is missing.
	if c.Request().ContentLength > 0 {
		if bindErr := c.Bind(&req); bindErr != nil {
			return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("body không hợp lệ"))
		}
	} else {
		req.Important = true
	}
	userID := xhttp.UserID(c.Request().Context())
	if err := h.uc.SetImportant(c.Request().Context(), id, userID, req.Important); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.NoContentResponse(c)
}
