package repository

import (
	"context"
	"time"
)

// AuditEntry is one immutable change-log record. Never put secrets/raw bodies in Changes.
type AuditEntry struct {
	ActorUserID int64
	ActorName   string
	ActorRole   string
	Action      string // taxonomy: <entity>.<verb>
	EntityType  string
	EntityID    int64
	EntityName  string
	Summary     string
	Changes     map[string]any
	IPAddress   string
	RequestID   string
	CreatedAt   time.Time // UTC; zero → set by repo
}

// AuditRepository writes audit_log rows. Call inside the same transaction as the mutation.
type AuditRepository interface {
	Write(ctx context.Context, e AuditEntry) error

	// ListLogs returns paginated, filtered activity log entries.
	ListLogs(ctx context.Context, filter AuditFilter) ([]AuditLogRow, int64, error)
	// ListActors returns distinct users who have audit entries.
	ListActors(ctx context.Context) ([]AuditActor, error)
}

// AuditFilter holds query params for listing audit logs.
type AuditFilter struct {
	Search     string
	Action     string
	UserID     *int64
	EntityType string
	DateFrom   string // YYYY-MM-DD
	DateTo     string // YYYY-MM-DD
	Page       int
	Limit      int
}

// AuditLogRow is a single audit log entry returned by ListLogs.
type AuditLogRow struct {
	ID           int64
	UserID       *int64
	UserFullName *string
	UserRole     *string
	Action       string
	EntityType   string
	EntityID     *int64
	EntityName   *string
	Description  *string
	Metadata     []byte // raw JSON
	IPAddress    *string
	CreatedAt    string
}

// AuditActor is a distinct user who has audit entries.
type AuditActor struct {
	UserID       int64
	UserFullName string
	UserRole     string
}
