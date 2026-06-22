package xhttp

import (
	"errors"
	"fmt"
	"net/http"
)

// AppError is a typed application error with a stable `Code`, an HTTP `Status`,
// a human `Message`, an optional `Field` (for validation-ish errors), and an
// optional wrapped `Err` (never exposed to clients). Construct via the helpers
// below — do NOT build AppError{} literals.
type AppError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
	Status  int    `json:"-"`
	Err     error  `json:"-"`
}

func (e *AppError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Code, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

func (e *AppError) Unwrap() error { return e.Err }

// WithField returns a copy with Field set.
func (e *AppError) WithField(field string) *AppError {
	c := *e
	c.Field = field
	return &c
}

// Wrap attaches an underlying error (logged, not exposed).
func (e *AppError) Wrap(err error) *AppError {
	c := *e
	c.Err = err
	return &c
}

func newf(code string, status int, format string, a ...any) *AppError {
	return &AppError{Code: code, Status: status, Message: fmt.Sprintf(format, a...)}
}

func BadRequestErrorf(format string, a ...any) *AppError {
	return newf("BAD_REQUEST", http.StatusBadRequest, format, a...)
}
func UnauthorizedErrorf(format string, a ...any) *AppError {
	return newf("UNAUTHORIZED", http.StatusUnauthorized, format, a...)
}
func ForbiddenErrorf(format string, a ...any) *AppError {
	return newf("FORBIDDEN", http.StatusForbidden, format, a...)
}

// ForcePasswordChangeErrorf returns a 403 with a stable "FORCE_PASSWORD_CHANGE"
// code — clients (web FE) phải redirect user về /change-password-required
// thay vì hiển thị generic "không đủ quyền".
func ForcePasswordChangeErrorf(format string, a ...any) *AppError {
	return newf("FORCE_PASSWORD_CHANGE", http.StatusForbidden, format, a...)
}
func NotFoundErrorf(format string, a ...any) *AppError {
	return newf("NOT_FOUND", http.StatusNotFound, format, a...)
}
func ConflictErrorf(format string, a ...any) *AppError {
	return newf("CONFLICT", http.StatusConflict, format, a...)
}
func UnprocessableErrorf(format string, a ...any) *AppError {
	return newf("UNPROCESSABLE", http.StatusUnprocessableEntity, format, a...)
}
func TooManyRequestsErrorf(format string, a ...any) *AppError {
	return newf("TOO_MANY_REQUESTS", http.StatusTooManyRequests, format, a...)
}
func ServiceUnavailableErrorf(format string, a ...any) *AppError {
	return newf("SERVICE_UNAVAILABLE", http.StatusServiceUnavailable, format, a...)
}
func InternalErrorf(format string, a ...any) *AppError {
	return newf("INTERNAL", http.StatusInternalServerError, format, a...)
}

// AsAppError returns the underlying *AppError if err is (or wraps) one.
func AsAppError(err error) (*AppError, bool) {
	var ae *AppError
	if errors.As(err, &ae) {
		return ae, true
	}
	return nil, false
}
