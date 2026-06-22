// Package usecase holds the use-case implementations. Use cases orchestrate:
// load → call entity methods → persist → write audit — all inside one transaction
// for write paths (xpostgres.TxRunner). They never touch HTTP.
package usecase

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"

	"vtv.vn/backend/internal/domain/consts"
	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/domain/repository"
	"vtv.vn/backend/internal/domain/usecase"
	"vtv.vn/backend/pkg/xauth"
	"vtv.vn/backend/pkg/xcrypto"
	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xmail"
	"vtv.vn/backend/pkg/xpostgres"
)

// resetTokenTTL is the lifespan of a password reset token.
const resetTokenTTL = time.Hour

// minPasswordLen mirrors the DTO validate:"min=8" rule. Use cases re-check it
// because they are reachable from non-HTTP entrypoints (cron, seed, import).
const minPasswordLen = 8

type authUseCase struct {
	logger    *xlogger.Logger
	tx        xpostgres.TxRunner
	users     repository.UserRepository
	audit     repository.AuditRepository
	tokens    *xauth.Manager
	blacklist *xauth.Blacklist // nil = no revocation
	mailer    xmail.Sender     // nil = log-to-stdout fallback (dev)
	resetURL  string           // base URL gửi user click — vd https://app.example.com/reset-password
}

// NewAuthUseCase builds the auth use case.
func NewAuthUseCase(logger *xlogger.Logger, tx xpostgres.TxRunner, users repository.UserRepository, audit repository.AuditRepository, tokens *xauth.Manager, blacklist *xauth.Blacklist, mailer xmail.Sender, resetURL string) usecase.AuthUseCase {
	return &authUseCase{logger: logger, tx: tx, users: users, audit: audit, tokens: tokens, blacklist: blacklist, mailer: mailer, resetURL: resetURL}
}

func (u *authUseCase) Login(ctx context.Context, username, password string) (string, *model.User, error) {
	user, err := u.users.GetByUsername(ctx, username)
	if err != nil {
		return "", nil, err
	}
	if user == nil || !xcrypto.ComparePassword(password, user.PasswordHash) {
		_ = u.audit.Write(ctx, repository.AuditEntry{
			Action: consts.AuditAuthLoginFailed, EntityType: "user", Summary: "đăng nhập thất bại: " + username,
		})
		return "", nil, xhttp.UnauthorizedErrorf("tài khoản hoặc mật khẩu không đúng")
	}
	if user.IsLocked() {
		return "", nil, xhttp.ForbiddenErrorf("tài khoản đã bị khoá")
	}
	token, err := u.tokens.Sign(user.ID, string(user.Role))
	if err != nil {
		return "", nil, err
	}
	_ = u.audit.Write(ctx, repository.AuditEntry{
		Action: consts.AuditAuthLoginSucceeded, EntityType: "user", EntityID: user.ID, EntityName: user.Username,
		ActorUserID: user.ID, ActorName: user.FullName, ActorRole: string(user.Role),
	})
	return token, user, nil
}

func (u *authUseCase) Me(ctx context.Context) (*model.User, error) {
	user, err := u.users.GetByID(ctx, xhttp.UserID(ctx))
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, xhttp.UnauthorizedErrorf("không tìm thấy người dùng")
	}
	return user, nil
}

func (u *authUseCase) Logout(ctx context.Context) error {
	if u.blacklist == nil {
		return nil
	}
	jti := xhttp.TokenID(ctx)
	expiry := xhttp.TokenExpiry(ctx)
	if jti == "" || expiry.IsZero() {
		return nil
	}
	return u.blacklist.Revoke(ctx, jti, expiry)
}

