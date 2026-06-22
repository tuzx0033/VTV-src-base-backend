package handler

import (
	"github.com/labstack/echo/v4"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/internal/server/http/dto"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
)

// PermissionHandler serves /user-permissions/* and /permissions/*.
type PermissionHandler struct {
	logger *xlogger.Logger
	uc     usecase.PermissionUseCase
}

// NewPermissionHandler builds the permission handler.
func NewPermissionHandler(logger *xlogger.Logger, uc usecase.PermissionUseCase) *PermissionHandler {
	return &PermissionHandler{logger: logger, uc: uc}
}

// Definitions godoc
// @Summary		Cây quyền & action định nghĩa
// @Tags		permissions
// @Produce		json
// @Success		200	{object}	xhttp.Envelope
// @Security	BearerAuth
// @Router		/permissions/definitions [get]
func (h *PermissionHandler) Definitions(c echo.Context) error {
	return xhttp.SuccessResponse(c, dto.ToPermissionDefinitions(consts.PermissionsDefinitions))
}

// Actions godoc
// @Summary		Danh sách action có tên VN
// @Tags		permissions
// @Produce		json
// @Success		200	{object}	xhttp.Envelope
// @Security	BearerAuth
// @Router		/permissions/actions [get]
func (h *PermissionHandler) Actions(c echo.Context) error {
	return xhttp.SuccessResponse(c, dto.ToActionDefinitions(consts.ActionsDefinitions))
}

// Me godoc
// @Summary		Phân quyền hiệu lực của tôi
// @Tags		permissions
// @Produce		json
// @Success		200	{object}	xhttp.Envelope{data=dto.MyPermissionsResponse}
// @Security	BearerAuth
// @Router		/permissions/me [get]
func (h *PermissionHandler) Me(c echo.Context) error {
	ctx := c.Request().Context()
	userID := xhttp.UserID(ctx)
	role := xhttp.Role(ctx)
	isAdmin := role == consts.RoleAdmin
	perms, err := h.uc.EffectivePermissions(ctx, userID, isAdmin)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	if perms == nil {
		perms = model.PermissionMap{}
	}
	// Return raw permission map directly under `data` (matches CP convention
	// + simplifies FE — admin gets {"*": {"*": true}} for god-mode access).
	return xhttp.SuccessResponse(c, map[string]map[string]bool(perms))
}

// Create godoc
// @Summary		Tạo phân quyền cho người dùng (matrix rỗng)
// @Tags		permissions
// @Accept		json
// @Produce		json
// @Param		request	body		dto.CreateUserPermissionRequest	true	"payload"
// @Success		201		{object}	xhttp.Envelope{data=dto.UserPermissionResponse}
// @Security	BearerAuth
// @Router		/user-permissions [post]
func (h *PermissionHandler) Create(c echo.Context) error {
	req := &dto.CreateUserPermissionRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	up, err := h.uc.Create(c.Request().Context(), usecase.CreateUserPermissionInput{UserID: req.UserID})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.CreatedResponse(c, dto.ToUserPermissionResponse(up))
}

// Get godoc
// @Summary		Chi tiết phân quyền (theo id row)
// @Tags		permissions
// @Produce		json
// @Param		id	path		int	true	"user_permission id"
// @Success		200	{object}	xhttp.Envelope{data=dto.UserPermissionResponse}
// @Security	BearerAuth
// @Router		/user-permissions/{id} [get]
func (h *PermissionHandler) Get(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	up, err := h.uc.GetByID(c.Request().Context(), id)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserPermissionResponse(up))
}

