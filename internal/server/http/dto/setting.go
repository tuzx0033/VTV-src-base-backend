package dto

import (
	"time"

	"vtv.vn/backend/internal/domain/model"
)

type SettingResponse struct {
	Key         string    `json:"key"`
	Value       string    `json:"value"`
	Description string    `json:"description"`
	UpdatedAt   time.Time `json:"updatedAt"`
	UpdatedBy   *int64    `json:"updatedBy,omitempty"`
}

func ToSettingResponse(s model.Setting) SettingResponse {
	return SettingResponse{
		Key: s.Key, Value: s.Value, Description: s.Description,
		UpdatedAt: s.UpdatedAt, UpdatedBy: s.UpdatedBy,
	}
}

type SetSettingRequest struct {
	Value string `json:"value"`
}
