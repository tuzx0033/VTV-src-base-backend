package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	domainRepo "vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xpostgres"
)

// userPermissionRow is the DB shape. `permissions` is stored as JSONB; we keep
// the raw JSON text and (un)marshal at the boundary so we never leak GORM-typed
// driver values into the domain.
type userPermissionRow struct {
	ID              int64     `gorm:"column:id;primaryKey"`
	UserID          int64     `gorm:"column:user_id"`
	PermissionCount int       `gorm:"column:permission_count"`
	Permissions     string    `gorm:"column:permissions;type:jsonb"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
	CreatedBy       int64     `gorm:"column:created_by"`
	UpdatedBy       int64     `gorm:"column:updated_by"`
}

func (userPermissionRow) TableName() string { return consts.TableUserPermissions }

func (r userPermissionRow) toDomain() *model.UserPermission {
	out := &model.UserPermission{
		ID: r.ID, UserID: r.UserID, PermissionCount: r.PermissionCount,
		CreatedAt: r.CreatedAt.UTC(), UpdatedAt: r.UpdatedAt.UTC(),
		CreatedBy: r.CreatedBy, UpdatedBy: r.UpdatedBy,
	}
	if r.Permissions != "" {
		var m model.PermissionMap
		_ = json.Unmarshal([]byte(r.Permissions), &m)
		out.Permissions = m
	}
	if out.Permissions == nil {
		out.Permissions = model.PermissionMap{}
	}
	return out
}

type permissionRepository struct {
	logger *xlogger.Logger
	db     *gorm.DB
}

// NewPermissionRepository builds the Postgres permission repository.
func NewPermissionRepository(logger *xlogger.Logger, db *gorm.DB) domainRepo.PermissionRepository {
	return &permissionRepository{logger: logger, db: db}
}

func (r *permissionRepository) q(ctx context.Context) *gorm.DB { return xpostgres.DB(ctx, r.db) }

func (r *permissionRepository) GetByID(ctx context.Context, id int64) (*model.UserPermission, error) {
	var row userPermissionRow
	err := r.q(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *permissionRepository) GetByUserID(ctx context.Context, userID int64) (*model.UserPermission, error) {
	var row userPermissionRow
	err := r.q(ctx).Where("user_id = ?", userID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

// joinedRow includes denormalised user fields for List enrichment.
type joinedPermissionRow struct {
	userPermissionRow
	UserFullName  string  `gorm:"column:user_full_name"`
	UserUsername  string  `gorm:"column:user_username"`
	UserStaffCode *string `gorm:"column:user_staff_code"`
}

func (r *permissionRepository) List(ctx context.Context, f domainRepo.UserPermissionListFilter) ([]model.UserPermission, int64, error) {
	base := r.q(ctx).
		Table(consts.TableUserPermissions + " AS up").
		Joins("LEFT JOIN " + consts.TableUsers + " u ON u.id = up.user_id")
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + s + "%"
		base = base.Where("u.username ILIKE ? OR u.full_name ILIKE ? OR u.staff_code ILIKE ?", like, like, like)
	}
	if f.UserID != nil && *f.UserID > 0 {
		base = base.Where("up.user_id = ?", *f.UserID)
	}
	var total int64
	if err := base.Session(&gorm.Session{}).Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []joinedPermissionRow
	err := base.
		Select(`up.id, up.user_id, up.permission_count, up.permissions, up.created_at, up.updated_at,
			up.created_by, up.updated_by,
			u.full_name AS user_full_name, u.username AS user_username, u.staff_code AS user_staff_code`).
		Order("up.created_at DESC").
		Limit(f.Page.Size).Offset(f.Page.Offset()).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}
	out := make([]model.UserPermission, 0, len(rows))
	for _, row := range rows {
		d := row.userPermissionRow.toDomain()
		d.UserFullName = row.UserFullName
		d.UserUsername = row.UserUsername
		d.UserStaffCode = row.UserStaffCode
		out = append(out, *d)
	}
	return out, total, nil
}

func (r *permissionRepository) Create(ctx context.Context, up *model.UserPermission) error {
	if up == nil {
		return errors.New("user permission cannot be nil")
	}
	now := time.Now().UTC()
	if up.CreatedAt.IsZero() {
		up.CreatedAt = now
	}
	up.UpdatedAt = now
	if up.Permissions == nil {
		up.Permissions = model.PermissionMap{}
	}
	b, err := json.Marshal(up.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}
	row := userPermissionRow{
		UserID: up.UserID, PermissionCount: up.PermissionCount,
		Permissions: string(b),
		CreatedAt:   up.CreatedAt, UpdatedAt: up.UpdatedAt,
		CreatedBy: up.CreatedBy, UpdatedBy: up.UpdatedBy,
	}
	if err := r.q(ctx).Omit("id").Create(&row).Error; err != nil {
		return err
	}
	up.ID = row.ID
	return nil
}

func (r *permissionRepository) Update(ctx context.Context, up *model.UserPermission) error {
	if up == nil {
		return errors.New("user permission cannot be nil")
	}
	up.UpdatedAt = time.Now().UTC()
	if up.Permissions == nil {
		up.Permissions = model.PermissionMap{}
	}
	b, err := json.Marshal(up.Permissions)
	if err != nil {
		return fmt.Errorf("marshal permissions: %w", err)
	}
	return r.q(ctx).Model(&userPermissionRow{}).Where("id = ?", up.ID).Updates(map[string]any{
		"permission_count": up.PermissionCount,
		"permissions":      string(b),
		"updated_at":       up.UpdatedAt,
		"updated_by":       up.UpdatedBy,
	}).Error
}

func (r *permissionRepository) Delete(ctx context.Context, id int64) error {
	return r.q(ctx).Where("id = ?", id).Delete(&userPermissionRow{}).Error
}

func (r *permissionRepository) HasPermission(ctx context.Context, userID int64, resource, action string) (bool, error) {
	up, err := r.GetByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	if up == nil {
		return false, nil
	}
	return up.Permissions.HasAction(resource, action), nil
}

func (r *permissionRepository) CountPermissionsAvailable() int {
	count := 0
	var walk func(p consts.Permission)
	walk = func(p consts.Permission) {
		count += len(p.Actions)
		for _, c := range p.Children {
			walk(c)
		}
	}
	for _, p := range consts.PermissionsDefinitions {
		walk(p)
	}
	return count
}
