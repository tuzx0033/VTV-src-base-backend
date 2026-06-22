// Package handler — storage.go exposes server-side proxy endpoints over
// xstorage (MinIO local / R2 prod). Why proxy instead of presigned URLs:
//
//  1. MinIO's presigned URLs embed the configured endpoint host. In prod we
//     run MinIO on 127.0.0.1:9000 (not reachable from browsers), so signed
//     URLs would be broken without extra nginx/MINIO_SERVER_URL plumbing.
//  2. Browser→MinIO direct uploads require CORS config on the bucket — extra
//     ops surface to manage per environment.
//
// Proxy through BE eats double bandwidth but works identically dev↔prod and
// gates uploads through our normal auth middleware.
//
// FE flow (legacy two-step compat):
//
//	POST /api/v1/storage/uploads/request-url  body {name,size,contentType}
//	     → {uploadURL: ".../uploads/direct?token=<HMAC>", objectPath}
//	PUT  <uploadURL>  body=<file bytes>          # Content-Type must match
//	GET  /api/v1/storage/<objectPath>            # streams via GetObject
package handler

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"vtv.vn/backend/pkg/xhttp"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xstorage"
)

// StorageHandler wraps an xstorage provider + size cap + HMAC sign secret.
type StorageHandler struct {
	logger         *xlogger.Logger
	storage        xstorage.Provider
	maxUploadBytes int64
	hmacSecret     []byte
	publicBaseURL  string // e.g. https://api.example.com — used to build absolute uploadURL
}

// NewStorageHandler returns nil if storage is nil (config disabled). Caller
// must check before registering routes.
func NewStorageHandler(logger *xlogger.Logger, storage xstorage.Provider, maxBytes int64, hmacSecret []byte, publicBaseURL string) *StorageHandler {
	if storage == nil {
		return nil
	}
	if maxBytes <= 0 {
		maxBytes = 10 * 1024 * 1024
	}
	return &StorageHandler{
		logger:         logger,
		storage:        storage,
		maxUploadBytes: maxBytes,
		hmacSecret:     hmacSecret,
		publicBaseURL:  strings.TrimRight(publicBaseURL, "/"),
	}
}

type requestUploadReq struct {
	Name        string `json:"name"`
	Size        int64  `json:"size"`
	ContentType string `json:"contentType"`
}

type requestUploadResp struct {
	UploadURL  string `json:"uploadURL"`
	ObjectPath string `json:"objectPath"`
	ExpiresIn  int    `json:"expiresIn"` // seconds
}

// allowedImageTypes whitelists browser-driven uploads. Non-image content
// (PDFs, CSV exports) should go through dedicated server-side endpoints that
// own their validation.
var allowedImageTypes = map[string]string{
	"image/jpeg": ".jpg",
	"image/jpg":  ".jpg",
	"image/png":  ".png",
	"image/webp": ".webp",
	"image/gif":  ".gif",
}

// signUploadToken HMAC-signs (key | ct | exp) so PUT can be verified without
// holding server-side state. Token format: "<exp_unix>.<hex(hmac256(key|ct|exp))>".
func (h *StorageHandler) signUploadToken(key, contentType string, exp time.Time) string {
	expUnix := exp.Unix()
	mac := hmac.New(sha256.New, h.hmacSecret)
	fmt.Fprintf(mac, "%s|%s|%d", key, contentType, expUnix)
	return fmt.Sprintf("%d.%s", expUnix, hex.EncodeToString(mac.Sum(nil)))
}

// verifyUploadToken returns nil if the token matches and hasn't expired.
func (h *StorageHandler) verifyUploadToken(key, contentType, token string) error {
	parts := strings.SplitN(token, ".", 2)
	if len(parts) != 2 {
		return errors.New("malformed token")
	}
	expUnix, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return errors.New("malformed token exp")
	}
	if time.Now().Unix() > expUnix {
		return errors.New("token expired")
	}
	expected := h.signUploadToken(key, contentType, time.Unix(expUnix, 0))
	if !hmac.Equal([]byte(expected), []byte(token)) {
		return errors.New("token mismatch")
	}
	return nil
}

