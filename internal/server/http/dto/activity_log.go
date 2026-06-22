package dto

import (
	"encoding/json"

	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
)

// ToAuditLogRowResponse converts a repository AuditLogRow to its response DTO.
func ToAuditLogRowResponse(r repository.AuditLogRow) model.AuditLogRowResponse {
	var meta any
	if len(r.Metadata) > 0 && string(r.Metadata) != "null" {
		_ = json.Unmarshal(r.Metadata, &meta)
	}
	return model.AuditLogRowResponse{
		ID:           r.ID,
		UserID:       r.UserID,
		UserFullName: r.UserFullName,
		UserRole:     r.UserRole,
		Action:       r.Action,
		EntityType:   r.EntityType,
		EntityID:     r.EntityID,
		EntityName:   r.EntityName,
		Description:  r.Description,
		Metadata:     meta,
		IPAddress:    r.IPAddress,
		CreatedAt:    r.CreatedAt,
	}
}

// ToAuditLogRowResponses converts a slice of repository AuditLogRow to response DTOs.
func ToAuditLogRowResponses(rows []repository.AuditLogRow) []model.AuditLogRowResponse {
	out := make([]model.AuditLogRowResponse, len(rows))
	for i, r := range rows {
		out[i] = ToAuditLogRowResponse(r)
	}
	return out
}

// ToAuditActorResponse converts a repository AuditActor to its response DTO.
func ToAuditActorResponse(a repository.AuditActor) model.AuditActorResponse {
	return model.AuditActorResponse{
		UserID:       a.UserID,
		UserFullName: a.UserFullName,
		UserRole:     a.UserRole,
	}
}

// ToAuditActorResponses converts a slice of repository AuditActor to response DTOs.
func ToAuditActorResponses(actors []repository.AuditActor) []model.AuditActorResponse {
	out := make([]model.AuditActorResponse, len(actors))
	for i, a := range actors {
		out[i] = ToAuditActorResponse(a)
	}
	return out
}
