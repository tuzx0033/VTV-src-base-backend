package usecase

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/pkg/xcrypto"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xmail"
	"vtv.vn/backend/pkg/xpostgres"
	"vtv.vn/backend/pkg/xstorage"
)

type userUseCase struct {
	logger   *xlogger.Logger
	tx       xpostgres.TxRunner
	users    repository.UserRepository
	perms    repository.PermissionRepository
	audit    repository.AuditRepository
	storage  xstorage.Provider // optional; nil = upload disabled
	mailer   xmail.Sender      // optional; nil = log fallback at DI level
	loginURL string            // base FE login URL (vd https://app.example.com/login)
}

// NewUserUseCase builds the user-management use case.
func NewUserUseCase(
	logger *xlogger.Logger,
	tx xpostgres.TxRunner,
	users repository.UserRepository,
	perms repository.PermissionRepository,
	audit repository.AuditRepository,
	storage xstorage.Provider,
	mailer xmail.Sender,
	loginURL string,
) usecase.UserUseCase {
	return &userUseCase{logger: logger, tx: tx, users: users, perms: perms, audit: audit, storage: storage, mailer: mailer, loginURL: loginURL}
}

func (u *userUseCase) Get(ctx context.Context, id int64) (*model.User, error) {
	user, err := u.users.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, xhttp.NotFoundErrorf("không tìm thấy người dùng %d", id)
	}
	return user, nil
}

func (u *userUseCase) List(ctx context.Context, f repository.UserListFilter) ([]model.User, int64, error) {
	return u.users.List(ctx, f)
}

func (u *userUseCase) Create(ctx context.Context, in usecase.CreateUserInput) (*model.User, error) {
	username := strings.TrimSpace(in.Username)
	if username == "" || strings.TrimSpace(in.FullName) == "" {
		return nil, xhttp.BadRequestErrorf("username và full_name là bắt buộc")
	}
	if len(in.Password) < minPasswordLen {
		return nil, xhttp.BadRequestErrorf("mật khẩu phải có ít nhất %d ký tự", minPasswordLen)
	}
	role := model.Role(in.Role)
	if !role.Valid() {
		return nil, xhttp.BadRequestErrorf("role không hợp lệ")
	}
	var created *model.User
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		exists, err := u.users.ExistsByUsername(ctx, username)
		if err != nil {
			return err
		}
		if exists {
			return xhttp.ConflictErrorf("username đã tồn tại")
		}
		hash, err := xcrypto.HashPassword(in.Password)
		if err != nil {
			return err
		}
		actor := xhttp.UserID(ctx)
		user := &model.User{
			Username: username, PasswordHash: hash, FullName: strings.TrimSpace(in.FullName),
			Email: trimPtr(in.Email), Phone: trimPtr(in.Phone), Role: role, Status: model.UserStatusActive,
			StaffCode:           upperPtr(in.StaffCode),
			EmploymentType:      model.EmploymentType(in.EmploymentType),
			ForcePasswordChange: in.ForcePasswordChange,
		}
		user.CreatedBy = actor
		user.UpdatedBy = actor
		if err := u.users.Create(ctx, user); err != nil {
			return err
		}
		created = user
		// Seed role-default permissions so /users/:id/permissions returns a
		// populated matrix immediately (FE permission editor shows checkboxes).
		if role != model.Role(consts.RoleAdmin) {
			defaults := model.RoleDefaultPermissions(string(role))
			up := &model.UserPermission{
				UserID:          user.ID,
				Permissions:     defaults,
				PermissionCount: defaults.Count(),
				CreatedBy:       actor,
				UpdatedBy:       actor,
			}
			if err := u.perms.Create(ctx, up); err != nil {
				return err
			}
		}
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserCreated, EntityType: "user", EntityID: user.ID, EntityName: user.Username,
			Summary: "tạo người dùng " + user.Username,
		})
	})
	if err != nil {
		return nil, err
	}
	return created, nil
}