func (u *authUseCase) RequestPasswordReset(ctx context.Context, identifier string) (string, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		return "", nil
	}
	user, err := u.users.GetByUsername(ctx, identifier)
	if err != nil {
		return "", err
	}
	if user == nil || user.IsLocked() {
		// Anti-enumeration: return without error; caller masks at handler.
		return "", nil
	}
	// Generate 32 random bytes → 64-char hex token (plain). Store sha256 hash.
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	plain := hex.EncodeToString(raw)
	sum := sha256.Sum256([]byte(plain))
	hash := hex.EncodeToString(sum[:])
	expires := time.Now().UTC().Add(resetTokenTTL)
	if err := u.users.SetResetToken(ctx, user.ID, hash, &expires); err != nil {
		return "", err
	}
	_ = u.audit.Write(ctx, repository.AuditEntry{
		Action: "auth.password_reset_requested", EntityType: "user", EntityID: user.ID,
		EntityName: user.Username,
	})

	// Compose magic-link URL: <resetURL>?token=<plain>
	link := plain
	if u.resetURL != "" {
		sep := "?"
		if strings.Contains(u.resetURL, "?") {
			sep = "&"
		}
		link = u.resetURL + sep + "token=" + plain
	}

	// Send email if SMTP configured + user has email. Else fall back to
	// stdout log (dev). Email failure is logged but doesn't surface to user
	// (anti-enumeration — handler always returns generic 200).
	emailed := false
	if u.mailer != nil && user.Email != nil && strings.TrimSpace(*user.Email) != "" {
		msg := buildResetEmail(*user.Email, user.FullName, link, expires)
		if err := u.mailer.Send(ctx, msg); err != nil {
			u.logger.Error("password reset email send failed",
				xlogger.Err(err),
				xlogger.String("username", user.Username),
				xlogger.String("email", *user.Email),
			)
		} else {
			emailed = true
		}
	}
	if !emailed {
		// Stdout fallback — keep dev convenience. Token plain text printed
		// only when SMTP isn't wired (config flag IsConfigured = false on
		// startup) so prod logs never carry tokens.
		u.logger.Warn("password reset (NO email sent)",
			xlogger.String("username", user.Username),
			xlogger.String("link", link),
			xlogger.String("expires_at", expires.Format(time.RFC3339)),
		)
	}
	return plain, nil
}

// buildResetEmail formats the magic-link email — plain text + HTML.
func buildResetEmail(to, fullName, link string, expires time.Time) xmail.Message {
	name := fullName
	if strings.TrimSpace(name) == "" {
		name = to
	}
	// Derive FE base từ link. Nếu base là localhost/127.0.0.1 (dev) → Gmail
	// proxy không load được → fallback dùng public prod URL.
	feBase := link
	if i := strings.Index(feBase, "://"); i >= 0 {
		if j := strings.Index(feBase[i+3:], "/"); j >= 0 {
			feBase = feBase[:i+3+j]
		}
	}
	if strings.Contains(feBase, "localhost") || strings.Contains(feBase, "127.0.0.1") {
		feBase = "https://app.example.com"
	}
	logoURL := feBase + "/images/logo.png"
	expiresVN := expires.In(time.FixedZone("ICT", 7*3600)).Format("15:04 02/01/2006")
	subject := "Đặt lại mật khẩu VTV"
	// Plain-text version giữ link đầy đủ cho mail client không hỗ trợ HTML.
	// HTML version KHÔNG hiển thị raw URL — chỉ nút "Đặt mật khẩu mới" — để
	// tránh lộ token khi mail bị forward/screenshot/Gmail search.
	text := "Xin chào " + name + ",\n\n" +
		"Bạn vừa yêu cầu đặt lại mật khẩu cho tài khoản VTV.\n" +
		"Click vào link sau để đặt mật khẩu mới (hết hạn " + expiresVN + "):\n\n" +
		link + "\n\n" +
		"Link chỉ dùng được 1 lần. Nếu bạn không yêu cầu, hãy bỏ qua email này.\n\n" +
		"— VTV"
	// Brand color = teal logo (#42BFC9). Gradient nhẹ tone-on-tone.
	html := `<!DOCTYPE html><html><body style="font-family:-apple-system,'Segoe UI',Roboto,sans-serif;max-width:560px;margin:32px auto;padding:0;color:#0f172a;line-height:1.6;background:#f8fafc">` +
		// Header card với logo
		`<div style="background:linear-gradient(135deg,#5BCAD2 0%,#2DA5AE 100%);padding:28px 28px 24px;border-radius:16px 16px 0 0;color:#fff;text-align:center">` +
		`<img src="` + escapeHTML(logoURL) + `" alt="VTV" width="64" height="64" style="display:inline-block;width:64px;height:64px;border-radius:16px;background:#fff;padding:4px;box-shadow:0 2px 8px rgba(0,0,0,0.08);object-fit:contain">` +
		`<p style="margin:14px 0 0;font-size:11px;letter-spacing:2px;text-transform:uppercase;opacity:0.9;font-weight:600">VTV</p>` +
		`<h2 style="margin:6px 0 0;color:#fff;font-size:24px;font-weight:800;letter-spacing:-0.02em">Đặt lại mật khẩu</h2>` +
		`</div>` +
		// Body
		`<div style="background:#fff;padding:28px;border:1px solid #e2e8f0;border-top:0;border-radius:0 0 16px 16px">` +
		`<p style="margin:0 0 12px">Xin chào <strong>` + escapeHTML(name) + `</strong>,</p>` +
		`<p style="margin:0 0 20px">Bạn vừa yêu cầu đặt lại mật khẩu cho tài khoản <strong>VTV</strong>. Bấm nút bên dưới để đặt mật khẩu mới:</p>` +
		`<p style="margin:0 0 8px;text-align:center"><a href="` + escapeHTML(link) + `" style="display:inline-block;padding:14px 32px;background:linear-gradient(135deg,#5BCAD2 0%,#2DA5AE 100%);color:#fff;text-decoration:none;border-radius:10px;font-weight:600;font-size:15px;box-shadow:0 4px 12px rgba(45,165,174,0.25)">Đặt mật khẩu mới</a></p>` +
		`<p style="margin:8px 0 24px;text-align:center;font-size:12px;color:#64748b">Link hết hạn lúc <strong>` + escapeHTML(expiresVN) + `</strong> và chỉ dùng được 1 lần.</p>` +
		// Security note
		`<div style="padding:14px 16px;background:#f1f5f9;border-radius:10px;border-left:3px solid #2DA5AE">` +
		`<p style="margin:0;font-size:12px;color:#475569"><strong>Lưu ý bảo mật:</strong> Đừng forward email này cho ai khác. Nếu không phải bạn yêu cầu đổi mật khẩu, vui lòng bỏ qua — mật khẩu hiện tại vẫn an toàn.</p>` +
		`</div>` +
		`<hr style="border:none;border-top:1px solid #e2e8f0;margin:24px 0 16px">` +
		`<p style="margin:0;font-size:11px;color:#94a3b8;text-align:center">Email tự động — không trả lời email này.</p>` +
		`</div>` +
		`</body></html>`
	return xmail.Message{
		To:      []string{to},
		Subject: subject,
		Text:    text,
		HTML:    html,
	}
}

