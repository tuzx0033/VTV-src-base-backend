package usecase

import (
	"context"
	"time"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
	domainUC "vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xpostgres"
)

type notificationUseCase struct {
	logger        *xlogger.Logger
	tx            xpostgres.TxRunner
	announcements repository.AnnouncementRepository
	notifications repository.NotificationRepository
	users         repository.UserRepository
	audits        repository.AuditRepository
}

// NewNotificationUseCase constructs the notification use case.
func NewNotificationUseCase(
	logger *xlogger.Logger,
	tx xpostgres.TxRunner,
	announcements repository.AnnouncementRepository,
	notifications repository.NotificationRepository,
	users repository.UserRepository,
	audits repository.AuditRepository,
) domainUC.NotificationUseCase {
	return &notificationUseCase{
		logger: logger, tx: tx,
		announcements: announcements, notifications: notifications,
		users: users, audits: audits,
	}
}

func (u *notificationUseCase) CreateAnnouncement(ctx context.Context, in domainUC.AnnouncementInput) (*model.Announcement, error) {
	callerID := xhttp.UserID(ctx)
	now := time.Now().UTC()

	a := &model.Announcement{
		Title:    in.Title,
		Body:     in.Body,
		Priority: model.AnnouncementPriority(in.Priority),
	}
	if in.PublishedAt != nil {
		t, err := time.Parse(time.RFC3339, *in.PublishedAt)
		if err != nil {
			return nil, xhttp.BadRequestErrorf("publishedAt phải có dạng RFC3339")
		}
		t = t.UTC()
		a.PublishedAt = &t
	}
	if in.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *in.ExpiresAt)
		if err != nil {
			return nil, xhttp.BadRequestErrorf("expiresAt phải có dạng RFC3339")
		}
		t = t.UTC()
		a.ExpiresAt = &t
	}
	a.AuditFields.CreatedAt = now
	a.AuditFields.UpdatedAt = now
	a.AuditFields.CreatedBy = callerID
	a.AuditFields.UpdatedBy = callerID

	if err := u.announcements.Create(ctx, a); err != nil {
		u.logger.Error("notifUC.CreateAnnouncement", xlogger.Err(err))
		return nil, xhttp.InternalErrorf("lỗi tạo thông báo")
	}
	_ = u.audits.Write(ctx, repository.AuditEntry{
		ActorUserID: callerID, Action: consts.AuditAnnouncementCreated,
		EntityType: consts.TableAnnouncements, EntityID: a.ID,
	})
	return a, nil
}

func (u *notificationUseCase) UpdateAnnouncement(ctx context.Context, id int64, in domainUC.AnnouncementInput) (*model.Announcement, error) {
	callerID := xhttp.UserID(ctx)
	now := time.Now().UTC()

	a, err := u.announcements.GetByID(ctx, id)
	if err != nil {
		return nil, xhttp.InternalErrorf("lỗi lấy thông báo")
	}
	if a == nil {
		return nil, xhttp.NotFoundErrorf("thông báo không tồn tại")
	}

	a.Title = in.Title
	a.Body = in.Body
	a.Priority = model.AnnouncementPriority(in.Priority)
	a.PublishedAt = nil
	a.ExpiresAt = nil

	if in.PublishedAt != nil {
		t, err := time.Parse(time.RFC3339, *in.PublishedAt)
		if err != nil {
			return nil, xhttp.BadRequestErrorf("publishedAt phải có dạng RFC3339")
		}
		t = t.UTC()
		a.PublishedAt = &t
	}
	if in.ExpiresAt != nil {
		t, err := time.Parse(time.RFC3339, *in.ExpiresAt)
		if err != nil {
			return nil, xhttp.BadRequestErrorf("expiresAt phải có dạng RFC3339")
		}
		t = t.UTC()
		a.ExpiresAt = &t
	}
	a.AuditFields.UpdatedAt = now
	a.AuditFields.UpdatedBy = callerID

	if err := u.announcements.Update(ctx, a); err != nil {
		u.logger.Error("notifUC.UpdateAnnouncement", xlogger.Err(err))
		return nil, xhttp.InternalErrorf("lỗi cập nhật thông báo")
	}
	_ = u.audits.Write(ctx, repository.AuditEntry{
		ActorUserID: callerID, Action: consts.AuditAnnouncementUpdated,
		EntityType: consts.TableAnnouncements, EntityID: a.ID,
	})
	return a, nil
}

