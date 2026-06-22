package usecase

import (
	"context"

	"vtv.vn/backend/internal/domain/model"
)

// AnnouncementInput is the input for creating/updating an announcement.
type AnnouncementInput struct {
	Title       string
	Body        string
	Priority    string
	PublishedAt *string // "2006-01-02T15:04:05Z" or nil
	ExpiresAt   *string
}

// NotificationInput is the input for emitting a single notification to a user.
type NotificationInput struct {
	UserID      int64
	Type        string // app-defined type (e.g. "item_created", "task_due")
	Category    string // "task" | "announcement" | "system" (empty defaults to "task")
	Title       string
	Body        string
	Link        *string // deep link (FE route)
	RefType     *string
	RefID       *int64
	IsImportant bool
}

// NotificationListFilter narrows the inbox query.
type NotificationListFilter struct {
	UnreadOnly    bool
	ImportantOnly bool
	Category      string  // "" = all
	DateFrom      *string // RFC3339
	DateTo        *string // RFC3339
}

// NotificationUseCase defines notification and announcement operations.
type NotificationUseCase interface {
	// Announcements
	CreateAnnouncement(ctx context.Context, in AnnouncementInput) (*model.Announcement, error)
	UpdateAnnouncement(ctx context.Context, id int64, in AnnouncementInput) (*model.Announcement, error)
	DeleteAnnouncement(ctx context.Context, id int64) error
	GetAnnouncement(ctx context.Context, id int64) (*model.Announcement, error)
	ListAnnouncements(ctx context.Context, includeUnpublished bool, page model.Page) ([]model.Announcement, int64, error)

	// Notifications
	ListForUser(ctx context.Context, userID int64, unreadOnly bool, page model.Page) ([]model.Notification, int64, error)
	ListForUserFiltered(ctx context.Context, userID int64, f NotificationListFilter, page model.Page) ([]model.Notification, int64, error)
	GetForUser(ctx context.Context, id int64, userID int64) (*model.Notification, error)
	MarkRead(ctx context.Context, id int64, userID int64) error
	MarkUnread(ctx context.Context, id int64, userID int64) error
	MarkAllRead(ctx context.Context, userID int64) error
	SetImportant(ctx context.Context, id int64, userID int64, important bool) error
	UnreadCount(ctx context.Context, userID int64) (int64, error)

	// Emit creates a notification for a single user and pushes via realtime.
	Emit(ctx context.Context, in NotificationInput) error
	// EmitToUsers fans-out a notification to multiple users.
	EmitToUsers(ctx context.Context, userIDs []int64, in NotificationInput) error
	// PublishToAll sends a notification to all active users (announcement fan-out).
	PublishToAll(ctx context.Context, title, body, refType string, refID int64) error
}
