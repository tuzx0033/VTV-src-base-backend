package model

import "time"

// AnnouncementPriority is the importance level of a system announcement.
type AnnouncementPriority string

const (
	AnnouncementPriorityLow    AnnouncementPriority = "low"
	AnnouncementPriorityNormal AnnouncementPriority = "normal"
	AnnouncementPriorityHigh   AnnouncementPriority = "high"
)

// Announcement is a broadcast message from admin/manager to all users.
type Announcement struct {
	ID          int64
	Title       string
	Body        string
	Priority    AnnouncementPriority
	PublishedAt *time.Time
	ExpiresAt   *time.Time
	AuditFields
}

// IsPublished reports whether the announcement is visible to end users.
func (a *Announcement) IsPublished() bool {
	if a.PublishedAt == nil {
		return false
	}
	now := time.Now().UTC()
	if a.PublishedAt.After(now) {
		return false
	}
	if a.ExpiresAt != nil && a.ExpiresAt.Before(now) {
		return false
	}
	return a.AuditFields.DeletedAt == nil
}

// Notification is a per-user inbox item.
type Notification struct {
	ID          int64
	UserID      int64
	Type        string
	Category    string // 'task' | 'announcement' | 'system'
	Title       string
	Body        string
	Link        *string // Deep link to detail page (e.g. /items/123)
	RefType     *string
	RefID       *int64
	IsRead      bool
	IsImportant bool
	ReadAt      *time.Time
	CreatedAt   time.Time
}

// Notification categories.
const (
	NotificationCategoryTask         = "task"         // Công việc cần thực hiện
	NotificationCategoryAnnouncement = "announcement" // Thông báo hệ thống (bảo trì, ...)
	NotificationCategorySystem       = "system"       // Misc system events
)

// NotificationFilter narrows the list query.
type NotificationFilter struct {
	UnreadOnly    bool
	ImportantOnly bool
	Category      string // empty = all
	DateFrom      *time.Time
	DateTo        *time.Time
}
