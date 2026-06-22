package xauth

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

const revokedKeyPrefix = "app:jwt:revoked:"

// Blacklist tracks revoked JWT tokens in Redis until they naturally expire.
type Blacklist struct {
	rdb *redis.Client
}

// NewBlacklist builds a Blacklist backed by the given Redis client.
func NewBlacklist(rdb *redis.Client) *Blacklist {
	return &Blacklist{rdb: rdb}
}

// Revoke marks a token (by its JTI) as revoked until expiresAt.
// If the token is already past expiry, it is a no-op.
func (b *Blacklist) Revoke(ctx context.Context, jti string, expiresAt time.Time) error {
	ttl := time.Until(expiresAt)
	if ttl <= 0 {
		return nil
	}
	return b.rdb.Set(ctx, revokedKeyPrefix+jti, 1, ttl).Err()
}

// IsRevoked returns true if the JTI is present in the blacklist.
func (b *Blacklist) IsRevoked(ctx context.Context, jti string) (bool, error) {
	n, err := b.rdb.Exists(ctx, revokedKeyPrefix+jti).Result()
	if err != nil {
		return false, fmt.Errorf("blacklist check: %w", err)
	}
	return n > 0, nil
}
