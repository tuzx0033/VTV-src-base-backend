package xhttp

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
)

// Envelope is the standard success body: {"data": ...}.
type Envelope struct {
	Data any `json:"data"`
}

// PageMeta is pagination metadata.
type PageMeta struct {
	Page       int   `json:"page"`
	Limit      int   `json:"limit"`
	Total      int64 `json:"total"`
	TotalPages int64 `json:"totalPages"`
}

// PagedEnvelope is the list body: {"data": [...], "meta": {...}}.
type PagedEnvelope struct {
	Data any      `json:"data"`
	Meta PageMeta `json:"meta"`
}

// Problem is an RFC 7807 problem-details body, extended with a stable `code`.
type Problem struct {
	Type   string            `json:"type"`             // URI ref; we use "about:blank" + code
	Title  string            `json:"title"`            // short human title
	Status int               `json:"status"`           // HTTP status
	Detail string            `json:"detail,omitempty"` // human explanation
	Code   string            `json:"code"`             // stable machine code (FE branches on this)
	Field  string            `json:"field,omitempty"`  // offending field, if any
	Errors []ValidationError `json:"errors,omitempty"` // for validation failures
}

// SuccessBody is the Swagger schema helper for success responses.
type SuccessBody struct {
	Data any `json:"data"`
}

// ErrorBody is the Swagger schema helper for error responses.
type ErrorBody struct {
	Type   string `json:"type"`
	Title  string `json:"title"`
	Status int    `json:"status"`
	Detail string `json:"detail"`
	Code   string `json:"code"`
}

func SuccessResponse(c echo.Context, data any) error {
	return c.JSON(http.StatusOK, Envelope{Data: data})
}

func CreatedResponse(c echo.Context, data any) error {
	return c.JSON(http.StatusCreated, Envelope{Data: data})
}

func NoContentResponse(c echo.Context) error {
	return c.NoContent(http.StatusNoContent)
}

func PaginatedResponse(c echo.Context, data any, page, limit int, total int64) error {
	tp := int64(0)
	if limit > 0 {
		tp = (total + int64(limit) - 1) / int64(limit)
	}
	return c.JSON(http.StatusOK, PagedEnvelope{Data: data, Meta: PageMeta{Page: page, Limit: limit, Total: total, TotalPages: tp}})
}

// PaginatedResponseWithMeta returns a list response with extra top-level fields
// merged alongside `items` (FE-friendly flat shape). Use this when the FE expects
// inline metadata (e.g. unreadCount) without unwrapping `data`.
func PaginatedResponseWithMeta(c echo.Context, items any, page, limit int, total int64, extra map[string]any) error {
	tp := int64(0)
	if limit > 0 {
		tp = (total + int64(limit) - 1) / int64(limit)
	}
	body := map[string]any{
		"items":      items,
		"data":       items, // backward-compat with PagedEnvelope consumers
		"page":       page,
		"limit":      limit,
		"total":      total,
		"totalPages": tp,
		"meta":       PageMeta{Page: page, Limit: limit, Total: total, TotalPages: tp},
	}
	for k, v := range extra {
		body[k] = v
	}
	return c.JSON(http.StatusOK, body)
}

// RawErrorContextKey is the echo.Context key under which AppErrorResponse stores
// the unwrapped error (or non-AppError) so the request-logging middleware can
// surface it in structured logs without leaking it to the client.
const RawErrorContextKey = "xhttp.raw_error"

// AppErrorResponse renders err as RFC 7807. Non-AppError errors are masked as a
// generic 500. The original error (or *AppError.Err) is stored on the echo
// context under RawErrorContextKey so request-logging middleware can log it.
func AppErrorResponse(c echo.Context, err error) error {
	if ae, ok := AsAppError(err); ok {
		if ae.Err != nil {
			c.Set(RawErrorContextKey, ae.Err)
		}
		return c.JSON(ae.Status, Problem{
			Type:   "about:blank",
			Title:  http.StatusText(ae.Status),
			Status: ae.Status,
			Detail: ae.Message,
			Code:   ae.Code,
			Field:  ae.Field,
		})
	}
	c.Set(RawErrorContextKey, err)
	return c.JSON(http.StatusInternalServerError, Problem{
		Type:   "about:blank",
		Title:  http.StatusText(http.StatusInternalServerError),
		Status: http.StatusInternalServerError,
		Detail: "Đã có lỗi xảy ra. Vui lòng thử lại sau.",
		Code:   "INTERNAL",
	})
}

// BadRequestResponse renders validation errors as RFC 7807 (422).
func BadRequestResponse(c echo.Context, errs []ValidationError) error {
	return c.JSON(http.StatusUnprocessableEntity, Problem{
		Type:   "about:blank",
		Title:  "Validation failed",
		Status: http.StatusUnprocessableEntity,
		Detail: "Dữ liệu gửi lên không hợp lệ.",
		Code:   "VALIDATION_FAILED",
		Errors: errs,
	})
}

// HTTPErrorHandler is the Echo-level fallback error handler.
func HTTPErrorHandler(err error, c echo.Context) {
	if c.Response().Committed {
		return
	}
	var he *echo.HTTPError
	if errors.As(err, &he) {
		msg := http.StatusText(he.Code)
		if s, ok := he.Message.(string); ok {
			msg = s
		}
		_ = c.JSON(he.Code, Problem{Type: "about:blank", Title: http.StatusText(he.Code), Status: he.Code, Detail: msg, Code: "HTTP_ERROR"})
		return
	}
	_ = AppErrorResponse(c, err)
}