func (u *notificationUseCase) DeleteAnnouncement(ctx context.Context, id int64) error {
	callerID := xhttp.UserID(ctx)
	if err := u.announcements.SoftDelete(ctx, id, callerID); err != nil {
		u.logger.Error("notifUC.DeleteAnnouncement", xlogger.Err(err))
		return xhttp.InternalErrorf("lỗi xóa thông báo")
	}
	_ = u.audits.Write(ctx, repository.AuditEntry{
		ActorUserID: callerID, Action: consts.AuditAnnouncementDeleted,
		EntityType: consts.TableAnnouncements, EntityID: id,
	})
	return nil
}

func (u *notificationUseCase) GetAnnouncement(ctx context.Context, id int64) (*model.Announcement, error) {
	a, err := u.announcements.GetByID(ctx, id)
	if err != nil {
		return nil, xhttp.InternalErrorf("lỗi lấy thông báo")
	}
	if a == nil {
		return nil, xhttp.NotFoundErrorf("thông báo không tồn tại")
	}
	return a, nil
}

func (u *notificationUseCase) ListAnnouncements(ctx context.Context, includeUnpublished bool, page model.Page) ([]model.Announcement, int64, error) {
	items, total, err := u.announcements.List(ctx, includeUnpublished, page)
	if err != nil {
		u.logger.Error("notifUC.ListAnnouncements", xlogger.Err(err))
		return nil, 0, xhttp.InternalErrorf("lỗi lấy danh sách thông báo")
	}
	return items, total, nil
}

func (u *notificationUseCase) ListForUser(ctx context.Context, userID int64, unreadOnly bool, page model.Page) ([]model.Notification, int64, error) {
	items, total, err := u.notifications.ListForUser(ctx, userID, unreadOnly, page)
	if err != nil {
		u.logger.Error("notifUC.ListForUser", xlogger.Err(err))
		return nil, 0, xhttp.InternalErrorf("lỗi lấy thông báo người dùng")
	}
	return items, total, nil
}

func (u *notificationUseCase) ListForUserFiltered(ctx context.Context, userID int64, in domainUC.NotificationListFilter, page model.Page) ([]model.Notification, int64, error) {
	f := model.NotificationFilter{
		UnreadOnly:    in.UnreadOnly,
		ImportantOnly: in.ImportantOnly,
		Category:      in.Category,
	}
	if in.DateFrom != nil {
		t, err := time.Parse(time.RFC3339, *in.DateFrom)
		if err != nil {
			return nil, 0, xhttp.BadRequestErrorf("dateFrom phải có dạng RFC3339")
		}
		f.DateFrom = &t
	}
	if in.DateTo != nil {
		t, err := time.Parse(time.RFC3339, *in.DateTo)
		if err != nil {
			return nil, 0, xhttp.BadRequestErrorf("dateTo phải có dạng RFC3339")
		}
		f.DateTo = &t
	}
	items, total, err := u.notifications.ListForUserFiltered(ctx, userID, f, page)
	if err != nil {
		u.logger.Error("notifUC.ListForUserFiltered", xlogger.Err(err))
		return nil, 0, xhttp.InternalErrorf("lỗi lấy thông báo người dùng")
	}
	return items, total, nil
}

func (u *notificationUseCase) GetForUser(ctx context.Context, id int64, userID int64) (*model.Notification, error) {
	n, err := u.notifications.GetByID(ctx, id, userID)
	if err != nil {
		u.logger.Error("notifUC.GetForUser", xlogger.Err(err))
		return nil, xhttp.InternalErrorf("lỗi lấy thông báo")
	}
	if n == nil {
		return nil, xhttp.NotFoundErrorf("thông báo không tồn tại")
	}
	return n, nil
}

