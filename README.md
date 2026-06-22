# backend — Go service skeleton

A clean backend skeleton (Clean Architecture) to start any new project on.
Stack: **Go 1.24 · Echo v4 · GORM + Postgres 16 · golang-migrate · Redis 7 · asynq · S3-compatible storage (MinIO/R2) · zerolog · Viper · swaggo**.

Module path: `vtv.vn/backend`.

## What's included

Foundation only — no business domain:

- **Auth** — JWT (HS256) login / logout / me / change-password / forgot+reset password, Redis token blacklist, httpOnly cookie.
- **Users** — CRUD, invite, bulk lock/unlock/delete, avatar upload, self-profile.
- **RBAC / permissions** — per-user permission map (JSONB) layered over role defaults, `RequirePermission` / `RequireRole` middleware.
- **Notifications + announcements** — per-user inbox, system announcements, real-time SSE stream via Redis pub/sub.
- **Settings** — key/value system settings.
- **Audit log** — every mutation recorded in one transaction.
- **Storage** — presigned upload URLs + public serve (S3-compatible).
- **Infra packages** (`pkg/x*`) — `xhttp` (AppError + RFC7807, validation), `xauth`, `xpostgres` (GORM + TxRunner), `xredis`, `xstorage`, `xqueue` (asynq), `xnotify`, `xmail`, `xlogger`, `xratelimit`, `xtime`, `xexcel`, `xcrypto`.

## Layout

```
cmd/{api,worker,migrate,seed-admin}
internal/{config,di,domain/{model,repository,service,usecase,port,consts},repository,usecase,server/{http/{handler,middleware,dto},cron,worker}}
pkg/{xhttp,xlogger,xpostgres,xredis,xstorage,xqueue,xnotify,xmail,xtime,xauth,xexcel,xcrypto,xratelimit}
migrations/   config/   docs/
```

See `CLAUDE.md` for architecture rules and conventions.

## Getting started (dev)

```bash
cp config/config.example.yaml config/base.yaml   # then fill in secrets
make init            # install golangci-lint, migrate, swag, air
make compose-up      # postgres + redis + minio
make migrate-up      # apply migrations
make seed-admin      # (optional) create the first admin user — see cmd/seed-admin
make run             # run API (ENV=dev) → http://localhost:8888
```

Health: `GET /healthz`. Swagger (after `make gen-swagger`): `/swagger/index.html`.

## Common commands

| | |
|---|---|
| `make help` | list targets |
| `make fmt` / `make lint` | gofmt+goimports / golangci-lint |
| `make test` / `make test-integration` / `make cover` | unit / integration (testcontainers) / coverage |
| `make migrate-new name=create_widgets` | create a new migration |
| `make build` | build all binaries into `bin/` |
| `make db-reset` | drop + recreate + migrate (DEV ONLY, destructive) |

## Deploy

VPS + Docker Compose (`docker-compose.yml` — postgres/redis/minio/migrate/api/worker). `Dockerfile` is multi-stage (`ARG TARGET=api|worker|migrate|seed-admin`). Reverse proxy / SSL / domain are handled outside compose.
