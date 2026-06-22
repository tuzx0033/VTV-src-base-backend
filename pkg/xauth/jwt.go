// Package xauth handles JWT signing/verification for the API.
package xauth

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims is the JWT payload.
type Claims struct {
	UserID int64  `json:"uid"`
	Role   string `json:"role"`
	jwt.RegisteredClaims
}

// Config for token issuance.
type Config struct {
	Secret     string        `mapstructure:"jwt_secret"`
	TTL        time.Duration `mapstructure:"jwt_ttl"`
	RefreshTTL time.Duration `mapstructure:"refresh_ttl"`
}

// Manager signs and verifies tokens.
type Manager struct {
	secret []byte
	ttl    time.Duration
}

// NewManager builds a Manager. Returns an error if the secret is too weak.
func NewManager(cfg *Config) (*Manager, error) {
	if len(cfg.Secret) < 32 {
		return nil, errors.New("xauth: jwt_secret must be at least 32 bytes")
	}
	ttl := cfg.TTL
	if ttl == 0 {
		ttl = 168 * time.Hour
	}
	return &Manager{secret: []byte(cfg.Secret), ttl: ttl}, nil
}

// Sign issues a token for the given user.
func (m *Manager) Sign(userID int64, role string) (string, error) {
	jti, err := newJTI()
	if err != nil {
		return "", err
	}
	now := time.Now().UTC()
	claims := Claims{
		UserID: userID,
		Role:   role,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        jti,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(m.ttl)),
			Subject:   "backend",
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func newJTI() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// Verify parses and validates a token, returning its claims.
// Algorithm is pinned to HS256 — `alg=none` and asymmetric tokens are rejected
// (defence against the classic JWT alg-confusion attack).
func (m *Manager) Verify(token string) (*Claims, error) {
	parsed, err := jwt.ParseWithClaims(
		token,
		&Claims{},
		func(t *jwt.Token) (any, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, errors.New("xauth: unexpected signing method")
			}
			return m.secret, nil
		},
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
	)
	if err != nil {
		return nil, err
	}
	claims, ok := parsed.Claims.(*Claims)
	if !ok || !parsed.Valid {
		return nil, errors.New("xauth: invalid token")
	}
	if claims.Subject != "backend" {
		return nil, errors.New("xauth: invalid token subject")
	}
	return claims, nil
}
