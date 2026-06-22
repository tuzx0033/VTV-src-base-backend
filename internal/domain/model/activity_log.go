package model

// AuditLogRowResponse is the JSON shape of a single audit log entry.
type AuditLogRowResponse struct {
	ID           int64   `json:"id"`
	UserID       *int64  `json:"userId"`
	UserFullName *string `json:"userFullName"`
	UserRole     *string `json:"userRole"`
	Action       string  `json:"action"`
	EntityType   string  `json:"entityType"`
	EntityID     *int64  `json:"entityId"`
	EntityName   *string `json:"entityName"`
	Description  *string `json:"description"`
	Method       *string `json:"method"`
	Path         *string `json:"path"`
	Metadata     any     `json:"metadata"`
	IPAddress    *string `json:"ipAddress"`
	CreatedAt    string  `json:"createdAt"`
}

// AuditActorResponse is the JSON shape of a distinct audit actor.
type AuditActorResponse struct {
	UserID       int64  `json:"userId"`
	UserFullName string `json:"userFullName"`
	UserRole     string `json:"userRole"`
}
