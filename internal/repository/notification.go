package repository

import (
	"context"
	"errors"
	"time"

	"gorm.io/gorm"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	domainRepo "vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/pkg/xpostgres"
)

// ── announcements ─────────────────────────────────────────────────────────────

type announcementRow struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	Title       string     `gorm:"column:title"`
	Body        string     `gorm:"column:body"`
	Priority    string     `gorm:"column:priority"`
	PublishedAt *time.Time `gorm:"column:published_at"`
	ExpiresAt   *time.Time `gorm:"column:expires_at"`
	DeletedAt   *time.Time `gorm:"column:deleted_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
	UpdatedAt   time.Time  `gorm:"column:updated_at"`
	CreatedBy   int64      `gorm:"column:created_by"`
	UpdatedBy   int64      `gorm:"column:updated_by"`
}

func (announcementRow) TableName() string { return consts.TableAnnouncements }

func (r announcementRow) toDomain() model.Announcement {
	a := model.Announcement{
		ID: r.ID, Title: r.Title, Body: r.Body,
		Priority:    model.AnnouncementPriority(r.Priority),
		PublishedAt: r.PublishedAt, ExpiresAt: r.ExpiresAt,
	}
	a.AuditFields.CreatedAt = r.CreatedAt.UTC()
	a.AuditFields.UpdatedAt = r.UpdatedAt.UTC()
	a.AuditFields.CreatedBy = r.CreatedBy
	a.AuditFields.UpdatedBy = r.UpdatedBy
	a.AuditFields.DeletedAt = r.DeletedAt
	return a
}

type announcementRepository struct {
	db *gorm.DB
}

// NewAnnouncementRepository builds the announcement repository.
func NewAnnouncementRepository(db *gorm.DB) domainRepo.AnnouncementRepository {
	return &announcementRepository{db: db}
}

func (r *announcementRepository) q(ctx context.Context) *gorm.DB { return xpostgres.DB(ctx, r.db) }

func (r *announcementRepository) Create(ctx context.Context, a *model.Announcement) error {
	row := announcementRow{
		Title: a.Title, Body: a.Body, Priority: string(a.Priority),
		PublishedAt: a.PublishedAt, ExpiresAt: a.ExpiresAt,
		CreatedAt: a.AuditFields.CreatedAt, UpdatedAt: a.AuditFields.UpdatedAt,
		CreatedBy: a.AuditFields.CreatedBy, UpdatedBy: a.AuditFields.UpdatedBy,
	}
	if err := r.q(ctx).Omit("id").Create(&row).Error; err != nil {
		return err
	}
	a.ID = row.ID
	return nil
}

func (r *announcementRepository) Update(ctx context.Context, a *model.Announcement) error {
	row := announcementRow{
		ID: a.ID, Title: a.Title, Body: a.Body, Priority: string(a.Priority),
		PublishedAt: a.PublishedAt, ExpiresAt: a.ExpiresAt,
		DeletedAt: a.AuditFields.DeletedAt,
		CreatedAt: a.AuditFields.CreatedAt, UpdatedAt: a.AuditFields.UpdatedAt,
		CreatedBy: a.AuditFields.CreatedBy, UpdatedBy: a.AuditFields.UpdatedBy,
	}
	return r.q(ctx).Save(&row).Error
}

func (r *announcementRepository) GetByID(ctx context.Context, id int64) (*model.Announcement, error) {
	var row announcementRow
	err := r.q(ctx).Where("id = ? AND deleted_at IS NULL", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	a := row.toDomain()
	return &a, nil
}

func (r *announcementRepository) List(ctx context.Context, includeUnpublished bool, page model.Page) ([]model.Announcement, int64, error) {
	where := "deleted_at IS NULL"
	if !includeUnpublished {
		where += " AND published_at IS NOT NULL AND published_at <= NOW()" +
			" AND (expires_at IS NULL OR expires_at > NOW())"
	}
	var total int64
	if err := r.q(ctx).Table(consts.TableAnnouncements).Where(where).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []announcementRow
	if err := r.q(ctx).Where(where).Order("published_at DESC NULLS LAST, created_at DESC").
		Limit(page.Size).Offset(page.Offset()).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]model.Announcement, len(rows))
	for i, row := range rows {
		out[i] = row.toDomain()
	}
	return out, total, nil
}

func (r *announcementRepository) SoftDelete(ctx context.Context, id int64, byUserID int64) error {
	now := time.Now().UTC()
	return r.q(ctx).Table(consts.TableAnnouncements).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]interface{}{"deleted_at": now, "updated_by": byUserID, "updated_at": now}).Error
}

// ── notifications ─────────────────────────────────────────────────────────────

type notificationRow struct {
	ID          int64      `gorm:"column:id;primaryKey"`
	UserID      int64      `gorm:"column:user_id"`
	Type        string     `gorm:"column:type"`
	Category    string     `gorm:"column:category"`
	Title       string     `gorm:"column:title"`
	Body        string     `gorm:"column:body"`
	Link        *string    `gorm:"column:link"`
	RefType     *string    `gorm:"column:ref_type"`
	RefID       *int64     `gorm:"column:ref_id"`
	IsRead      bool       `gorm:"column:is_read"`
	IsImportant bool       `gorm:"column:is_important"`
	ReadAt      *time.Time `gorm:"column:read_at"`
	CreatedAt   time.Time  `gorm:"column:created_at"`
}

func (notificationRow) TableName() string { return consts.TableNotifications }

func (r notificationRow) toDomain() model.Notification {
	return model.Notification{
		ID: r.ID, UserID: r.UserID, Type: r.Type, Category: r.Category,
		Title: r.Title, Body: r.Body, Link: r.Link,
		RefType: r.RefType, RefID: r.RefID,
		IsRead: r.IsRead, IsImportant: r.IsImportant, ReadAt: r.ReadAt,
		CreatedAt: r.CreatedAt.UTC(),
	}
}

type notificationRepository struct {
	db *gorm.DB
}

// NewNotificationRepository builds the notification repository.
func NewNotificationRepository(db *gorm.DB) domainRepo.NotificationRepository {
	return &notificationRepository{db: db}
}

func (r *notificationRepository) q(ctx context.Context) *gorm.DB { return xpostgres.DB(ctx, r.db) }

func (r *notificationRepository) Create(ctx context.Context, n *model.Notification) error {
	row := notificationRow{
		UserID: n.UserID, Type: n.Type, Category: defaultCategory(n.Category),
		Title: n.Title, Body: n.Body, Link: n.Link,
		RefType: n.RefType, RefID: n.RefID,
		IsRead: n.IsRead, IsImportant: n.IsImportant,
	}
	if err := r.q(ctx).Omit("id").Create(&row).Error; err != nil {
		return err
	}
	n.ID = row.ID
	n.CreatedAt = row.CreatedAt
	n.Category = row.Category
	return nil
}

func (r *notificationRepository) BulkCreate(ctx context.Context, ns []model.Notification) error {
	if len(ns) == 0 {
		return nil
	}
	rows := make([]notificationRow, len(ns))
	for i, n := range ns {
		rows[i] = notificationRow{
			UserID: n.UserID, Type: n.Type, Category: defaultCategory(n.Category),
			Title: n.Title, Body: n.Body, Link: n.Link,
			RefType: n.RefType, RefID: n.RefID,
			IsImportant: n.IsImportant,
		}
	}
	if err := r.q(ctx).Omit("id").Create(&rows).Error; err != nil {
		return err
	}
	for i := range rows {
		ns[i].ID = rows[i].ID
		ns[i].CreatedAt = rows[i].CreatedAt
		ns[i].Category = rows[i].Category
	}
	return nil
}

func defaultCategory(c string) string {
	if c == "" {
		return model.NotificationCategoryTask
	}
	return c
}

func (r *notificationRepository) ListForUser(ctx context.Context, userID int64, unreadOnly bool, page model.Page) ([]model.Notification, int64, error) {
	return r.ListForUserFiltered(ctx, userID, model.NotificationFilter{UnreadOnly: unreadOnly}, page)
}

func (r *notificationRepository) ListForUserFiltered(ctx context.Context, userID int64, f model.NotificationFilter, page model.Page) ([]model.Notification, int64, error) {
	where := "user_id = ?"
	args := []interface{}{userID}
	if f.UnreadOnly {
		where += " AND is_read = FALSE"
	}
	if f.ImportantOnly {
		where += " AND is_important = TRUE"
	}
	if f.Category != "" {
		where += " AND category = ?"
		args = append(args, f.Category)
	}
	if f.DateFrom != nil {
		where += " AND created_at >= ?"
		args = append(args, *f.DateFrom)
	}
	if f.DateTo != nil {
		where += " AND created_at <= ?"
		args = append(args, *f.DateTo)
	}
	var total int64
	if err := r.q(ctx).Table(consts.TableNotifications).Where(where, args...).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []notificationRow
	if err := r.q(ctx).Where(where, args...).Order("created_at DESC").
		Limit(page.Size).Offset(page.Offset()).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]model.Notification, len(rows))
	for i, row := range rows {
		out[i] = row.toDomain()
	}
	return out, total, nil
}

func (r *notificationRepository) GetByID(ctx context.Context, id int64, userID int64) (*model.Notification, error) {
	var row notificationRow
	err := r.q(ctx).Where("id = ? AND user_id = ?", id, userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	n := row.toDomain()
	return &n, nil
}

func (r *notificationRepository) MarkRead(ctx context.Context, id int64, userID int64) error {
	now := time.Now().UTC()
	return r.q(ctx).Table(consts.TableNotifications).
		Where("id = ? AND user_id = ? AND is_read = FALSE", id, userID).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *notificationRepository) MarkUnread(ctx context.Context, id int64, userID int64) error {
	return r.q(ctx).Table(consts.TableNotifications).
		Where("id = ? AND user_id = ?", id, userID).
		Updates(map[string]interface{}{"is_read": false, "read_at": nil}).Error
}

func (r *notificationRepository) SetImportant(ctx context.Context, id int64, userID int64, important bool) error {
	return r.q(ctx).Table(consts.TableNotifications).
		Where("id = ? AND user_id = ?", id, userID).
		Update("is_important", important).Error
}

func (r *notificationRepository) MarkAllRead(ctx context.Context, userID int64) error {
	now := time.Now().UTC()
	return r.q(ctx).Table(consts.TableNotifications).
		Where("user_id = ? AND is_read = FALSE", userID).
		Updates(map[string]interface{}{"is_read": true, "read_at": now}).Error
}

func (r *notificationRepository) UnreadCount(ctx context.Context, userID int64) (int64, error) {
	var count int64
	if err := r.q(ctx).Table(consts.TableNotifications).
		Where("user_id = ? AND is_read = FALSE", userID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}