func (u *userUseCase) Update(ctx context.Context, id int64, in usecase.UpdateUserInput) (*model.User, error) {
	if in.Role != nil && !model.Role(*in.Role).Valid() {
		return nil, xhttp.BadRequestErrorf("role không hợp lệ")
	}
	var out *model.User
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		user, err := u.users.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if user == nil {
			return xhttp.NotFoundErrorf("không tìm thấy người dùng %d", id)
		}
		if in.FullName != nil && strings.TrimSpace(*in.FullName) != "" {
			user.FullName = strings.TrimSpace(*in.FullName)
		}
		if in.Email != nil {
			user.Email = trimPtr(in.Email)
		}
		if in.Phone != nil {
			user.Phone = trimPtr(in.Phone)
		}
		if in.Role != nil {
			user.Role = model.Role(*in.Role)
		}
		if in.StaffCode != nil {
			user.StaffCode = upperPtr(in.StaffCode)
		}
		if in.EmploymentType != nil {
			user.EmploymentType = model.EmploymentType(strings.TrimSpace(*in.EmploymentType))
		}
		user.UpdatedBy = xhttp.UserID(ctx)
		if err := u.users.Update(ctx, user); err != nil {
			return err
		}
		out = user
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserUpdated, EntityType: "user", EntityID: user.ID, EntityName: user.Username,
			Summary: "cập nhật người dùng " + user.Username,
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (u *userUseCase) ResetPassword(ctx context.Context, id int64, newPassword string) error {
	if len(newPassword) < minPasswordLen {
		return xhttp.BadRequestErrorf("mật khẩu phải có ít nhất %d ký tự", minPasswordLen)
	}
	return u.tx.Run(ctx, func(ctx context.Context) error {
		user, err := u.users.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if user == nil {
			return xhttp.NotFoundErrorf("không tìm thấy người dùng %d", id)
		}
		hash, err := xcrypto.HashPassword(newPassword)
		if err != nil {
			return err
		}
		user.SetPasswordHash(hash)
		user.UpdatedBy = xhttp.UserID(ctx)
		if err := u.users.Update(ctx, user); err != nil {
			return err
		}
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserPasswordReset, EntityType: "user", EntityID: user.ID, EntityName: user.Username,
			Summary: "đặt lại mật khẩu cho " + user.Username,
		})
	})
}

