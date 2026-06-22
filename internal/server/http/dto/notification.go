package dto

import (
	"vtv.vn/backend/internal/domain/model"
)

// ── requests ──────────────────────────────────────────────────────────────────

type AnnouncementRequest struct {
	Title       string  `json:"title"      validate:"required"`
	Body        string  `json:"body"       validate:"required"`
	Priority    string  `json:"priority"   validate:"required,oneof=low normal high"`
	PublishedAt *string `json:"publishedAt,omitempty"`
	ExpiresAt   *string `json:"expiresAt,omitempty"`
}

// ── responses ─────────────────────────────────────────────────────────────────

type AnnouncementResponse struct {
	ID          int64   `json:"id"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	Priority    string  `json:"priority"`
	PublishedAt *string `json:"publishedAt,omitempty"`
	ExpiresAt   *string `json:"expiresAt,omitempty"`
	CreatedAt   string  `json:"createdAt"`
	UpdatedAt   string  `json:"updatedAt"`
}

func ToAnnouncementResponse(a model.Announcement) AnnouncementResponse {
	r := AnnouncementResponse{
		ID: a.ID, Title: a.Title, Body: a.Body, Priority: string(a.Priority),
		CreatedAt: a.AuditFields.CreatedAt.Format("2006-01-02T15:04:05Z"),
		UpdatedAt: a.AuditFields.UpdatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if a.PublishedAt != nil {
		s := a.PublishedAt.Format("2006-01-02T15:04:05Z")
		r.PublishedAt = &s
	}
	if a.ExpiresAt != nil {
		s := a.ExpiresAt.Format("2006-01-02T15:04:05Z")
		r.ExpiresAt = &s
	}
	return r
}

type NotificationResponse struct {
	ID          int64   `json:"id"`
	Type        string  `json:"type"`
	Category    string  `json:"category"`
	Title       string  `json:"title"`
	Body        string  `json:"body"`
	Message     string  `json:"message"` // alias of Body for FE compatibility
	Link        *string `json:"link,omitempty"`
	RefType     *string `json:"refType,omitempty"`
	RefID       *int64  `json:"refId,omitempty"`
	IsRead      bool    `json:"isRead"`
	IsImportant bool    `json:"isImportant"`
	ReadAt      *string `json:"readAt,omitempty"`
	CreatedAt   string  `json:"createdAt"`
}

func ToNotificationResponse(n model.Notification) NotificationResponse {
	r := NotificationResponse{
		ID: n.ID, Type: n.Type, Category: n.Category,
		Title: n.Title, Body: n.Body, Message: n.Body, Link: n.Link,
		RefType: n.RefType, RefID: n.RefID,
		IsRead: n.IsRead, IsImportant: n.IsImportant,
		CreatedAt: n.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}
	if n.ReadAt != nil {
		s := n.ReadAt.Format("2006-01-02T15:04:05Z")
		r.ReadAt = &s
	}
	return r
}

func ToNotificationResponses(items []model.Notification) []NotificationResponse {
	out := make([]NotificationResponse, len(items))
	for i, n := range items {
		out[i] = ToNotificationResponse(n)
	}
	return out
}