func escapeHTML(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;", "'", "&#39;")
	return r.Replace(s)
}

func (u *authUseCase) ResetPassword(ctx context.Context, token, newPassword string) error {
	token = strings.TrimSpace(token)
	if token == "" {
		return xhttp.BadRequestErrorf("token không được để trống")
	}
	if len(newPassword) < minPasswordLen {
		return xhttp.BadRequestErrorf("mật khẩu mới phải có ít nhất %d ký tự", minPasswordLen)
	}
	sum := sha256.Sum256([]byte(token))
	hash := hex.EncodeToString(sum[:])
	return u.tx.Run(ctx, func(ctx context.Context) error {
		user, err := u.users.FindByResetTokenHash(ctx, hash)
		if err != nil {
			return err
		}
		if user == nil {
			return xhttp.BadRequestErrorf("token không hợp lệ hoặc đã hết hạn")
		}
		newHash, err := xcrypto.HashPassword(newPassword)
		if err != nil {
			return err
		}
		user.SetPasswordHash(newHash)
		// Reset qua magic link cũng tính là user đã sở hữu password mới — clear
		// cờ force-change để user không bị khoá trong forced-rotation loop.
		user.ClearForcePasswordChange()
		user.UpdatedBy = user.ID
		if err := u.users.Update(ctx, user); err != nil {
			return err
		}
		if err := u.users.SetResetToken(ctx, user.ID, "", nil); err != nil {
			return err
		}
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: "auth.password_reset_completed", EntityType: "user",
			EntityID: user.ID, EntityName: user.Username,
		})
	})
}

func (u *authUseCase) ChangePassword(ctx context.Context, currentPassword, newPassword string) error {
	if len(newPassword) < minPasswordLen {
		return xhttp.BadRequestErrorf("mật khẩu mới phải có ít nhất %d ký tự", minPasswordLen)
	}
	if newPassword == currentPassword {
		return xhttp.BadRequestErrorf("mật khẩu mới không được trùng mật khẩu hiện tại")
	}
	return u.tx.Run(ctx, func(ctx context.Context) error {
		id := xhttp.UserID(ctx)
		user, err := u.users.GetByID(ctx, id)
		if err != nil {
			return err
		}
		if user == nil {
			return xhttp.UnauthorizedErrorf("không tìm thấy người dùng")
		}
		if !xcrypto.ComparePassword(currentPassword, user.PasswordHash) {
			return xhttp.UnauthorizedErrorf("mật khẩu hiện tại không đúng")
		}
		hash, err := xcrypto.HashPassword(newPassword)
		if err != nil {
			return err
		}
		user.SetPasswordHash(hash)
		// Khi user vừa hoàn tất mandatory first-login rotation, clear cờ ngay
		// — middleware sẽ tự unblock các API khác từ request kế tiếp.
		user.ClearForcePasswordChange()
		user.UpdatedBy = id
		if err := u.users.Update(ctx, user); err != nil {
			return err
		}
		return u.audit.Write(ctx, repository.AuditEntry{
			Action: "user.password_changed", EntityType: "user", EntityID: user.ID, EntityName: user.Username,
			Summary: "đổi mật khẩu",
		})
	})
}
