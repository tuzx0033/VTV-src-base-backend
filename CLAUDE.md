# CLAUDE.md — backend skeleton · conventions & rules

> Go backend skeleton (Clean Architecture). Foundation only: auth, users, RBAC/permissions, notifications/announcements, settings, audit log, storage + shared `pkg/x*` infrastructure. No business domain — add yours on top of this structure.

---

## 0. Architecture decisions
| Item | Decision |
|---|---|
| Module path | `vtv.vn/backend`. goimports local-prefix = `vtv.vn`. |
| Go | 1.24+ |
| Web framework | Echo v4 (`labstack/echo/v4`) |
| ORM | **GORM** (`gorm.io/gorm` + `gorm.io/driver/postgres`). Domain models are **pure** (no GORM tags) — repository maps `domain ↔ GORM row` (see §3). |
| Migrations | `golang-migrate/v4` (file SQL up/down) in `migrations/`, named `NNNN_snake_desc.{up,down}.sql` (4 digits). No GORM AutoMigrate in prod. |
| Config | Viper + YAML 2 layers: `config/base.yaml` + `config/<env>.yaml` (override). Select env via flag `-env dev\|staging\|prod` or `APP_ENV`. Env vars `APP_*` override (nested via `_`). `Config.Validate()` at startup. Commit only `*.example.yaml`; gitignore real files. |
| Auth | JWT HS256 (`golang-jwt/v5`), local validate + profile cache + token blacklist in Redis. httpOnly cookie. No hardcoded secrets. RBAC via permission strings, middleware `RequirePermission(...)` / `RequireRole(...)`. |
| Logging | zerolog via `pkg/xlogger`. Structured fields. Never log secrets/tokens/passwords. |
| Validation | `go-playground/validator/v10` + `creasty/defaults`, via `xhttp.ReadAndValidateRequest(c, &req)`. Validate on DTO (tags), not in handlers. |
| Errors | `pkg/xhttp.AppError{Code,Message,Field,Status,Err}` + constructors (`NotFoundErrorf`, `BadRequestErrorf`, `ForbiddenErrorf`, …). HTTP renders `application/problem+json` (RFC 7807) with a stable `code`. Never construct `AppError{}` literals. Never leak internals on 5xx. |
| Background jobs | `robfig/cron/v3` for periodic; `hibiken/asynq` (Redis) for queued jobs. Run in `cmd/worker`. |
| Storage | S3-compatible via `pkg/xstorage` (MinIO local / R2 prod). Auth + type/size limits enforced server-side. |
| Money | Use `numeric` columns ↔ `shopspring/decimal.Decimal`. **Never `float64` for money.** |
| Time | DB `timestamptz`, store & compare in **UTC**; render local time only at the display edge (`pkg/xtime`). Never store Unix-int / local time. |
| OpenAPI | swaggo annotations on handlers → `swag init` (`make gen-swagger`). |

---

## 1. Layout
```
cmd/<bin>/main.go        binaries: api, worker, migrate, seed-admin
internal/
  config/                Config struct + Load(env) (Viper) + Validate()
  di/wire.go             composition root — manual DI. NewAppContainer(...)
  domain/
    model/               pure entities (struct + methods), NO GORM tags
    repository/           port interfaces: <Aggregate>Repository
    service/             cross-aggregate / external service interfaces
    usecase/             use-case interfaces + input structs
    port/                infra interfaces used by domain
    consts/              domain constants (roles, permissions, table names, audit verbs)
  repository/             Postgres (GORM) impls; private row structs + map in/out
  usecase/               use-case impls
  server/
    http/
      handler/           1 struct/resource; routes registered in handler.go
      middleware/         Auth, Internal, RequestLogging, rate-limit
      dto/               request/response DTOs + ToXxxResponse mappers
    cron/                cron Job interface + Server
    worker/              asynq server + task handlers
pkg/                     shared infra, prefix `x`:
  xhttp xlogger xpostgres xredis xstorage xqueue xnotify xmail xtime xauth xexcel xcrypto xratelimit
migrations/  config/  docs/
```

---

