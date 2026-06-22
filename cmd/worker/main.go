// Command worker runs background jobs: cron (robfig/cron) + the asynq queue
// consumer. (Phase 1: scaffolding — registers no jobs/tasks yet.)
package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"syscall"

	"github.com/robfig/cron/v3"

	"vtv.vn/backend/internal/config"
	"vtv.vn/backend/pkg/xlogger"
	"vtv.vn/backend/pkg/xpostgres"
	"vtv.vn/backend/pkg/xqueue"
)

// Version is set at build time.
var Version = "dev"

func main() {
	env := flag.String("env", envOr("APP_ENV", "dev"), "environment")
	flag.Parse()

	cfg, err := config.Load(*env)
	if err != nil {
		fatal("load config: " + err.Error())
	}
	logger, err := xlogger.New(&cfg.Logger)
	if err != nil {
		fatal("init logger: " + err.Error())
	}
	logger.Info("starting worker", xlogger.String("env", cfg.Env), xlogger.String("version", Version))

	db, err := xpostgres.NewDB(context.Background(), &cfg.Postgres)
	if err != nil {
		logger.Fatal("connect postgres", xlogger.Err(err))
	}
	defer func() {
		if sqlDB, derr := db.DB(); derr == nil {
			_ = sqlDB.Close()
		}
	}()

	// ── cron ─────────────────────────────────────────────────────────────────
	c := cron.New(cron.WithSeconds(), cron.WithChain(cron.Recover(cron.DefaultLogger)))
	// TODO: register jobs, e.g.
	//   c.AddJob("0 0 2 * * *", jobs.NewReconcileJob(...))
	c.Start()
	defer c.Stop()

	// ── asynq ────────────────────────────────────────────────────────────────
	srv := xqueue.NewServer(&cfg.Queue)
	mux := xqueue.NewMux()
	// TODO: mux.HandleFunc("import:video", handler.HandleImportVideo)
	go func() {
		if rerr := srv.Run(mux); rerr != nil {
			logger.Error("asynq server stopped", xlogger.Err(rerr))
		}
	}()
	defer srv.Shutdown()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit
	logger.Info("worker shutting down…")
}

func envOr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
func fatal(msg string) {
	_, _ = os.Stderr.WriteString("FATAL: " + msg + "\n")
	os.Exit(1)
}
