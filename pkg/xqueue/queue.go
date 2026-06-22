// Package xqueue wraps the asynq client/server (Redis-backed background jobs).
//
// TODO(phase-1): flesh out task registration. This defines config + the small
// surface the worker needs so wiring compiles.
package xqueue

import "github.com/hibiken/asynq"

// Config for the queue.
type Config struct {
	RedisAddr   string `mapstructure:"redis_addr"`
	RedisDB     int    `mapstructure:"redis_db"`
	Concurrency int    `mapstructure:"concurrency"`
}

// NewClient builds an asynq client for enqueuing tasks.
func NewClient(cfg *Config) *asynq.Client {
	return asynq.NewClient(asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: cfg.RedisDB})
}

// NewServer builds an asynq server for processing tasks.
func NewServer(cfg *Config) *asynq.Server {
	c := cfg.Concurrency
	if c <= 0 {
		c = 10
	}
	return asynq.NewServer(
		asynq.RedisClientOpt{Addr: cfg.RedisAddr, DB: cfg.RedisDB},
		asynq.Config{Concurrency: c},
	)
}

// NewMux builds an empty handler mux; register task handlers on it.
func NewMux() *asynq.ServeMux { return asynq.NewServeMux() }