func (u *userUseCase) ToggleStatus(ctx context.Context, id int64) (*model.User, error) {
	if id == xhttp.UserID(ctx) {
		return nil, xhttp.BadRequestErrorf("không thể tự khoá tài khoản của mình")
	}
	var out *model.User
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		user, err := u.users.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if user == nil {
			return xhttp.NotFoundErrorf("không tìm thấy người dùng %d", id)
		}
		if user.IsLocked() {
			if e := user.Unlock(); e != nil {
				return xhttp.UnprocessableErrorf("%v", e)
			}
		} else if e := user.Lock(); e != nil {
			return xhttp.UnprocessableErrorf("%v", e)
		}
		user.UpdatedBy = xhttp.UserID(ctx)
		if err := u.users.Update(ctx, user); err != nil {
			return err
		}
		out = user
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserStatusToggled, EntityType: "user", EntityID: user.ID, EntityName: user.Username,
			Summary: "đổi trạng thái thành " + string(user.Status),
			Changes: map[string]any{"status": string(user.Status)},
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (u *userUseCase) UpdateProfile(ctx context.Context, in usecase.UpdateProfileInput) (*model.User, error) {
	if in.Password != nil && len(*in.Password) < minPasswordLen {
		return nil, xhttp.BadRequestErrorf("mật khẩu phải có ít nhất %d ký tự", minPasswordLen)
	}
	var out *model.User
	err := u.tx.Run(ctx, func(ctx context.Context) error {
		id := xhttp.UserID(ctx)
		user, err := u.users.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if user == nil {
			return xhttp.UnauthorizedErrorf("không tìm thấy người dùng")
		}
		if in.FullName != nil && strings.TrimSpace(*in.FullName) != "" {
			user.FullName = strings.TrimSpace(*in.FullName)
		}
		if in.Email != nil {
			user.Email = trimPtr(in.Email)
		}
		if in.Phone != nil {
			user.Phone = trimPtr(in.Phone)
		}
		if in.AvatarURL != nil {
			user.AvatarURL = trimPtr(in.AvatarURL)
		}
		if in.Password != nil {
			if in.CurrentPassword == nil || *in.CurrentPassword == "" {
				return xhttp.BadRequestErrorf("thiếu mật khẩu hiện tại để xác nhận đổi")
			}
			if !xcrypto.ComparePassword(*in.CurrentPassword, user.PasswordHash) {
				return xhttp.UnauthorizedErrorf("mật khẩu hiện tại không đúng")
			}
			if *in.Password == *in.CurrentPassword {
				return xhttp.BadRequestErrorf("mật khẩu mới không được trùng mật khẩu hiện tại")
			}
			hash, herr := xcrypto.HashPassword(*in.Password)
			if herr != nil {
				return herr
			}
			user.SetPasswordHash(hash)
		}
		user.UpdatedBy = id
		if err := u.users.Update(ctx, user); err != nil {
			return err
		}
		out = user
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: "user.profile_updated", EntityType: "user", EntityID: user.ID, EntityName: user.Username,
			Summary: "cập nhật hồ sơ cá nhân",
		})
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

func (u *userUseCase) NextStaffCode(ctx context.Context, role string) (string, error) {
	var prefix string
	switch role {
	case consts.RoleAdmin:
		prefix = "ADMIN"
	case consts.RoleManager:
		prefix = "MGR"
	default:
		prefix = "ST"
	}
	codes, err := u.users.StaffCodesWithPrefix(ctx, prefix)
	if err != nil {
		return "", err
	}
	maxN := 0
	for _, c := range codes {
		num := strings.TrimLeft(strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(c)), prefix), "0")
		if num == "" {
			num = "0"
		}
		if n, e := strconv.Atoi(num); e == nil && n > maxN {
			maxN = n
		}
	}
	next := maxN + 1
	if role == consts.RoleAdmin && next == 1 {
		return "ADMIN", nil
	}
	if role == consts.RoleAdmin {
		return fmt.Sprintf("ADMIN%02d", next), nil
	}
	return fmt.Sprintf("%s%02d", prefix, next), nil
}

func (u *userUseCase) UploadAvatar(ctx context.Context, userID int64, filename string, size int64, body io.Reader) (*model.User, error) {
	if u.storage == nil {
		return nil, xhttp.ServiceUnavailableErrorf("tính năng upload ảnh chưa được cấu hình")
	}
	if size > maxAvatarBytes {
		return nil, xhttp.BadRequestErrorf("kích thước ảnh không được vượt quá 5MB")
	}

	callerID := xhttp.UserID(ctx)
	role := xhttp.Role(ctx)

	// Authorization: user can only upload their own avatar; admin can upload for anyone.
	if role != consts.RoleAdmin && callerID != userID {
		return nil, xhttp.ForbiddenErrorf("không có quyền upload avatar cho người dùng này")
	}

	// Sniff content type from first 512 bytes.
	var sniff [512]byte
	n, err := io.ReadFull(body, sniff[:])
	if err != nil && err != io.ErrUnexpectedEOF {
		return nil, xhttp.BadRequestErrorf("không thể đọc file")
	}
	ct := http.DetectContentType(sniff[:n])
	ext, extErr := avatarExtFromContentType(ct)
	if extErr != nil {
		return nil, xhttp.BadRequestErrorf("định dạng ảnh không hợp lệ, chỉ chấp nhận JPEG, PNG hoặc WebP")
	}

	fullReader := io.MultiReader(bytes.NewReader(sniff[:n]), body)

	user, err := u.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, xhttp.NotFoundErrorf("không tìm thấy người dùng %d", userID)
	}

	key := fmt.Sprintf("users/%d/avatar_%d%s", userID, time.Now().UnixMilli(), ext)
	publicURL, uploadErr := u.storage.Upload(ctx, xstorage.UploadInput{
		Key:         key,
		Body:        fullReader,
		Size:        size,
		ContentType: ct,
	})
	if uploadErr != nil {
		return nil, xhttp.InternalErrorf("upload ảnh thất bại").Wrap(uploadErr)
	}

	user.AvatarURL = &publicURL
	user.UpdatedBy = callerID
	if err := u.users.Update(ctx, user); err != nil {
		return nil, err
	}
	_ = u.audit.Write(ctx, repository.AuditEntry{
		Action: consts.AuditUserUpdated, EntityType: "user", EntityID: user.ID, EntityName: user.Username,
		ActorUserID: callerID, ActorRole: role, Summary: "upload avatar người dùng " + user.Username,
	})
	return user, nil
}

