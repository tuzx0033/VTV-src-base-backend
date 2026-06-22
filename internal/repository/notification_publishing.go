package repository

import (
	"context"
	"encoding/json"

	"vtv.vn/backend/internal/domain/model"
	domainRepo "vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/internal/server/http/dto"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xnotify"
)

// PublishingNotificationRepository wraps a NotificationRepository and publishes
// real-time events to the notification hub after successful DB writes.
// Publish errors are logged but never propagated (notifications are best-effort).
type PublishingNotificationRepository struct {
	inner  domainRepo.NotificationRepository
	hub    xnotify.Hub
	logger *xlogger.Logger
}

// NewPublishingNotificationRepository wraps inner with real-time push via hub.
func NewPublishingNotificationRepository(
	inner domainRepo.NotificationRepository,
	hub xnotify.Hub,
	logger *xlogger.Logger,
) domainRepo.NotificationRepository {
	return &PublishingNotificationRepository{inner: inner, hub: hub, logger: logger}
}

func (r *PublishingNotificationRepository) Create(ctx context.Context, n *model.Notification) error {
	if err := r.inner.Create(ctx, n); err != nil {
		return err
	}
	r.publishNew(ctx, n)
	return nil
}

func (r *PublishingNotificationRepository) BulkCreate(ctx context.Context, ns []model.Notification) error {
	if err := r.inner.BulkCreate(ctx, ns); err != nil {
		return err
	}
	// Publish each notification to its respective user.
	for i := range ns {
		r.publishNew(ctx, &ns[i])
	}
	return nil
}

func (r *PublishingNotificationRepository) ListForUser(ctx context.Context, userID int64, unreadOnly bool, page model.Page) ([]model.Notification, int64, error) {
	return r.inner.ListForUser(ctx, userID, unreadOnly, page)
}

func (r *PublishingNotificationRepository) ListForUserFiltered(ctx context.Context, userID int64, f model.NotificationFilter, page model.Page) ([]model.Notification, int64, error) {
	return r.inner.ListForUserFiltered(ctx, userID, f, page)
}

func (r *PublishingNotificationRepository) GetByID(ctx context.Context, id int64, userID int64) (*model.Notification, error) {
	return r.inner.GetByID(ctx, id, userID)
}

func (r *PublishingNotificationRepository) MarkRead(ctx context.Context, id int64, userID int64) error {
	return r.inner.MarkRead(ctx, id, userID)
}

func (r *PublishingNotificationRepository) MarkUnread(ctx context.Context, id int64, userID int64) error {
	return r.inner.MarkUnread(ctx, id, userID)
}

func (r *PublishingNotificationRepository) SetImportant(ctx context.Context, id int64, userID int64, important bool) error {
	return r.inner.SetImportant(ctx, id, userID, important)
}

func (r *PublishingNotificationRepository) MarkAllRead(ctx context.Context, userID int64) error {
	return r.inner.MarkAllRead(ctx, userID)
}

func (r *PublishingNotificationRepository) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	return r.inner.UnreadCount(ctx, userID)
}

// publishNew publishes a "new" event for the given notification.
func (r *PublishingNotificationRepository) publishNew(ctx context.Context, n *model.Notification) {
	resp := dto.ToNotificationResponse(*n)
	data, err := json.Marshal(resp)
	if err != nil {
		r.logger.Warn("publishingRepo: marshal notification",
			xlogger.Err(err),
			xlogger.Int64("notifID", n.ID),
		)
		return
	}
	event := xnotify.Event{Type: xnotify.EventNew, Data: data}
	if pErr := r.hub.Publish(ctx, n.UserID, event); pErr != nil {
		r.logger.Warn("publishingRepo: publish event",
			xlogger.Err(pErr),
			xlogger.Int64("userID", n.UserID),
			xlogger.Int64("notifID", n.ID),
		)
	}
}
