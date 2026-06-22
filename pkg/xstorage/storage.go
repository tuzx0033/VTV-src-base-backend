// Package xstorage abstracts S3-compatible object storage (MinIO local / Cloudflare R2 prod).
// Both providers speak the S3 API, so the minio-go client is used for both.
package xstorage

import (
	"context"
	"fmt"
	"io"
	"time"
)

// Config for the storage provider.
type Config struct {
	Provider       string        `mapstructure:"provider"` // minio | r2
	Endpoint       string        `mapstructure:"endpoint"`
	UseSSL         bool          `mapstructure:"use_ssl"`
	AccessKey      string        `mapstructure:"access_key"`
	SecretKey      string        `mapstructure:"secret_key"`
	Bucket         string        `mapstructure:"bucket"`
	PublicBaseURL  string        `mapstructure:"public_base_url"`
	PresignGetTTL  time.Duration `mapstructure:"presign_get_ttl"`
	MaxUploadBytes int64         `mapstructure:"max_upload_bytes"`
}

// UploadInput describes an object to store.
type UploadInput struct {
	Key         string // object path within the bucket
	Body        io.Reader
	Size        int64
	ContentType string
}

// PresignPutInput describes a presigned PUT request.
type PresignPutInput struct {
	Key         string
	ContentType string
	TTL         time.Duration // 0 → 15m default
}

// GetObjectResult is a server-side fetched object — caller MUST close Body.
type GetObjectResult struct {
	Body         io.ReadCloser
	ContentType  string
	Size         int64
	LastModified time.Time
	ETag         string
}

// Provider is the storage abstraction used by use cases.
type Provider interface {
	Upload(ctx context.Context, in UploadInput) (publicURL string, err error)
	GetObject(ctx context.Context, key string) (*GetObjectResult, error)
	PresignGet(ctx context.Context, key string) (url string, err error)
	PresignPut(ctx context.Context, in PresignPutInput) (url string, err error)
	Delete(ctx context.Context, key string) error
}

// New builds a Provider from cfg. Supports "minio" and "r2" (both S3-compatible).
func New(ctx context.Context, cfg *Config) (Provider, error) {
	switch cfg.Provider {
	case "minio", "r2", "s3", "":
		return newMinIOProvider(ctx, cfg)
	default:
		return nil, fmt.Errorf("xstorage: unknown provider %q", cfg.Provider)
	}
}
