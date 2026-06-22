// Package config loads layered YAML config (base.yaml + <env>.yaml) via Viper,
// with APP_* env-var overrides, and validates it.
package config

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"

	"vtv.vn/backend/pkg/xauth"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xpostgres"
	"vtv.vn/backend/pkg/xqueue"
	"vtv.vn/backend/pkg/xredis"
	"vtv.vn/backend/pkg/xstorage"
)

// Config is the full application configuration.
type Config struct {
	Env            string             `mapstructure:"env"`
	HTTP           HTTPConfig         `mapstructure:"http"`
	Postgres       xpostgres.Config   `mapstructure:"postgres"`
	Redis          xredis.Config      `mapstructure:"redis"`
	Auth           AuthConfig         `mapstructure:"auth"`
	InternalAPIKey string             `mapstructure:"internal_api_key"`
	Storage        xstorage.Config    `mapstructure:"storage"`
	Queue          xqueue.Config      `mapstructure:"queue"`
	Logger         xlogger.Config     `mapstructure:"logger"`
	Integrations   IntegrationsConfig `mapstructure:"integrations"`
}

// HTTPConfig holds HTTP server settings.
type HTTPConfig struct {
	Host             string        `mapstructure:"host"`
	Port             int           `mapstructure:"port"`
	BaseURL          string        `mapstructure:"base_url"`
	ReadTimeout      time.Duration `mapstructure:"read_timeout"`
	WriteTimeout     time.Duration `mapstructure:"write_timeout"`
	CORSAllowOrigins []string      `mapstructure:"cors_allow_origins"`
	TrustedProxies   []string      `mapstructure:"trusted_proxies"`
}

// Addr returns host:port.
func (h HTTPConfig) Addr() string { return fmt.Sprintf("%s:%d", h.Host, h.Port) }

// AuthConfig holds auth/JWT/cookie settings.
type AuthConfig struct {
	xauth.Config    `mapstructure:",squash"`
	CookieName      string        `mapstructure:"cookie_name"`
	CookieSecure    bool          `mapstructure:"cookie_secure"`
	CookieDomain    string        `mapstructure:"cookie_domain"`
	ProfileCacheTTL time.Duration `mapstructure:"profile_cache_ttl"`
}

// IntegrationsConfig holds optional external integrations.
type IntegrationsConfig struct {
	SMTP SMTPConfig `mapstructure:"smtp"`
}

// SMTPConfig holds outbound email settings.
type SMTPConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Username string `mapstructure:"username"`
	Password string `mapstructure:"password"`
	From     string `mapstructure:"from"`
}

// Load reads config/base.yaml then config/<env>.yaml (override), then env-var
// overrides (APP_*, nested via `_`), unmarshals and validates.
func Load(env string) (*Config, error) {
	if env == "" {
		env = "dev"
	}
	v := viper.New()
	v.SetConfigType("yaml")
	v.AddConfigPath("config")
	v.AddConfigPath(".")

	var notFound viper.ConfigFileNotFoundError

	v.SetConfigName("base")
	if err := v.ReadInConfig(); err != nil && !errors.As(err, &notFound) {
		return nil, fmt.Errorf("read base.yaml: %w", err)
	}
	v.SetConfigName(env)
	if err := v.MergeInConfig(); err != nil && !errors.As(err, &notFound) {
		return nil, fmt.Errorf("merge %s.yaml: %w", env, err)
	}

	v.SetEnvPrefix("APP")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}
	if cfg.Env == "" {
		cfg.Env = env
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Validate checks required fields.
func (c *Config) Validate() error {
	var errs []string
	if c.HTTP.Port == 0 {
		errs = append(errs, "http.port is required")
	}
	if c.Postgres.DSN == "" {
		errs = append(errs, "postgres.dsn is required")
	}
	if c.Redis.Addr == "" {
		errs = append(errs, "redis.addr is required")
	}
	if len(c.Auth.Secret) < 32 {
		errs = append(errs, "auth.jwt_secret must be at least 32 bytes")
	}
	if c.InternalAPIKey == "" {
		errs = append(errs, "internal_api_key is required")
	}
	for _, o := range c.HTTP.CORSAllowOrigins {
		if o == "*" || o == "" {
			errs = append(errs, `http.cors_allow_origins must list explicit origins (no "*"/empty — credentials are sent)`)
			break
		}
	}
	if len(errs) > 0 {
		return errors.New("invalid config: " + strings.Join(errs, "; "))
	}
	return nil
}