// ─────────────────────────────────────────────────────────────────────────
// Invite flow — admin invite user qua email; BE auto-derive username từ
// email local-part, sinh password ngẫu nhiên (12 ký tự, exclude ambiguous
// chars), set force_password_change=true, audit + gửi credentials qua email.
// ─────────────────────────────────────────────────────────────────────────

// invitePasswordAlphabet excludes ambiguous chars (0/O/1/l/I) để admin có
// thể đọc & dictate password qua chat/phone nếu email không tới.
const invitePasswordAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789"

// generateInvitePassword sinh password 12 ký tự mix upper+lower+digit, dùng
// crypto/rand. Guarantee có ít nhất 1 upper, 1 lower, 1 digit.
func generateInvitePassword() (string, error) {
	const length = 12
	// Pre-allocate 3 slots cho upper/lower/digit để guarantee composition.
	parts := []string{"ABCDEFGHJKLMNPQRSTUVWXYZ", "abcdefghijkmnopqrstuvwxyz", "23456789"}
	buf := make([]byte, 0, length)
	for _, set := range parts {
		c, err := randCharFrom(set)
		if err != nil {
			return "", err
		}
		buf = append(buf, c)
	}
	for len(buf) < length {
		c, err := randCharFrom(invitePasswordAlphabet)
		if err != nil {
			return "", err
		}
		buf = append(buf, c)
	}
	// Fisher–Yates shuffle để vị trí 3 ký tự pre-allocated không cố định.
	for i := len(buf) - 1; i > 0; i-- {
		jBig, err := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		if err != nil {
			return "", err
		}
		j := int(jBig.Int64())
		buf[i], buf[j] = buf[j], buf[i]
	}
	return string(buf), nil
}

func randCharFrom(set string) (byte, error) {
	idx, err := rand.Int(rand.Reader, big.NewInt(int64(len(set))))
	if err != nil {
		return 0, err
	}
	return set[idx.Int64()], nil
}

