// Package xpostgres provides the GORM connection pool and a TxRunner that
// scopes a *gorm.DB through context so repositories transparently use the
// surrounding transaction (one transaction per use-case write path).
package xpostgres

import (
	"context"
	"log"
	"os"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// Config for the Postgres pool.
type Config struct {
	DSN             string        `mapstructure:"dsn"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	LogLevel        string        `mapstructure:"log_level"` // silent | error | warn | info
}

// NewDB opens the pool and verifies connectivity.
func NewDB(ctx context.Context, cfg *Config) (*gorm.DB, error) {
	db, err := gorm.Open(postgres.Open(cfg.DSN), &gorm.Config{
		Logger:                 gormLogLevel(cfg.LogLevel),
		SkipDefaultTransaction: true, // we manage transactions explicitly
		NowFunc:                func() time.Time { return time.Now().UTC() },
	})
	if err != nil {
		return nil, err
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	if cfg.MaxOpenConns > 0 {
		sqlDB.SetMaxOpenConns(cfg.MaxOpenConns)
	}
	if cfg.MaxIdleConns > 0 {
		sqlDB.SetMaxIdleConns(cfg.MaxIdleConns)
	}
	if cfg.ConnMaxLifetime > 0 {
		sqlDB.SetConnMaxLifetime(cfg.ConnMaxLifetime)
	}
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, err
	}
	return db, nil
}

func gormLogLevel(s string) logger.Interface {
	lvl := logger.Warn
	switch s {
	case "silent":
		lvl = logger.Silent
	case "error":
		lvl = logger.Error
	case "info":
		lvl = logger.Info
	}
	return logger.New(log.New(os.Stdout, "", log.LstdFlags), logger.Config{
		SlowThreshold:             200 * time.Millisecond,
		LogLevel:                  lvl,
		IgnoreRecordNotFoundError: true, // "not found" is a normal outcome, not a warning
		Colorful:                  false,
	})
}

// ── context-scoped DB ─────────────────────────────────────────────────────────

type ctxKey struct{}

// With returns a context carrying db (used internally by TxRunner).
func With(ctx context.Context, db *gorm.DB) context.Context {
	return context.WithValue(ctx, ctxKey{}, db)
}

// DB returns the transaction-scoped *gorm.DB from ctx, or fallback. Repositories
// MUST obtain their handle via this so writes join the surrounding transaction.
func DB(ctx context.Context, fallback *gorm.DB) *gorm.DB {
	if db, ok := ctx.Value(ctxKey{}).(*gorm.DB); ok {
		return db.WithContext(ctx)
	}
	return fallback.WithContext(ctx)
}

// ── transactions ──────────────────────────────────────────────────────────────

// TxRunner runs a function inside a database transaction.
type TxRunner interface {
	Run(ctx context.Context, fn func(ctx context.Context) error) error
}

type txRunner struct{ db *gorm.DB }

// NewTxRunner builds a TxRunner over the pool.
func NewTxRunner(db *gorm.DB) TxRunner { return &txRunner{db: db} }

func (r *txRunner) Run(ctx context.Context, fn func(ctx context.Context) error) error {
	// If already inside a transaction, reuse it (nested calls share one txn).
	if _, ok := ctx.Value(ctxKey{}).(*gorm.DB); ok {
		return fn(ctx)
	}
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return fn(With(ctx, tx))
	})
}