// List godoc
// @Summary		Danh sách phân quyền theo user
// @Tags		permissions
// @Produce		json
// @Param		page	query	int	false "page"
// @Param		limit	query	int	false "limit"
// @Param		userId	query	int	false "filter theo user"
// @Param		search	query	string false "tìm theo tên / username / mã"
// @Success		200	{object}	xhttp.PagedEnvelope{data=[]dto.UserPermissionResponse}
// @Security	BearerAuth
// @Router		/user-permissions [get]
func (h *PermissionHandler) List(c echo.Context) error {
	q := &dto.ListUserPermissionsQuery{}
	if errs := xhttp.ReadAndValidateRequest(c, q); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	rows, total, err := h.uc.List(c.Request().Context(), repository.UserPermissionListFilter{
		Search: q.Search, UserID: q.UserID, Page: model.NewPage(q.Page, q.Limit),
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.PaginatedResponse(c, dto.ToUserPermissionResponses(rows), q.Page, q.Limit, total)
}

// Update godoc
// @Summary		Cập nhật ma trận phân quyền (theo id row)
// @Tags		permissions
// @Accept		json
// @Produce		json
// @Param		id		path		int								true	"user_permission id"
// @Param		request	body		dto.UpdateUserPermissionRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope{data=dto.UserPermissionResponse}
// @Security	BearerAuth
// @Router		/user-permissions/{id} [put]
func (h *PermissionHandler) Update(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	req := &dto.UpdateUserPermissionRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	up, err := h.uc.Update(c.Request().Context(), id, usecase.UpdateUserPermissionInput{
		Permissions: toPermissionMap(req.Permissions),
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.SuccessResponse(c, dto.ToUserPermissionResponse(up))
}

// UpdateByUser godoc
// @Summary		Cập nhật phân quyền theo user_id (FE-compat)
// @Tags		permissions
// @Accept		json
// @Produce		json
// @Param		id		path		int										true	"user id"
// @Param		request	body		dto.UpdateUserPermissionsByUserRequest	true	"payload"
// @Success		200		{object}	xhttp.Envelope{data=dto.UserPermissionResponse}
// @Security	BearerAuth
// @Router		/users/{id}/permissions [put]
func (h *PermissionHandler) UpdateByUser(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	req := &dto.UpdateUserPermissionsByUserRequest{}
	if errs := xhttp.ReadAndValidateRequest(c, req); errs != nil {
		return xhttp.BadRequestResponse(c, errs)
	}
	up, err := h.uc.UpdateByUserID(c.Request().Context(), id, usecase.UpdateUserPermissionInput{
		Permissions: toPermissionMap(req.Permissions),
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	resp := dto.ToUserPermissionResponse(up)
	// Return effective so FE can immediately display the merged result
	// without waiting for a separate GET refetch.
	eff, err := h.uc.EffectivePermissions(c.Request().Context(), id, false)
	if err == nil && eff != nil {
		resp.Effective = map[string]map[string]bool(eff)
	}
	return xhttp.SuccessResponse(c, resp)
}

// GetByUser godoc
// @Summary		Lấy phân quyền hiện tại theo user_id (FE-compat)
// @Tags		permissions
// @Produce		json
// @Param		id	path		int	true	"user id"
// @Success		200	{object}	xhttp.Envelope{data=dto.UserPermissionResponse}
// @Security	BearerAuth
// @Router		/users/{id}/permissions [get]
func (h *PermissionHandler) GetByUser(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	up, err := h.uc.GetByUserID(c.Request().Context(), id)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	resp := dto.ToUserPermissionResponse(up)
	// Compute effective = role defaults merged with overrides, so FE
	// permission matrix shows the full picture (same as /permissions/me).
	eff, err := h.uc.EffectivePermissions(c.Request().Context(), id, false)
	if err == nil && eff != nil {
		resp.Effective = map[string]map[string]bool(eff)
	}
	return xhttp.SuccessResponse(c, resp)
}

// Delete godoc
// @Summary		Xoá phân quyền (theo id row)
// @Tags		permissions
// @Produce		json
// @Param		id	path	int	true	"user_permission id"
// @Success		204
// @Security	BearerAuth
// @Router		/user-permissions/{id} [delete]
func (h *PermissionHandler) Delete(c echo.Context) error {
	id, err := parseIDParam(c)
	if err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	if err := h.uc.Delete(c.Request().Context(), id); err != nil {
		return xhttp.AppErrorResponse(c, err)
	}
	return xhttp.NoContentResponse(c)
}

func toPermissionMap(in map[string]map[string]bool) model.PermissionMap {
	out := model.PermissionMap{}
	for r, acts := range in {
		if acts == nil {
			continue
		}
		copyActs := make(map[string]bool, len(acts))
		for k, v := range acts {
			copyActs[k] = v
		}
		out[r] = copyActs
	}
	return out
}
