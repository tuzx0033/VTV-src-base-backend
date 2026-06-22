package repository

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/gorm"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xpostgres"
)

type auditRow struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	ActorUserID *int64    `gorm:"column:actor_user_id"`
	ActorName   string    `gorm:"column:actor_name"`
	ActorRole   string    `gorm:"column:actor_role"`
	Action      string    `gorm:"column:action"`
	EntityType  string    `gorm:"column:entity_type"`
	EntityID    *int64    `gorm:"column:entity_id"`
	EntityName  string    `gorm:"column:entity_name"`
	Summary     string    `gorm:"column:summary"`
	Changes     *string   `gorm:"column:changes;type:jsonb"` // nil → SQL NULL; non-nil must be valid JSON
	IPAddress   string    `gorm:"column:ip_address"`
	RequestID   string    `gorm:"column:request_id"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (auditRow) TableName() string { return consts.TableAuditLog }

type auditRepository struct{ db *gorm.DB }

// NewAuditRepository builds the Postgres audit-log repository.
func NewAuditRepository(db *gorm.DB) repository.AuditRepository { return &auditRepository{db: db} }

func (r *auditRepository) Write(ctx context.Context, e repository.AuditEntry) error {
	// Enrich from context when the caller didn't set them.
	if e.ActorUserID == 0 {
		e.ActorUserID = xhttp.UserID(ctx)
	}
	if e.ActorRole == "" {
		e.ActorRole = xhttp.Role(ctx)
	}
	if e.RequestID == "" {
		e.RequestID = xhttp.RequestID(ctx)
	}
	if e.IPAddress == "" {
		e.IPAddress = xhttp.ClientIP(ctx)
	}
	createdAt := e.CreatedAt.UTC()
	if e.CreatedAt.IsZero() {
		createdAt = time.Now().UTC()
	}
	row := auditRow{
		ActorName: e.ActorName, ActorRole: e.ActorRole, Action: e.Action,
		EntityType: e.EntityType, EntityName: e.EntityName, Summary: e.Summary,
		IPAddress: e.IPAddress, RequestID: e.RequestID, CreatedAt: createdAt,
	}
	if e.ActorUserID != 0 {
		v := e.ActorUserID
		row.ActorUserID = &v
	}
	if e.EntityID != 0 {
		v := e.EntityID
		row.EntityID = &v
	}
	if len(e.Changes) > 0 {
		if b, err := json.Marshal(e.Changes); err == nil {
			s := string(b)
			row.Changes = &s
		}
	}
	return xpostgres.DB(ctx, r.db).Omit("id").Create(&row).Error
}

func (r *auditRepository) ListLogs(ctx context.Context, filter repository.AuditFilter) ([]repository.AuditLogRow, int64, error) {
	where := "1=1"
	var args []interface{}

	if filter.Search != "" {
		where += " AND (al.action ILIKE ? OR al.entity_name ILIKE ? OR al.summary ILIKE ? OR al.actor_name ILIKE ?)"
		like := "%" + filter.Search + "%"
		args = append(args, like, like, like, like)
	}
	if filter.Action != "" {
		where += " AND al.action ILIKE ?"
		args = append(args, "%"+filter.Action+"%")
	}
	if filter.UserID != nil {
		where += " AND al.actor_user_id = ?"
		args = append(args, *filter.UserID)
	}
	if filter.EntityType != "" {
		where += " AND al.entity_type = ?"
		args = append(args, filter.EntityType)
	}
	if filter.DateFrom != "" {
		where += " AND al.created_at >= ?::date"
		args = append(args, filter.DateFrom)
	}
	if filter.DateTo != "" {
		where += " AND al.created_at < (?::date + 1)"
		args = append(args, filter.DateTo)
	}

	var total int64
	if err := r.db.WithContext(ctx).Raw("SELECT COUNT(*) FROM audit_log al WHERE "+where, args...).Scan(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page <= 0 {
		page = 1
	}
	limit := filter.Limit
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	offset := (page - 1) * limit

	type row struct {
		ID         int64   `gorm:"column:id"`
		UserID     *int64  `gorm:"column:actor_user_id"`
		ActorName  *string `gorm:"column:actor_name"`
		ActorRole  *string `gorm:"column:actor_role"`
		Action     string  `gorm:"column:action"`
		EntityType string  `gorm:"column:entity_type"`
		EntityID   *int64  `gorm:"column:entity_id"`
		EntityName *string `gorm:"column:entity_name"`
		Summary    *string `gorm:"column:summary"`
		Changes    []byte  `gorm:"column:changes"`
		IPAddress  *string `gorm:"column:ip_address"`
		CreatedAt  string  `gorm:"column:created_at"`
	}

	dataSQL := `SELECT al.id, al.actor_user_id,
		COALESCE(NULLIF(al.actor_name, ''), u.full_name) AS actor_name,
		COALESCE(NULLIF(al.actor_role, ''), u.role) AS actor_role,
		al.action, al.entity_type, al.entity_id, al.entity_name, al.summary,
		al.changes, al.ip_address, al.created_at::text AS created_at
		FROM audit_log al LEFT JOIN users u ON u.id = al.actor_user_id
		WHERE ` + where + ` ORDER BY al.id DESC LIMIT ? OFFSET ?`
	dataArgs := append(args, limit, offset)

	var rows []row
	if err := r.db.WithContext(ctx).Raw(dataSQL, dataArgs...).Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]repository.AuditLogRow, len(rows))
	for i, r := range rows {
		out[i] = repository.AuditLogRow{
			ID: r.ID, UserID: r.UserID, UserFullName: r.ActorName,
			UserRole: r.ActorRole, Action: r.Action, EntityType: r.EntityType,
			EntityID: r.EntityID, EntityName: r.EntityName, Description: r.Summary,
			Metadata: r.Changes, IPAddress: r.IPAddress, CreatedAt: r.CreatedAt,
		}
	}
	return out, total, nil
}

func (r *auditRepository) ListActors(ctx context.Context) ([]repository.AuditActor, error) {
	type row struct {
		UserID       int64  `gorm:"column:user_id"`
		UserFullName string `gorm:"column:full_name"`
		UserRole     string `gorm:"column:role"`
	}
	var rows []row
	err := r.db.WithContext(ctx).Raw(`SELECT DISTINCT u.id AS user_id, u.full_name, u.role FROM users u
		JOIN audit_log al ON al.actor_user_id = u.id ORDER BY u.full_name`).Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]repository.AuditActor, len(rows))
	for i, r := range rows {
		out[i] = repository.AuditActor{
			UserID: r.UserID, UserFullName: r.UserFullName, UserRole: r.UserRole,
		}
	}
	return out, nil
}
