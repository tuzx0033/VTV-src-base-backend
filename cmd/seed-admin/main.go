// Command seed-admin creates the initial admin user if none exists.
// Prints a random password ONCE (never hardcode admin/admin123).
// Pass -password to set a known one.
//
//	seed-admin -env dev [-username admin] [-password ...] [-name "..."]
package main

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"flag"
	"fmt"
	"os"

	"vtv.vn/backend/internal/config"
	"vtv.vn/backend/internal/domain/model"
	"vtv.vn/backend/internal/repository"
	"vtv.vn/backend/pkg/xcrypto"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xpostgres"
)

func main() {
	env := flag.String("env", envOr("APP_ENV", "dev"), "environment")
	username := flag.String("username", "admin", "admin username")
	password := flag.String("password", "", "admin password (empty = generate random)")
	fullName := flag.String("name", "Administrator", "admin full name")
	flag.Parse()

	cfg, err := config.Load(*env)
	if err != nil {
		die("load config: %v", err)
	}
	logger, err := xlogger.New(&cfg.Logger)
	if err != nil {
		die("init logger: %v", err)
	}
	db, err := xpostgres.NewDB(context.Background(), &cfg.Postgres)
	if err != nil {
		die("connect postgres: %v", err)
	}
	defer func() {
		if s, e := db.DB(); e == nil {
			_ = s.Close()
		}
	}()

	ctx := context.Background()
	userRepo := repository.NewUserRepository(logger, db)

	if existing, e := userRepo.GetByUsername(ctx, *username); e != nil {
		die("lookup user: %v", e)
	} else if existing != nil {
		fmt.Printf("user %q already exists (id=%d) — nothing to do\n", *username, existing.ID)
		return
	}

	pw := *password
	if pw == "" {
		buf := make([]byte, 12)
		if _, e := rand.Read(buf); e != nil {
			die("rand: %v", e)
		}
		pw = base64.RawURLEncoding.EncodeToString(buf)
	}
	hash, err := xcrypto.HashPassword(pw)
	if err != nil {
		die("hash: %v", err)
	}
	staffCode := "ADMIN"
	u := &model.User{
		Username: *username, PasswordHash: hash, FullName: *fullName,
		Role: model.RoleAdmin, Status: model.UserStatusActive, StaffCode: &staffCode,
	}
	if err := userRepo.Create(ctx, u); err != nil {
		die("create user: %v", err)
	}
	fmt.Printf("✓ created admin user\n  id:       %d\n  username: %s\n  password: %s\n", u.ID, *username, pw)
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", a...)
	os.Exit(1)
}
