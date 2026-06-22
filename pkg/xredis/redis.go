// Package xredis wraps go-redis client construction.
package xredis

import (
	"context"

	"github.com/redis/go-redis/v9"
)

// Config for the Redis client.
type Config struct {
	Addr     string `mapstructure:"addr"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

// New builds and pings a Redis client.
func New(ctx context.Context, cfg *Config) (*redis.Client, error) {
	c := redis.NewClient(&redis.Options{
		Addr:     cfg.Addr,
		Password: cfg.Password,
		DB:       cfg.DB,
	})
	if err := c.Ping(ctx).Err(); err != nil {
		return nil, err
	}
	return c, nil
}