## 2. Architecture rules (HARD)
1. **Dependency direction:** `domain → ∅` · `internal/{repository,usecase} → domain` · `internal/server → domain (usecase iface)` · `internal/di → everything` · `cmd → di + config + pkg`. `domain/*` must NOT import `internal/{repository,usecase,server,di}` nor infra `pkg/x*`. goimports order: stdlib → third-party → `vtv.vn/backend/...`.
2. **Aggregates own state-changing logic.** Invariants live on entities (`user.Lock()`); use cases orchestrate: load → call entity method → persist → side-effect. Repositories only persist.
3. **One transaction per write use case.** `xpostgres.TxRunner.Run(ctx, fn)` — all DB changes + audit-log write in one transaction. Repos take `*gorm.DB` from ctx.
4. **Errors carry stable codes** (`AppError.Code` string is part of the contract). FE branches on `code`, not `message`.
5. **Time** `timestamptz` UTC; render local at the edge. **Money** `numeric` ↔ `decimal.Decimal`.
6. **Validate on DTO** (tags), not in handlers. Handler: parse → call use case → render.
7. **Never log secrets** — `password/token/Authorization/secret` redacted.
8. **Every mutation goes through audit** — write `audit_log` (entity, verb, before→after) in the same transaction. Never log raw request bodies or passwords.
9. **Every mutable entity** has `created_at/updated_at timestamptz`, `created_by/updated_by bigint` (+ `deleted_at` if soft-delete). Embed `model.AuditFields`.
10. **Typed responses.** Every response is a DTO; OpenAPI is the source of truth. No ad-hoc `interface{}`/`map`. Business logic on the backend, not the frontend.

## 3. Domain ↔ persistence mapping
- `internal/domain/model/<aggregate>.go`: pure struct (standard Go types + `decimal.Decimal` for money), invariant methods, NO DB/JSON tags.
- `internal/repository/<aggregate>.go`: private `<aggregate>Row` struct with GORM tags + `TableName()`; `toDomain()` / `fromDomain()`. Row structs never leave `internal/repository`. Table names = constants in `internal/domain/consts`.
- Repository interfaces in `internal/domain/repository`; impls in `internal/repository`.

## 4. HTTP / DTO / errors
- Handler: 1 struct/resource, `New<Resource>Handler(logger, <uc iface>)`. Routes registered in `handler.go` via `Handler.RegisterRoutes(e)`. Groups: `/api/v1/...` + `authMW` + `permMW`; `/internal/api/v1/...` + `internalMW` (API key).
- Request DTO: `json/query/param` binding + `validate:"..."` + `default:"..."`. Read via `xhttp.ReadAndValidateRequest(c, &req)`.
- Response: `xhttp.SuccessResponse` / `CreatedResponse`; list `{data:[...], meta:{page,limit,total,totalPages}}`; error `xhttp.AppErrorResponse` (RFC 7807).

## 5. Testing (target ≥ 70% statements; 100% for state machines + auth + money formulas)
| Layer | Stack |
|---|---|
| Domain (entity, invariants, formulas) | `go test` table-driven, no DB |
| Use case | fake repo (interfaces) |
| Repository | `testcontainers-go` Postgres, build tag `//go:build integration` |
| HTTP | `testcontainers-go` Postgres+Redis, build tag `//go:build e2e` |

## 6. Linting / format
- `.golangci.yml` enables a broad linter set; `goimports.local-prefixes: vtv.vn`.
- `make fmt` = `gofmt` + `goimports -local vtv.vn -w .`. Run `make lint` before commit.

## 7. Don't
- Don't import `gorm.io/gorm` in `internal/domain/...`.
- Don't put validation in handlers — DTO tags + `pkg/xhttp`.
- Don't log `Authorization`/`password`/`token`/secrets.
- Don't skip audit on mutations.
- Don't hard-delete cascade; don't auto-migrate in prod.
- Don't use `float64` for money.
- Don't construct `AppError{}` literals — use constructors.
- Don't commit real config files (only `*.example.yaml`).

## 8. Pointers
- API base: `${HTTP_BASE_URL}/api/v1` · Swagger: `/swagger/index.html` (`make gen-swagger`) · health: `/healthz`
- First admin: `make seed-admin` (or `go run ./cmd/seed-admin -env dev`).
