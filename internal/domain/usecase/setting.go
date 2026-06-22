package usecase

import (
	"context"

	"vtv.vn/backend/internal/domain/model"
)

// SettingUseCase provides system configuration management.
type SettingUseCase interface {
	GetAll(ctx context.Context) ([]model.Setting, error)
	Get(ctx context.Context, key string) (*model.Setting, error)
	Set(ctx context.Context, key, value string) error
}
