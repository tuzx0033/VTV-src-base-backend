// Package repository holds the Postgres (GORM) implementations of the domain
// repository interfaces. Row structs (with GORM tags) never leave this package.
package repository

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xpostgres"
)

type userRow struct {
	ID                  int64      `gorm:"column:id;primaryKey"`
	Username            string     `gorm:"column:username"`
	PasswordHash        string     `gorm:"column:password_hash"`
	FullName            string     `gorm:"column:full_name"`
	Email               *string    `gorm:"column:email"`
	Phone               *string    `gorm:"column:phone"`
	Role                string     `gorm:"column:role"`
	Status              string     `gorm:"column:status"`
	StaffCode           *string    `gorm:"column:staff_code"`
	AvatarURL           *string    `gorm:"column:avatar_url"`
	ForcePasswordChange bool       `gorm:"column:force_password_change"`
	EmploymentType      string     `gorm:"column:employment_type"`
	CreatedAt           time.Time  `gorm:"column:created_at"`
	UpdatedAt           time.Time  `gorm:"column:updated_at"`
	CreatedBy           int64      `gorm:"column:created_by"`
	UpdatedBy           int64      `gorm:"column:updated_by"`
	DeletedAt           *time.Time `gorm:"column:deleted_at"`
}

func (userRow) TableName() string { return consts.TableUsers }

// employmentTypeOrDefault coalesces an empty/invalid employment type to fulltime
// so writes never violate the users.employment_type CHECK constraint.
func employmentTypeOrDefault(e model.EmploymentType) model.EmploymentType {
	if e.Valid() {
		return e
	}
	return model.EmploymentFulltime
}

func (r userRow) toDomain() *model.User {
	u := &model.User{
		ID: r.ID, Username: r.Username, PasswordHash: r.PasswordHash, FullName: r.FullName,
		Email: r.Email, Phone: r.Phone, Role: model.Role(r.Role), Status: model.UserStatus(r.Status),
		StaffCode: r.StaffCode, AvatarURL: r.AvatarURL,
		ForcePasswordChange: r.ForcePasswordChange,
		EmploymentType:      model.EmploymentType(r.EmploymentType),
	}
	u.CreatedAt = r.CreatedAt.UTC()
	u.UpdatedAt = r.UpdatedAt.UTC()
	u.CreatedBy = r.CreatedBy
	u.UpdatedBy = r.UpdatedBy
	if r.DeletedAt != nil {
		t := r.DeletedAt.UTC()
		u.DeletedAt = &t
	}
	return u
}

type userRepository struct {
	logger *xlogger.Logger
	db     *gorm.DB
}

// NewUserRepository builds the Postgres user repository.
func NewUserRepository(logger *xlogger.Logger, db *gorm.DB) repository.UserRepository {
	return &userRepository{logger: logger, db: db}
}

func (r *userRepository) q(ctx context.Context) *gorm.DB { return xpostgres.DB(ctx, r.db) }