// deriveUsername lấy local-part trước @, lowercase, replace ký tự không an
// toàn bằng "-". Caller phải check trùng (loop suffix "-2", "-3", …).
func deriveUsername(email string) string {
	at := strings.IndexByte(email, '@')
	local := email
	if at > 0 {
		local = email[:at]
	}
	local = strings.ToLower(strings.TrimSpace(local))
	var b strings.Builder
	b.Grow(len(local))
	for _, r := range local {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-' || r == '+':
			b.WriteByte('-')
		default:
			// strip ký tự khác (unicode dấu, ký tự đặc biệt, …) — username
			// chỉ chứa [a-z0-9-]. Empty result sẽ rơi về fallback.
		}
	}
	out := strings.Trim(b.String(), "-")
	// Collapse multiple "-" thành 1.
	for strings.Contains(out, "--") {
		out = strings.ReplaceAll(out, "--", "-")
	}
	if out == "" {
		out = "user"
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func (u *userUseCase) Invite(ctx context.Context, in usecase.InviteUserInput) (*model.User, bool, string, error) {
	email := strings.TrimSpace(in.Email)
	fullName := strings.TrimSpace(in.FullName)
	if email == "" || fullName == "" {
		return nil, false, "", xhttp.BadRequestErrorf("email và họ tên là bắt buộc")
	}
	role := model.Role(in.Role)
	if !role.Valid() || role == model.RoleAdmin {
		// Admin role bị cấm ở DTO layer (oneof manager staff); double-check ở đây.
		return nil, false, "", xhttp.BadRequestErrorf("vai trò không hợp lệ")
	}

	plainPw, err := generateInvitePassword()
	if err != nil {
		return nil, false, "", xhttp.InternalErrorf("sinh mật khẩu thất bại").Wrap(err)
	}
	hash, err := xcrypto.HashPassword(plainPw)
	if err != nil {
		return nil, false, "", xhttp.InternalErrorf("hash mật khẩu thất bại").Wrap(err)
	}

	var created *model.User
	err = u.tx.Run(ctx, func(ctx context.Context) error {
		// Resolve unique username — derive từ email, suffix nếu trùng.
		base := deriveUsername(email)
		username := base
		for i := 2; i < 100; i++ {
			exists, e := u.users.ExistsByUsername(ctx, username)
			if e != nil {
				return e
			}
			if !exists {
				break
			}
			username = fmt.Sprintf("%s-%d", base, i)
		}
		// Sanity: nếu loop tới 100 vẫn trùng → fail fast (cực kỳ hiếm).
		if exists, _ := u.users.ExistsByUsername(ctx, username); exists {
			return xhttp.ConflictErrorf("không thể sinh username duy nhất từ email %s", email)
		}

		actor := xhttp.UserID(ctx)
		emailCopy := email
		user := &model.User{
			Username: username, PasswordHash: hash, FullName: fullName,
			Email: &emailCopy, Phone: trimPtr(in.Phone), Role: role,
			Status:              model.UserStatusActive,
			StaffCode:           upperPtr(in.StaffCode),
			ForcePasswordChange: true,
		}
		user.CreatedBy = actor
		user.UpdatedBy = actor
		if err := u.users.Create(ctx, user); err != nil {
			return err
		}
		created = user
		// Seed role-default permissions (same as Create).
		if role != model.Role(consts.RoleAdmin) {
			defaults := model.RoleDefaultPermissions(string(role))
			up := &model.UserPermission{
				UserID:          user.ID,
				Permissions:     defaults,
				PermissionCount: defaults.Count(),
				CreatedBy:       actor,
				UpdatedBy:       actor,
			}
			if err := u.perms.Create(ctx, up); err != nil {
				return err
			}
		}
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserInvited, EntityType: "user",
			EntityID: user.ID, EntityName: user.Username,
			Summary: "mời người dùng " + user.Username + " (" + email + ")",
		})
	})
	if err != nil {
		return nil, false, "", err
	}

	// Send email outside the DB transaction — failure không rollback user;
	// admin nhận tempPassword qua response để handoff thủ công.
	emailSent := false
	if u.mailer != nil {
		msg := buildInviteEmail(email, fullName, created.Username, plainPw, u.loginURL)
		if mErr := u.mailer.Send(ctx, msg); mErr != nil {
			u.logger.Error("invite email send failed",
				xlogger.Err(mErr),
				xlogger.String("username", created.Username),
				xlogger.String("email", email),
			)
		} else {
			emailSent = true
		}
	}
	if !emailSent {
		u.logger.Warn("invite created — email NOT sent (admin must handoff)",
			xlogger.String("username", created.Username),
			xlogger.String("email", email),
		)
		return created, false, plainPw, nil
	}
	return created, true, "", nil
}

