package usecase

import (
	"context"

	"vtv.vn/backend/internal/domain/model"
	domainRepo "vtv.vn/backend/internal/domain/repository"
	domainUC "vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

type settingUseCase struct {
	logger   *xlogger.Logger
	settings domainRepo.SettingRepository
}

func NewSettingUseCase(logger *xlogger.Logger, settings domainRepo.SettingRepository) domainUC.SettingUseCase {
	return &settingUseCase{logger: logger, settings: settings}
}

func (u *settingUseCase) GetAll(ctx context.Context) ([]model.Setting, error) {
	rows, err := u.settings.GetAll(ctx)
	if err != nil {
		u.logger.Error("settingUC.GetAll", xlogger.Err(err))
		return nil, xhttp.InternalErrorf("lỗi lấy danh sách cài đặt")
	}
	return rows, nil
}

func (u *settingUseCase) Get(ctx context.Context, key string) (*model.Setting, error) {
	s, err := u.settings.Get(ctx, key)
	if err != nil {
		u.logger.Error("settingUC.Get", xlogger.Err(err))
		return nil, xhttp.InternalErrorf("lỗi lấy cài đặt")
	}
	if s == nil {
		return nil, xhttp.NotFoundErrorf("cài đặt '%s' không tồn tại", key)
	}
	return s, nil
}

func (u *settingUseCase) Set(ctx context.Context, key, value string) error {
	callerID := xhttp.UserID(ctx)
	if err := u.settings.Set(ctx, key, value, callerID); err != nil {
		u.logger.Error("settingUC.Set", xlogger.Err(err))
		return xhttp.InternalErrorf("lỗi lưu cài đặt")
	}
	return nil
}
