package repository

import (
	"context"
	"time"

	"gorm.io/gorm"

	"vtv.vn/backend/internal/domain/model"
	domainRepo "vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/pkg/xpostgres"
)

type settingRepository struct{ db *gorm.DB }

func NewSettingRepository(db *gorm.DB) domainRepo.SettingRepository {
	return &settingRepository{db: db}
}

func (r *settingRepository) q(ctx context.Context) *gorm.DB { return xpostgres.DB(ctx, r.db) }

type settingRow struct {
	Key         string    `gorm:"column:key"`
	Value       string    `gorm:"column:value"`
	Description string    `gorm:"column:description"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
	UpdatedBy   *int64    `gorm:"column:updated_by"`
}

func toSetting(r settingRow) model.Setting {
	return model.Setting{
		Key: r.Key, Value: r.Value, Description: r.Description,
		UpdatedAt: r.UpdatedAt, UpdatedBy: r.UpdatedBy,
	}
}

func (r *settingRepository) GetAll(ctx context.Context) ([]model.Setting, error) {
	var rows []settingRow
	if err := r.q(ctx).Raw("SELECT key, value, description, updated_at, updated_by FROM settings ORDER BY key").Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]model.Setting, len(rows))
	for i, row := range rows {
		out[i] = toSetting(row)
	}
	return out, nil
}

func (r *settingRepository) Get(ctx context.Context, key string) (*model.Setting, error) {
	var row settingRow
	if err := r.q(ctx).Raw("SELECT key, value, description, updated_at, updated_by FROM settings WHERE key = ? LIMIT 1", key).Scan(&row).Error; err != nil {
		return nil, err
	}
	if row.Key == "" {
		return nil, nil
	}
	s := toSetting(row)
	return &s, nil
}

func (r *settingRepository) Set(ctx context.Context, key, value string, updatedBy int64) error {
	return r.q(ctx).Exec(
		`INSERT INTO settings (key, value, updated_at, updated_by) VALUES (?, ?, NOW(), ?)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = NOW(), updated_by = EXCLUDED.updated_by`,
		key, value, updatedBy,
	).Error
}