// buildInviteEmail formats the welcome email — plain text + HTML.
func buildInviteEmail(to, fullName, username, password, loginURL string) xmail.Message {
	name := fullName
	if strings.TrimSpace(name) == "" {
		name = to
	}
	if strings.TrimSpace(loginURL) == "" {
		loginURL = "/login"
	}
	subject := "Tài khoản VTV của bạn"
	text := "Xin chào " + name + ",\n\n" +
		"Bạn đã được mời tham gia hệ thống VTV. Thông tin đăng nhập:\n\n" +
		"  Trang đăng nhập: " + loginURL + "\n" +
		"  Tên đăng nhập:   " + username + "\n" +
		"  Mật khẩu tạm:    " + password + "\n\n" +
		"Quan trọng: vì lý do bảo mật, hệ thống sẽ buộc bạn đổi mật khẩu " +
		"ngay sau khi đăng nhập lần đầu tiên. Tuyệt đối không chia sẻ " +
		"mật khẩu này cho bất kỳ ai.\n\n" +
		"Nếu bạn không yêu cầu tài khoản này, vui lòng liên hệ quản trị viên.\n\n" +
		"— Hệ thống VTV"
	html := `<!DOCTYPE html><html><body style="font-family:-apple-system,'Segoe UI',Roboto,sans-serif;max-width:560px;margin:32px auto;padding:0;color:#0f172a;line-height:1.6;background:#f8fafc">` +
		`<div style="background:linear-gradient(135deg,#4f46e5 0%,#7c3aed 100%);padding:32px 24px;border-radius:12px 12px 0 0;color:#fff">` +
		`<h2 style="margin:0;font-size:22px">Chào mừng đến VTV</h2>` +
		`<p style="margin:8px 0 0;opacity:.9;font-size:14px">Tài khoản nội bộ đã được tạo cho bạn.</p>` +
		`</div>` +
		`<div style="background:#fff;padding:24px;border-radius:0 0 12px 12px;border:1px solid #e2e8f0;border-top:none">` +
		`<p>Xin chào <strong>` + escapeHTML(name) + `</strong>,</p>` +
		`<p>Quản trị viên vừa tạo cho bạn một tài khoản đăng nhập <strong>VTV</strong>. Dưới đây là thông tin truy cập:</p>` +
		`<div style="background:#f1f5f9;border-radius:8px;padding:16px 20px;margin:16px 0;font-family:'SF Mono',Menlo,Consolas,monospace;font-size:14px">` +
		`<div><span style="color:#64748b">Tên đăng nhập:</span> <strong>` + escapeHTML(username) + `</strong></div>` +
		`<div style="margin-top:6px"><span style="color:#64748b">Mật khẩu tạm:</span> <strong style="color:#4f46e5">` + escapeHTML(password) + `</strong></div>` +
		`</div>` +
		`<p style="margin:24px 0"><a href="` + escapeHTML(loginURL) + `" style="display:inline-block;padding:12px 28px;background:#4f46e5;color:#fff;text-decoration:none;border-radius:8px;font-weight:600">Đăng nhập ngay</a></p>` +
		`<div style="background:#fef3c7;border-left:3px solid #f59e0b;border-radius:4px;padding:12px 16px;margin:20px 0;font-size:13px;color:#78350f">` +
		`<strong>⚠ Quan trọng:</strong> Hệ thống sẽ buộc bạn đổi mật khẩu ngay sau khi đăng nhập lần đầu. Tuyệt đối không chia sẻ mật khẩu này.` +
		`</div>` +
		`<p style="font-size:12px;color:#64748b;margin-top:24px">Nếu nút không hoạt động, mở link: <span style="word-break:break-all">` + escapeHTML(loginURL) + `</span></p>` +
		`<hr style="border:none;border-top:1px solid #e2e8f0;margin:24px 0">` +
		`<p style="font-size:12px;color:#64748b;margin:0">Nếu bạn không yêu cầu tài khoản này, hãy liên hệ quản trị viên ngay.</p>` +
		`</div></body></html>`
	return xmail.Message{
		To:      []string{to},
		Subject: subject,
		Text:    text,
		HTML:    html,
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Bulk lock / unlock / delete
// ─────────────────────────────────────────────────────────────────────────

// bulkApply iterates ids, calls perform per row, accumulates failures. Mỗi
// row chạy trong tx riêng để 1 row fail không rollback whole batch — UX là
// "partial success" với danh sách failed.
func (u *userUseCase) bulkApply(
	ctx context.Context,
	ids []int64,
	perform func(ctx context.Context, user *model.User) (skipReason string, err error),
) (int, []usecase.BulkUserFailure, error) {
	callerID := xhttp.UserID(ctx)
	failures := make([]usecase.BulkUserFailure, 0)
	ok := 0
	seen := make(map[int64]bool, len(ids))
	for _, id := range ids {
		if seen[id] {
			continue
		}
		seen[id] = true
		if id == callerID {
			failures = append(failures, usecase.BulkUserFailure{UserID: id, Reason: "không thể tự thao tác với tài khoản của mình"})
			continue
		}
		var skip string
		err := u.tx.Run(ctx, func(ctx context.Context) error {
			user, gErr := u.users.GetByID(ctx, id)
			if gErr != nil {
				return gErr
			}
			if user == nil {
				skip = "không tìm thấy người dùng"
				return nil
			}
			if user.Role == model.RoleAdmin {
				skip = "không thao tác được lên tài khoản quản trị"
				return nil
			}
			reason, pErr := perform(ctx, user)
			if pErr != nil {
				return pErr
			}
			if reason != "" {
				skip = reason
			}
			return nil
		})
		if err != nil {
			failures = append(failures, usecase.BulkUserFailure{UserID: id, Reason: err.Error()})
			continue
		}
		if skip != "" {
			failures = append(failures, usecase.BulkUserFailure{UserID: id, Reason: skip})
			continue
		}
		ok++
	}
	return ok, failures, nil
}

func (u *userUseCase) BulkLock(ctx context.Context, userIDs []int64) (int, []usecase.BulkUserFailure, error) {
	return u.bulkApply(ctx, userIDs, func(ctx context.Context, user *model.User) (string, error) {
		if user.IsDeleted() {
			return "tài khoản đã bị xoá", nil
		}
		if user.IsLocked() {
			return "đã bị khoá", nil
		}
		if err := user.Lock(); err != nil {
			return err.Error(), nil
		}
		user.UpdatedBy = xhttp.UserID(ctx)
		if err := u.users.Update(ctx, user); err != nil {
			return "", err
		}
		return "", u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserLocked, EntityType: "user",
			EntityID: user.ID, EntityName: user.Username,
			Summary: "khoá người dùng " + user.Username,
		})
	})
}

