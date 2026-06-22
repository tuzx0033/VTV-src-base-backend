package consts

// Permission resources — keys used in the `user_permissions.permissions` JSONB.
// Hierarchy is dot-delimited (e.g. "users.permissions"). Order of declaration
// mirrors the FE permission tree. Add project-specific resources here.
const (
	ResourceDashboard = "dashboard"

	ResourceNotifications = "notifications"
	ResourceAnnouncements = "announcements"
	ResourceAuditLogs     = "audit_logs"

	ResourceUsers           = "users"
	ResourceUserPermissions = "users.permissions"

	ResourceSettings = "settings"
	ResourceInternal = "internal"
)

// Permission actions — verbs applied to a resource. Stored verbatim in JSONB.
const (
	ActionRead     = "read"
	ActionWrite    = "write"
	ActionDelete   = "delete"
	ActionVerify   = "verify"
	ActionApprove  = "approve"
	ActionReject   = "reject"
	ActionImport   = "import"
	ActionExport   = "export"
	ActionUpload   = "upload"
	ActionDownload = "download"
	ActionSendMail = "send_mail"
	ActionSyncData = "sync_data"
	ActionManage   = "manage"
)

// Action is the FE-facing label for an action ID.
type Action struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Permission is one node of the permission tree.
type Permission struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Actions     []string     `json:"actions"`
	Children    []Permission `json:"children,omitempty"`
}

// ActionsDefinitions is the source of truth for the action picker UI.
var ActionsDefinitions = []Action{
	{ID: ActionRead, Name: "Xem"},
	{ID: ActionWrite, Name: "Chỉnh sửa"},
	{ID: ActionDelete, Name: "Xóa"},
	{ID: ActionVerify, Name: "Xác minh"},
	{ID: ActionApprove, Name: "Phê duyệt"},
	{ID: ActionReject, Name: "Từ chối"},
	{ID: ActionImport, Name: "Tải dữ liệu"},
	{ID: ActionExport, Name: "Xuất dữ liệu"},
	{ID: ActionUpload, Name: "Tải lên tệp tài liệu"},
	{ID: ActionDownload, Name: "Tải xuống tệp tài liệu"},
	{ID: ActionSendMail, Name: "Gửi mail"},
	{ID: ActionSyncData, Name: "Làm mới dữ liệu"},
	{ID: ActionManage, Name: "Quản trị"},
}

// PermissionsDefinitions is the source of truth for the resource tree.
// Adding a new resource here automatically exposes it in /permissions/definitions.
var PermissionsDefinitions = []Permission{
	{
		ID:          ResourceDashboard,
		Name:        "Tổng quan",
		Description: "Truy cập dashboard",
		Actions:     []string{ActionRead},
	},
	{
		ID:          ResourceNotifications,
		Name:        "Thông báo cá nhân",
		Description: "Hộp thông báo của tài khoản",
		Actions:     []string{ActionRead, ActionWrite},
	},
	{
		ID:          ResourceAnnouncements,
		Name:        "Bảng tin",
		Description: "Đăng announcement cho toàn hệ thống",
		Actions:     []string{ActionRead, ActionWrite, ActionDelete, ActionApprove},
	},
	{
		ID:          ResourceAuditLogs,
		Name:        "Nhật ký hoạt động",
		Description: "Xem activity / audit log",
		Actions:     []string{ActionRead, ActionDelete},
	},
	{
		ID:          ResourceUsers,
		Name:        "Quản lý người dùng",
		Description: "CRUD tài khoản nội bộ",
		Actions:     []string{ActionRead, ActionWrite, ActionDelete},
		Children: []Permission{
			{
				ID:          ResourceUserPermissions,
				Name:        "Phân quyền người dùng",
				Description: "Cấp/thu hồi quyền cho người dùng — cấp thận trọng",
				Actions:     []string{ActionRead, ActionWrite, ActionDelete},
			},
		},
	},
	{
		ID:          ResourceSettings,
		Name:        "Cài đặt hệ thống",
		Description: "Các tham số hệ thống",
		Actions:     []string{ActionRead, ActionWrite},
	},
	{
		ID:          ResourceInternal,
		Name:        "Internal API",
		Description: "Gọi API service-to-service",
		Actions:     []string{ActionManage},
	},
}