func (u *notificationUseCase) MarkRead(ctx context.Context, id int64, userID int64) error {
	if err := u.notifications.MarkRead(ctx, id, userID); err != nil {
		u.logger.Error("notifUC.MarkRead", xlogger.Err(err))
		return xhttp.InternalErrorf("lỗi đánh dấu đã đọc")
	}
	return nil
}

func (u *notificationUseCase) MarkUnread(ctx context.Context, id int64, userID int64) error {
	if err := u.notifications.MarkUnread(ctx, id, userID); err != nil {
		u.logger.Error("notifUC.MarkUnread", xlogger.Err(err))
		return xhttp.InternalErrorf("lỗi đánh dấu chưa đọc")
	}
	return nil
}

func (u *notificationUseCase) SetImportant(ctx context.Context, id int64, userID int64, important bool) error {
	if err := u.notifications.SetImportant(ctx, id, userID, important); err != nil {
		u.logger.Error("notifUC.SetImportant", xlogger.Err(err))
		return xhttp.InternalErrorf("lỗi cập nhật thông báo quan trọng")
	}
	return nil
}

func (u *notificationUseCase) Emit(ctx context.Context, in domainUC.NotificationInput) error {
	if in.UserID <= 0 {
		return nil
	}
	n := &model.Notification{
		UserID: in.UserID, Type: in.Type, Category: in.Category,
		Title: in.Title, Body: in.Body, Link: in.Link,
		RefType: in.RefType, RefID: in.RefID,
		IsImportant: in.IsImportant,
	}
	if err := u.notifications.Create(ctx, n); err != nil {
		u.logger.Warn("notifUC.Emit", xlogger.Err(err), xlogger.Int64("userID", in.UserID))
		return err
	}
	return nil
}

func (u *notificationUseCase) EmitToUsers(ctx context.Context, userIDs []int64, in domainUC.NotificationInput) error {
	if len(userIDs) == 0 {
		return nil
	}
	ns := make([]model.Notification, 0, len(userIDs))
	for _, uid := range userIDs {
		if uid <= 0 {
			continue
		}
		ns = append(ns, model.Notification{
			UserID: uid, Type: in.Type, Category: in.Category,
			Title: in.Title, Body: in.Body, Link: in.Link,
			RefType: in.RefType, RefID: in.RefID,
			IsImportant: in.IsImportant,
		})
	}
	if err := u.notifications.BulkCreate(ctx, ns); err != nil {
		u.logger.Warn("notifUC.EmitToUsers", xlogger.Err(err))
		return err
	}
	return nil
}

func (u *notificationUseCase) MarkAllRead(ctx context.Context, userID int64) error {
	if err := u.notifications.MarkAllRead(ctx, userID); err != nil {
		u.logger.Error("notifUC.MarkAllRead", xlogger.Err(err))
		return xhttp.InternalErrorf("lỗi đánh dấu tất cả đã đọc")
	}
	return nil
}

func (u *notificationUseCase) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	count, err := u.notifications.UnreadCount(ctx, userID)
	if err != nil {
		u.logger.Error("notifUC.UnreadCount", xlogger.Err(err))
		return 0, xhttp.InternalErrorf("lỗi đếm thông báo chưa đọc")
	}
	return count, nil
}

func (u *notificationUseCase) PublishToAll(ctx context.Context, title, body, refType string, refID int64) error {
	// Load all active user IDs.
	users, _, err := u.users.List(ctx, repository.UserListFilter{Status: "active", Page: model.NewPage(1, 10000)})
	if err != nil {
		u.logger.Error("notifUC.PublishToAll loadUsers", xlogger.Err(err))
		return xhttp.InternalErrorf("lỗi gửi thông báo")
	}
	ns := make([]model.Notification, len(users))
	for i, usr := range users {
		ns[i] = model.Notification{
			UserID: usr.ID, Type: "announcement",
			Category: model.NotificationCategoryAnnouncement,
			Title:    title, Body: body,
			RefType: &refType, RefID: &refID,
		}
	}
	if err := u.notifications.BulkCreate(ctx, ns); err != nil {
		u.logger.Error("notifUC.PublishToAll BulkCreate", xlogger.Err(err))
		return xhttp.InternalErrorf("lỗi gửi thông báo")
	}
	return nil
}