func (u *userUseCase) BulkUnlock(ctx context.Context, userIDs []int64) (int, []usecase.BulkUserFailure, error) {
	return u.bulkApply(ctx, userIDs, func(ctx context.Context, user *model.User) (string, error) {
		if user.IsDeleted() {
			return "tài khoản đã bị xoá", nil
		}
		if user.Status == model.UserStatusActive {
			return "đã hoạt động", nil
		}
		if err := user.Unlock(); err != nil {
			return err.Error(), nil
		}
		user.UpdatedBy = xhttp.UserID(ctx)
		if err := u.users.Update(ctx, user); err != nil {
			return "", err
		}
		return "", u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserUnlocked, EntityType: "user",
			EntityID: user.ID, EntityName: user.Username,
			Summary: "mở khoá người dùng " + user.Username,
		})
	})
}

func (u *userUseCase) BulkDelete(ctx context.Context, userIDs []int64) (int, []usecase.BulkUserFailure, error) {
	return u.bulkApply(ctx, userIDs, func(ctx context.Context, user *model.User) (string, error) {
		if user.IsDeleted() {
			return "đã bị xoá", nil
		}
		actor := xhttp.UserID(ctx)
		if err := u.users.SoftDelete(ctx, user.ID, actor); err != nil {
			return "", err
		}
		return "", u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditUserDeleted, EntityType: "user",
			EntityID: user.ID, EntityName: user.Username,
			Summary: "xoá người dùng " + user.Username,
		})
	})
}

func trimPtr(p *string) *string {
	if p == nil {
		return nil
	}
	s := strings.TrimSpace(*p)
	if s == "" {
		return nil
	}
	return &s
}

func upperPtr(p *string) *string {
	if p == nil {
		return nil
	}
	s := strings.ToUpper(strings.TrimSpace(*p))
	if s == "" {
		return nil
	}
	return &s
}
