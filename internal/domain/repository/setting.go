package repository

import (
	"context"

	"vtv.vn/backend/internal/domain/model"
)

// SettingRepository manages system key-value configuration.
type SettingRepository interface {
	GetAll(ctx context.Context) ([]model.Setting, error)
	Get(ctx context.Context, key string) (*model.Setting, error)
	Set(ctx context.Context, key, value string, updatedBy int64) error
}
