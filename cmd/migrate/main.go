// Command migrate runs database migrations from the migrations/ directory.
//
// Usage:
//
//	migrate -env dev up            # apply all up migrations
//	migrate -env dev down 1        # roll back one
//	migrate -env dev version       # print current version
//	migrate -env dev force 3       # set version (recovery)
//
// DSN resolution order: -dsn flag > MIGRATE_DSN env > config.Postgres.DSN.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"vtv.vn/backend/internal/config"
)

func main() {
	env := flag.String("env", envOr("APP_ENV", "dev"), "environment")
	dsn := flag.String("dsn", os.Getenv("MIGRATE_DSN"), "database DSN (overrides env/config)")
	path := flag.String("path", "migrations", "migrations directory")
	flag.Parse()

	resolvedDSN := *dsn
	if resolvedDSN == "" {
		cfg, err := config.Load(*env)
		if err != nil {
			die("load config: %v", err)
		}
		resolvedDSN = cfg.Postgres.DSN
	}

	m, err := migrate.New("file://"+*path, resolvedDSN)
	if err != nil {
		die("init migrate: %v", err)
	}
	defer func() { _, _ = m.Close() }()

	args := flag.Args()
	cmd := "up"
	if len(args) > 0 {
		cmd = args[0]
	}

	switch cmd {
	case "up":
		err = m.Up()
	case "down":
		n := 1
		if len(args) > 1 {
			n, _ = strconv.Atoi(args[1])
		}
		err = m.Steps(-n)
	case "version":
		v, dirty, verr := m.Version()
		if verr != nil {
			die("version: %v", verr)
		}
		fmt.Printf("version=%d dirty=%v\n", v, dirty)
		return
	case "force":
		if len(args) < 2 {
			die("force requires a version")
		}
		v, _ := strconv.Atoi(args[1])
		err = m.Force(v)
	case "drop":
		err = m.Drop()
	default:
		die("unknown command %q (up|down|version|force|drop)", cmd)
	}

	if err != nil && !errors.Is(err, migrate.ErrNoChange) {
		die("%s: %v", cmd, err)
	}
	fmt.Printf("migrate %s: ok\n", cmd)
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
