package xstorage

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type minioProvider struct {
	client    *minio.Client
	bucket    string
	publicURL string        // base URL for public objects, e.g. http://host:9000/bucket
	getTTL    time.Duration // presigned-GET expiry
}

func newMinIOProvider(ctx context.Context, cfg *Config) (Provider, error) {
	cl, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, fmt.Errorf("xstorage: minio client: %w", err)
	}
	ok, err := cl.BucketExists(ctx, cfg.Bucket)
	if err != nil {
		return nil, fmt.Errorf("xstorage: bucket check: %w", err)
	}
	if !ok {
		if mkErr := cl.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); mkErr != nil {
			return nil, fmt.Errorf("xstorage: create bucket %q: %w", cfg.Bucket, mkErr)
		}
	}
	getTTL := cfg.PresignGetTTL
	if getTTL <= 0 {
		getTTL = 15 * time.Minute
	}
	return &minioProvider{
		client:    cl,
		bucket:    cfg.Bucket,
		publicURL: strings.TrimRight(cfg.PublicBaseURL, "/"),
		getTTL:    getTTL,
	}, nil
}

func (p *minioProvider) Upload(ctx context.Context, in UploadInput) (string, error) {
	key := strings.TrimLeft(in.Key, "/")
	_, err := p.client.PutObject(ctx, p.bucket, key, in.Body, in.Size, minio.PutObjectOptions{
		ContentType: in.ContentType,
	})
	if err != nil {
		return "", fmt.Errorf("xstorage: upload %q: %w", key, err)
	}
	if p.publicURL != "" {
		return p.publicURL + "/" + key, nil
	}
	return key, nil
}

func (p *minioProvider) PresignGet(ctx context.Context, key string) (string, error) {
	key = strings.TrimLeft(key, "/")
	u, err := p.client.PresignedGetObject(ctx, p.bucket, key, p.getTTL, url.Values{})
	if err != nil {
		return "", fmt.Errorf("xstorage: presign %q: %w", key, err)
	}
	return u.String(), nil
}

// GetObject reads an object server-side so the API can stream-through to the
// browser. This avoids exposing MinIO's internal endpoint or wrestling with
// presigned-URL host mismatches across dev/prod.
func (p *minioProvider) GetObject(ctx context.Context, key string) (*GetObjectResult, error) {
	key = strings.TrimLeft(key, "/")
	obj, err := p.client.GetObject(ctx, p.bucket, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("xstorage: get-object %q: %w", key, err)
	}
	stat, err := obj.Stat()
	if err != nil {
		_ = obj.Close()
		return nil, fmt.Errorf("xstorage: stat %q: %w", key, err)
	}
	return &GetObjectResult{
		Body:         obj,
		ContentType:  stat.ContentType,
		Size:         stat.Size,
		LastModified: stat.LastModified,
		ETag:         stat.ETag,
	}, nil
}

// PresignPut returns a temporary PUT URL the browser can upload to directly.
// The client must send the request with the same Content-Type used here, else
// the signature will not match.
func (p *minioProvider) PresignPut(ctx context.Context, in PresignPutInput) (string, error) {
	key := strings.TrimLeft(in.Key, "/")
	ttl := in.TTL
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	u, err := p.client.PresignedPutObject(ctx, p.bucket, key, ttl)
	if err != nil {
		return "", fmt.Errorf("xstorage: presign-put %q: %w", key, err)
	}
	return u.String(), nil
}

func (p *minioProvider) Delete(ctx context.Context, key string) error {
	key = strings.TrimLeft(key, "/")
	if err := p.client.RemoveObject(ctx, p.bucket, key, minio.RemoveObjectOptions{}); err != nil {
		return fmt.Errorf("xstorage: delete %q: %w", key, err)
	}
	return nil
}
