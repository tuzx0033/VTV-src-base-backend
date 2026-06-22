package xmiddleware

import (
	"bytes"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"vtv.vn/backend/pkg/xlogger"
)

// logLine is the subset of fields we assert on.
type logLine struct {
	Level  string `json:"level"`
	Status int    `json:"status"`
	Path   string `json:"path"`
	Msg    string `json:"message"`
}

// runReq drives a single request through the RequestLogging middleware wrapping
// handler h, returning the parsed log line that was emitted.
func runReq(t *testing.T, h echo.HandlerFunc, path string) logLine {
	t.Helper()
	var buf bytes.Buffer
	logger := xlogger.NewWithWriter(&buf, "debug")

	e := echo.New()
	// Wire the same error handler used in production so route/handler errors
	// flip the response status exactly like the real server.
	e.Use(RequestLogging(logger))
	e.GET("/ok", h)

	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	var line logLine
	if err := json.Unmarshal(bytes.TrimSpace(buf.Bytes()), &line); err != nil {
		t.Fatalf("log not valid json: %v\nraw=%s", err, buf.String())
	}
	return line
}

func TestRequestLogging_Status200_Info(t *testing.T) {
	line := runReq(t, func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, "/ok")
	if line.Status != 200 {
		t.Fatalf("status = %d, want 200", line.Status)
	}
	if line.Level != "info" {
		t.Fatalf("level = %q, want info", line.Level)
	}
}

func TestRequestLogging_RouteNotFound_LogsRealStatusAsWarn(t *testing.T) {
	// Hitting an unregistered path makes Echo return echo.ErrNotFound. This is
	// the bot-scan case: status must log as 404 (not the default 200) and at
	// warn level (not error).
	line := runReq(t, func(c echo.Context) error {
		return c.String(http.StatusOK, "ok")
	}, "/this-route-does-not-exist")
	if line.Status != 404 {
		t.Fatalf("status = %d, want 404 (was logging 200 before fix)", line.Status)
	}
	if line.Level != "warn" {
		t.Fatalf("level = %q, want warn (was error before fix)", line.Level)
	}
}

func TestRequestLogging_HandlerHTTPError404_Warn(t *testing.T) {
	line := runReq(t, func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusNotFound, "item not found")
	}, "/ok")
	if line.Status != 404 {
		t.Fatalf("status = %d, want 404", line.Status)
	}
	if line.Level != "warn" {
		t.Fatalf("level = %q, want warn", line.Level)
	}
}

func TestRequestLogging_HandlerHTTPError400_Warn(t *testing.T) {
	line := runReq(t, func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusBadRequest, "bad input")
	}, "/ok")
	if line.Status != 400 || line.Level != "warn" {
		t.Fatalf("status=%d level=%q, want 400/warn", line.Status, line.Level)
	}
}

func TestRequestLogging_HandlerHTTPError500_Error(t *testing.T) {
	line := runReq(t, func(c echo.Context) error {
		return echo.NewHTTPError(http.StatusInternalServerError, "boom")
	}, "/ok")
	if line.Status != 500 || line.Level != "error" {
		t.Fatalf("status=%d level=%q, want 500/error", line.Status, line.Level)
	}
}

func TestRequestLogging_PlainError_TreatedAs500Error(t *testing.T) {
	// A non-HTTPError surfaced from the handler with no committed status must
	// be classified as a 500 server error.
	line := runReq(t, func(c echo.Context) error {
		return errors.New("unexpected failure")
	}, "/ok")
	if line.Status != 500 {
		t.Fatalf("status = %d, want 500", line.Status)
	}
	if line.Level != "error" {
		t.Fatalf("level = %q, want error", line.Level)
	}
}
