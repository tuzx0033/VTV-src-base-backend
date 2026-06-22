package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xnotify"
)

// SSEHandler serves the Server-Sent Events stream for real-time notifications.
type SSEHandler struct {
	logger *xlogger.Logger
	hub    xnotify.Hub
}

// NewSSEHandler constructs an SSEHandler.
func NewSSEHandler(logger *xlogger.Logger, hub xnotify.Hub) *SSEHandler {
	return &SSEHandler{logger: logger, hub: hub}
}

// Stream handles GET /api/notifications/stream.
// It keeps the connection open and pushes notification events via SSE.
func (h *SSEHandler) Stream(c echo.Context) error {
	userID := xhttp.UserID(c.Request().Context())
	if userID == 0 {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
	}

	// Subscribe to the notification hub.
	ch, cancel, err := h.hub.Subscribe(c.Request().Context(), userID)
	if err != nil {
		h.logger.Warn("sse: subscribe failed",
			xlogger.Int64("userID", userID),
			xlogger.Err(err),
		)
		return c.JSON(http.StatusTooManyRequests, map[string]string{
			"error": "quá nhiều kết nối, vui lòng đóng tab khác",
		})
	}
	defer cancel()

	// Set SSE headers.
	resp := c.Response()
	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.Header().Set("X-Accel-Buffering", "no") // disable nginx buffering
	resp.WriteHeader(http.StatusOK)

	// Send initial connected comment.
	if _, err := fmt.Fprint(resp, ": connected\n\n"); err != nil {
		return nil
	}
	resp.Flush()

	h.logger.Info("sse: client connected", xlogger.Int64("userID", userID))

	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()

	ctx := c.Request().Context()
	for {
		select {
		case event, ok := <-ch:
			if !ok {
				// Channel closed (hub shutting down or cancel called).
				return nil
			}
			data, mErr := json.Marshal(event)
			if mErr != nil {
				h.logger.Warn("sse: marshal event", xlogger.Err(mErr))
				continue
			}
			if _, wErr := fmt.Fprintf(resp, "event: %s\ndata: %s\n\n", event.Type, data); wErr != nil {
				return nil // client disconnected
			}
			resp.Flush()

		case <-ticker.C:
			// Heartbeat keeps connection alive through proxies.
			if _, wErr := fmt.Fprint(resp, ": heartbeat\n\n"); wErr != nil {
				return nil // client disconnected
			}
			resp.Flush()

		case <-ctx.Done():
			h.logger.Info("sse: client disconnected", xlogger.Int64("userID", userID))
			return nil
		}
	}
}
