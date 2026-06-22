package xhttp

import (
	"context"
	"time"
)

type ctxKey int

const (
	ctxKeyUserID ctxKey = iota
	ctxKeyRole
	ctxKeyRequestID
	ctxKeyTokenID
	ctxKeyTokenExpiry
	ctxKeyClientIP
)

// WithUserID stores the authenticated user id in ctx.
func WithUserID(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKeyUserID, id)
}

// UserID returns the authenticated user id, or 0 if none.
func UserID(ctx context.Context) int64 {
	if v, ok := ctx.Value(ctxKeyUserID).(int64); ok {
		return v
	}
	return 0
}

// WithRole stores the authenticated user's role in ctx.
func WithRole(ctx context.Context, role string) context.Context {
	return context.WithValue(ctx, ctxKeyRole, role)
}

// Role returns the authenticated user's role, or "" if none.
func Role(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRole).(string); ok {
		return v
	}
	return ""
}

// WithRequestID stores the request id in ctx.
func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, ctxKeyRequestID, id)
}

// RequestID returns the request id, or "" if none.
func RequestID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyRequestID).(string); ok {
		return v
	}
	return ""
}

// WithTokenID stores the JWT ID (JTI) of the authenticated token in ctx.
func WithTokenID(ctx context.Context, jti string) context.Context {
	return context.WithValue(ctx, ctxKeyTokenID, jti)
}

// TokenID returns the JWT ID (JTI), or "" if none.
func TokenID(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyTokenID).(string); ok {
		return v
	}
	return ""
}

// WithTokenExpiry stores the expiry time of the authenticated token in ctx.
func WithTokenExpiry(ctx context.Context, t time.Time) context.Context {
	return context.WithValue(ctx, ctxKeyTokenExpiry, t)
}

// TokenExpiry returns the token expiry, or zero time if none.
func TokenExpiry(ctx context.Context) time.Time {
	if v, ok := ctx.Value(ctxKeyTokenExpiry).(time.Time); ok {
		return v
	}
	return time.Time{}
}

// WithClientIP stores the request's client IP in ctx (used by audit_log to
// attribute every mutation to a network identity).
func WithClientIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ctxKeyClientIP, ip)
}

// ClientIP returns the request's client IP, or "" if none.
func ClientIP(ctx context.Context) string {
	if v, ok := ctx.Value(ctxKeyClientIP).(string); ok {
		return v
	}
	return ""
}
