package handler

import (
	"github.com/labstack/echo/v4"

	domainUC "vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/internal/server/http/dto"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// SettingHandler handles /api/v1/settings routes.
type SettingHandler struct {
	logger *xlogger.Logger
	uc     domainUC.SettingUseCase
}

func NewSettingHandler(logger *xlogger.Logger, uc domainUC.SettingUseCase) *SettingHandler {
	return &SettingHandler{logger: logger, uc: uc}
}

// List godoc
// @Summary  Danh sách cài đặt hệ thống
// @Tags     settings
// @Produce  json
// @Success  200  {object}  xhttp.Envelope{data=[]dto.SettingResponse}
// @Security BearerAuth
// @Router   /settings [get]
func (h *SettingHandler) List(c echo.Context) error {
	rows, err := h.uc.GetAll(c.Request().Context())
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	out := make([]dto.SettingResponse, len(rows))
	for i, s := range rows {
		out[i] = dto.ToSettingResponse(s)
	}
	return xhttp.SuccessResponse(c, out)
}

// Get godoc
// @Summary  Lấy một cài đặt theo key
// @Tags     settings
// @Produce  json
// @Param    key  path  string  true  "Setting key"
// @Success  200  {object}  xhttp.Envelope{data=dto.SettingResponse}
// @Failure  404  {object}  xhttp.Problem
// @Security BearerAuth
// @Router   /settings/{key} [get]
func (h *SettingHandler) Get(c echo.Context) error {
	key := c.Param("key")
	s, err := h.uc.Get(c.Request().Context(), key)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToSettingResponse(*s))
}

// Set godoc
// @Summary  Cập nhật giá trị cài đặt
// @Tags     settings
// @Accept   json
// @Produce  json
// @Param    key   path  string                true  "Setting key"
// @Param    body  body  dto.SetSettingRequest  true  "Giá trị mới"
// @Success  200  {object}  xhttp.Envelope
// @Security BearerAuth
// @Router   /settings/{key} [put]
func (h *SettingHandler) Set(c echo.Context) error {
	key := c.Param("key")
	var req dto.SetSettingRequest
	if errs := xhttp.ReadAndValidateRequest(c, &req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	if err := h.uc.Set(c.Request().Context(), key, req.Value); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, nil)
}