// RequestUploadURL handles POST /storage/uploads/request-url. Returns a
// signed BE upload URL — FE then PUTs the file bytes there, BE streams to
// MinIO. Keeps the existing two-step FE contract intact.
func (h *StorageHandler) RequestUploadURL(c echo.Context) error {
	var req requestUploadReq
	if err := c.Bind(&req); err != nil {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("body không hợp lệ"))
	}
	if req.Size <= 0 {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("size phải lớn hơn 0"))
	}
	if req.Size > h.maxUploadBytes {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("file vượt giới hạn %d bytes", h.maxUploadBytes))
	}
	ct := strings.ToLower(strings.TrimSpace(req.ContentType))
	ext, ok := allowedImageTypes[ct]
	if !ok {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("content type không hỗ trợ: %s", ct))
	}
	// Prefer extension from filename when safe + known, else derive from MIME.
	if cleanExt := strings.ToLower(path.Ext(req.Name)); cleanExt == ".jpg" || cleanExt == ".jpeg" || cleanExt == ".png" || cleanExt == ".webp" || cleanExt == ".gif" {
		ext = cleanExt
		if ext == ".jpeg" {
			ext = ".jpg"
		}
	}

	now := time.Now().UTC()
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return xhttp.AppErrorResponse(c, xhttp.InternalErrorf("không tạo được khoá upload").Wrap(err))
	}
	key := strings.Join([]string{
		"uploads",
		now.Format("2006"),
		now.Format("01"),
		hex.EncodeToString(random) + ext,
	}, "/")

	ttl := 15 * time.Minute
	exp := now.Add(ttl)
	token := h.signUploadToken(key, ct, exp)

	// uploadURL: relative path is fine — FE prepends its API base. Browsers
	// don't need an absolute URL here, and keeping it relative avoids hard-
	// coding the BE host (which differs dev↔prod).
	uploadURL := fmt.Sprintf("/api/v1/storage/uploads/direct?key=%s&ct=%s&token=%s",
		key, ct, token)
	if h.publicBaseURL != "" {
		uploadURL = h.publicBaseURL + uploadURL
	}

	return xhttp.SuccessResponse(c, requestUploadResp{
		UploadURL:  uploadURL,
		ObjectPath: "/" + key,
		ExpiresIn:  int(ttl.Seconds()),
	})
}

// DirectUpload handles PUT /storage/uploads/direct?key=...&ct=...&token=...
// FE PUTs file bytes here (with matching Content-Type header). BE verifies
// the HMAC token, then streams the body straight to MinIO via xstorage.Upload.
// This endpoint is intentionally NOT behind the JWT middleware — the HMAC
// token IS the capability, valid 15m, scoped to one specific objectPath.
func (h *StorageHandler) DirectUpload(c echo.Context) error {
	key := c.QueryParam("key")
	ct := c.QueryParam("ct")
	token := c.QueryParam("token")
	if key == "" || ct == "" || token == "" {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("thiếu key/ct/token"))
	}
	if err := h.verifyUploadToken(key, ct, token); err != nil {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("token không hợp lệ: %s", err.Error()))
	}
	// Reject Content-Type mismatch — token only signs the announced type, so
	// allowing arbitrary bytes here would let an attacker upload .exe under
	// the .jpg key generated for them.
	gotCT := strings.ToLower(strings.TrimSpace(c.Request().Header.Get(echo.HeaderContentType)))
	// Browsers may suffix charset etc.; compare prefix.
	if !strings.HasPrefix(gotCT, ct) {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("content-type không khớp token: %s ≠ %s", gotCT, ct))
	}
	// Enforce size cap on the way in — Echo's BodyLimit middleware is global,
	// but we want a friendlier error here too.
	req := c.Request()
	if req.ContentLength > h.maxUploadBytes {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("file vượt giới hạn %d bytes", h.maxUploadBytes))
	}
	defer req.Body.Close()
	body := http.MaxBytesReader(c.Response(), req.Body, h.maxUploadBytes+1024)
	_, err := h.storage.Upload(c.Request().Context(), xstorage.UploadInput{
		Key:         key,
		Body:        body,
		Size:        req.ContentLength,
		ContentType: ct,
	})
	if err != nil {
		return xhttp.AppErrorResponse(c, xhttp.InternalErrorf("upload thất bại").Wrap(err))
	}
	return c.NoContent(http.StatusNoContent)
}

// Serve handles GET /storage/* — streams the object from MinIO through the
// API. Avoids exposing MinIO's internal endpoint to browsers and works
// uniformly across dev/prod (no nginx /storage proxy needed).
func (h *StorageHandler) Serve(c echo.Context) error {
	raw := c.Param("*")
	if raw == "" {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("thiếu object path"))
	}
	clean := path.Clean("/" + raw)
	// Reject path traversal — path.Clean strips `..` but be explicit so a
	// curious caller can't break out of the bucket prefix.
	if strings.Contains(clean, "/../") || strings.HasPrefix(clean, "/..") {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("object path không hợp lệ"))
	}
	key := strings.TrimPrefix(clean, "/")
	if key == "" {
		return xhttp.AppErrorResponse(c, xhttp.BadRequestErrorf("thiếu object path"))
	}
	obj, err := h.storage.GetObject(c.Request().Context(), key)
	if err != nil {
		// MinIO returns various "not found" shapes; treat as 404.
		return c.NoContent(http.StatusNotFound)
	}
	defer obj.Body.Close()
	resp := c.Response()
	if obj.ContentType != "" {
		resp.Header().Set(echo.HeaderContentType, obj.ContentType)
	}
	if obj.Size > 0 {
		resp.Header().Set(echo.HeaderContentLength, strconv.FormatInt(obj.Size, 10))
	}
	if obj.ETag != "" {
		resp.Header().Set("ETag", obj.ETag)
	}
	// 1h cache — uploads use random suffix so URLs don't collide. Browsers
	// + CDNs can cache aggressively.
	resp.Header().Set(echo.HeaderCacheControl, "public, max-age=3600")
	if !obj.LastModified.IsZero() {
		resp.Header().Set("Last-Modified", obj.LastModified.UTC().Format(http.TimeFormat))
	}
	resp.WriteHeader(http.StatusOK)
	_, _ = io.Copy(resp, obj.Body)
	return nil
}
