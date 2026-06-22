package repository

import (
	"context"

	"vtv.vn/backend/internal/domain/model"
)

// AnnouncementRepository persists announcements.
type AnnouncementRepository interface {
	Create(ctx context.Context, a *model.Announcement) error
	Update(ctx context.Context, a *model.Announcement) error
	GetByID(ctx context.Context, id int64) (*model.Announcement, error)
	List(ctx context.Context, includeUnpublished bool, page model.Page) ([]model.Announcement, int64, error)
	SoftDelete(ctx context.Context, id int64, byUserID int64) error
}

// NotificationRepository persists per-user notifications.
type NotificationRepository interface {
	Create(ctx context.Context, n *model.Notification) error
	BulkCreate(ctx context.Context, ns []model.Notification) error
	ListForUser(ctx context.Context, userID int64, unreadOnly bool, page model.Page) ([]model.Notification, int64, error)
	// ListForUserFiltered supports category, important, dateRange filters.
	ListForUserFiltered(ctx context.Context, userID int64, f model.NotificationFilter, page model.Page) ([]model.Notification, int64, error)
	GetByID(ctx context.Context, id int64, userID int64) (*model.Notification, error)
	MarkRead(ctx context.Context, id int64, userID int64) error
	MarkUnread(ctx context.Context, id int64, userID int64) error
	MarkAllRead(ctx context.Context, userID int64) error
	SetImportant(ctx context.Context, id int64, userID int64, important bool) error
	UnreadCount(ctx context.Context, userID int64) (int64, error)
}
