// Package model holds pure domain entities (no GORM/JSON tags). State-transition
// logic lives as methods on the aggregates. Repositories map model ↔ row structs.
package model

import "time"

// AuditFields are embedded in every mutable aggregate.
type AuditFields struct {
	CreatedAt time.Time // UTC
	UpdatedAt time.Time // UTC
	CreatedBy int64
	UpdatedBy int64
	DeletedAt *time.Time // nil = not deleted (soft delete)
}

// IsDeleted reports whether the entity is soft-deleted.
func (a AuditFields) IsDeleted() bool { return a.DeletedAt != nil }

// Page is the parsed pagination input shared by list use cases.
type Page struct {
	Number int // 1-based
	Size   int
}

// Offset returns the SQL offset.
func (p Page) Offset() int {
	if p.Number < 1 {
		return 0
	}
	return (p.Number - 1) * p.Size
}

// NewPage normalizes raw page/limit values (defaults: page 1, size 20, max 1000).
func NewPage(page, limit int) Page {
	if page < 1 {
		page = 1
	}
	switch {
	case limit < 1:
		limit = 20
	case limit > 1000:
		limit = 1000
	}
	return Page{Number: page, Size: limit}
}

// DateRange is an inclusive [From, To] interval in UTC.
type DateRange struct {
	From time.Time
	To   time.Time
}

// ── Common response models ────────────────────────────────────────────────────

// OkResponse is the standard {"ok": true} response.
type OkResponse struct {
	Ok bool `json:"ok"`
}

// DeletedResponse confirms a soft-delete by ID.
type DeletedResponse struct {
	ID      int64 `json:"id"`
	Deleted bool  `json:"deleted"`
}

// AddedResponse reports how many items were added.
type AddedResponse struct {
	Added int64 `json:"added"`
}

// BulkActionResponse reports the result of a bulk approve/reject operation.
type BulkActionResponse struct {
	Succeeded int      `json:"succeeded"`
	Failed    int      `json:"failed"`
	Errors    []string `json:"errors"`
}