func (r *userRepository) GetByID(ctx context.Context, id int64) (*model.User, error) {
	var row userRow
	err := r.q(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *userRepository) GetByUsername(ctx context.Context, username string) (*model.User, error) {
	var row userRow
	err := r.q(ctx).Where("lower(username) = lower(?)", strings.TrimSpace(username)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}

func (r *userRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	var n int64
	err := r.q(ctx).Model(&userRow{}).Where("lower(username) = lower(?)", strings.TrimSpace(username)).Count(&n).Error
	return n > 0, err
}

func (r *userRepository) List(ctx context.Context, f repository.UserListFilter) ([]model.User, int64, error) {
	q := r.q(ctx).Model(&userRow{}).Where("deleted_at IS NULL")
	if s := strings.TrimSpace(f.Search); s != "" {
		like := "%" + s + "%"
		q = q.Where("username ILIKE ? OR full_name ILIKE ? OR email ILIKE ?", like, like, like)
	}
	if len(f.Roles) > 0 {
		q = q.Where("role IN ?", f.Roles)
	} else if f.Role != "" {
		q = q.Where("role = ?", f.Role)
	}
	if len(f.Statuses) > 0 {
		q = q.Where("status IN ?", f.Statuses)
	} else if f.Status != "" {
		q = q.Where("status = ?", f.Status)
	}
	var total int64
	if err := q.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	var rows []userRow
	if err := q.Order("created_at ASC").Limit(f.Page.Size).Offset(f.Page.Offset()).Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	out := make([]model.User, 0, len(rows))
	for i := range rows {
		out = append(out, *rows[i].toDomain())
	}
	return out, total, nil
}

func (r *userRepository) Create(ctx context.Context, u *model.User) error {
	now := time.Now().UTC()
	if u.CreatedAt.IsZero() {
		u.CreatedAt = now
	}
	u.UpdatedAt = now
	row := userRow{
		Username: u.Username, PasswordHash: u.PasswordHash, FullName: u.FullName,
		Email: u.Email, Phone: u.Phone, Role: string(u.Role), Status: string(u.Status),
		StaffCode: u.StaffCode, AvatarURL: u.AvatarURL,
		ForcePasswordChange: u.ForcePasswordChange,
		EmploymentType:      string(employmentTypeOrDefault(u.EmploymentType)),
		CreatedAt:           u.CreatedAt, UpdatedAt: u.UpdatedAt, CreatedBy: u.CreatedBy, UpdatedBy: u.UpdatedBy,
	}
	// id is GENERATED ALWAYS AS IDENTITY — omit it; GORM will RETURNING it.
	if err := r.q(ctx).Omit("id").Create(&row).Error; err != nil {
		return err
	}
	u.ID = row.ID
	return nil
}

func (r *userRepository) Update(ctx context.Context, u *model.User) error {
	u.UpdatedAt = time.Now().UTC()
	return r.q(ctx).Model(&userRow{}).Where("id = ?", u.ID).Updates(map[string]any{
		"username":              u.Username,
		"password_hash":         u.PasswordHash,
		"full_name":             u.FullName,
		"email":                 u.Email,
		"phone":                 u.Phone,
		"role":                  string(u.Role),
		"status":                string(u.Status),
		"staff_code":            u.StaffCode,
		"avatar_url":            u.AvatarURL,
		"force_password_change": u.ForcePasswordChange,
		"employment_type":       string(employmentTypeOrDefault(u.EmploymentType)),
		"updated_at":            u.UpdatedAt,
		"updated_by":            u.UpdatedBy,
		"deleted_at":            u.DeletedAt,
	}).Error
}

func (r *userRepository) MustForcePasswordChange(ctx context.Context, id int64) (bool, error) {
	var flag bool
	err := r.q(ctx).Model(&userRow{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Pluck("force_password_change", &flag).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return flag, err
}

func (r *userRepository) SoftDelete(ctx context.Context, id int64, actorID int64) error {
	now := time.Now().UTC()
	return r.q(ctx).Model(&userRow{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Updates(map[string]any{
			"deleted_at": now,
			"updated_at": now,
			"updated_by": actorID,
		}).Error
}

func (r *userRepository) StaffCodesWithPrefix(ctx context.Context, prefix string) ([]string, error) {
	var codes []string
	err := r.q(ctx).Model(&userRow{}).
		Where("staff_code IS NOT NULL AND staff_code ILIKE ?", prefix+"%").
		Pluck("staff_code", &codes).Error
	return codes, err
}

func (r *userRepository) SetResetToken(ctx context.Context, userID int64, tokenHash string, expiresAt *time.Time) error {
	updates := map[string]any{
		"updated_at": time.Now().UTC(),
	}
	if tokenHash == "" {
		updates["password_reset_token_hash"] = nil
		updates["password_reset_expires_at"] = nil
	} else {
		updates["password_reset_token_hash"] = tokenHash
		updates["password_reset_expires_at"] = expiresAt
	}
	return r.q(ctx).Model(&userRow{}).Where("id = ?", userID).Updates(updates).Error
}

func (r *userRepository) FindByResetTokenHash(ctx context.Context, tokenHash string) (*model.User, error) {
	if tokenHash == "" {
		return nil, nil
	}
	var row userRow
	err := r.q(ctx).Where(
		"password_reset_token_hash = ? AND password_reset_expires_at IS NOT NULL AND password_reset_expires_at > now() AND deleted_at IS NULL",
		tokenHash,
	).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return row.toDomain(), nil
}
