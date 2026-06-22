package handler

import (
	"strconv"

	"github.com/labstack/echo/v4"

	"vtv.vn/backend/pkg/xhttp"
)

// parseIDParam extracts ":id" path param as int64.
// Returns a 400 AppError if the value is not a positive integer.
func parseIDParam(c echo.Context) (int64, error) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil || id <= 0 {
		return 0, xhttp.BadRequestErrorf("id không hợp lệ")
	}
	return id, nil
}

// safeInt64Param extracts a named path param as int64.
// Returns a 400 AppError if the value is not a positive integer.
func safeInt64Param(c echo.Context, name string) (int64, error) {
	id, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || id <= 0 {
		return 0, xhttp.BadRequestErrorf("%s không hợp lệ", name)
	}
	return id, nil
}

// mustInt converts a string to int64, returning 0 on failure.
// Prefer safeInt64Param for proper error handling.
func mustInt(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

// parsePeriodParams extracts optional ?periodYear and ?periodMonth query params.
// Returns zero values when the params are absent or malformed.
func parsePeriodParams(c echo.Context) (year, month int) {
	year, _ = strconv.Atoi(c.QueryParam("periodYear"))
	month, _ = strconv.Atoi(c.QueryParam("periodMonth"))
	return
}

// parsePage extracts optional ?page and ?limit query params with sensible defaults.
func parsePage(c echo.Context, defaultLimit int) (page, limit int) {
	page = 1
	limit = defaultLimit
	if p, err := strconv.Atoi(c.QueryParam("page")); err == nil && p > 0 {
		page = p
	}
	if l, err := strconv.Atoi(c.QueryParam("limit")); err == nil && l > 0 {
		limit = l
	}
	return
}
