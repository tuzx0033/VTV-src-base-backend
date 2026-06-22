GOPATH      := $(shell go env GOPATH)
VERSION     := $(shell git describe --tags --always 2>/dev/null || echo dev)
NAME        := backend
ENV         ?= dev
MIGRATE_DSN ?= postgres://app:app@localhost:5432/appdb?sslmode=disable
GOLANGCI    := v1.61.0

.PHONY: init
# install dev tools (golangci-lint, migrate, swag, air)
init:
	@which golangci-lint >/dev/null || go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCI)
	@which migrate >/dev/null || go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	@which swag >/dev/null || go install github.com/swaggo/swag/cmd/swag@latest
	@which air >/dev/null || go install github.com/air-verse/air@latest

.PHONY: tidy
# go mod tidy
tidy:
	@go mod tidy

.PHONY: fmt
# gofmt + goimports
fmt:
	@gofmt -w . && goimports -local vtv.vn -w .

.PHONY: lint
# golangci-lint
lint:
	@golangci-lint run ./...

.PHONY: test
# unit tests
test:
	@go test ./...

.PHONY: test-integration
# integration tests (testcontainers; build tag integration)
test-integration:
	@go test -tags=integration ./...

.PHONY: cover
# unit test coverage report
cover:
	@go test -coverprofile=coverage.out ./... && go tool cover -func=coverage.out

.PHONY: run
# fmt + lint + run api (ENV=dev|staging|prod)
run: fmt lint
	@go run ./cmd/api -env $(ENV)

.PHONY: dev
# air hot-reload api
dev:
	@air -c .air.toml

.PHONY: worker
# run background worker (cron + asynq)
worker:
	@go run ./cmd/worker -env $(ENV)

.PHONY: seed-admin
# create the first admin user (prints a random password once)
seed-admin:
	@go run ./cmd/seed-admin -env $(ENV)

.PHONY: build
# build all binaries into bin/
build:
	@CGO_ENABLED=0 go build -ldflags "-s -w -X main.Version=$(VERSION)" -o bin/api          ./cmd/api
	@CGO_ENABLED=0 go build -ldflags "-s -w"                          -o bin/worker       ./cmd/worker
	@CGO_ENABLED=0 go build -ldflags "-s -w"                          -o bin/migrate      ./cmd/migrate
	@CGO_ENABLED=0 go build -ldflags "-s -w"                          -o bin/seed-admin   ./cmd/seed-admin

.PHONY: migrate-up migrate-down migrate-version migrate-new migrate-force
# apply all up migrations
migrate-up:
	@migrate -path migrations -database "$(MIGRATE_DSN)" up
# rollback one migration
migrate-down:
	@migrate -path migrations -database "$(MIGRATE_DSN)" down 1
# show current migration version
migrate-version:
	@migrate -path migrations -database "$(MIGRATE_DSN)" version
# create a new migration: make migrate-new name=create_widgets
migrate-new:
	@migrate create -ext sql -dir migrations -seq $(name)
# force a version (recovery): make migrate-force v=3
migrate-force:
	@migrate -path migrations -database "$(MIGRATE_DSN)" force $(v)

.PHONY: gen-swagger
# generate OpenAPI spec into docs/swagger/
gen-swagger:
	@swag init -g ./cmd/api/main.go -o ./docs/swagger --parseInternal --outputTypes go,json,yaml

.PHONY: compose-up compose-down compose-logs
# start local infra (postgres, redis, minio) + app
compose-up:
	@docker compose up -d
compose-down:
	@docker compose down
compose-logs:
	@docker compose logs -f

.PHONY: db-reset
# drop + recreate + migrate (DESTRUCTIVE, dev only)
db-reset:
	@migrate -path migrations -database "$(MIGRATE_DSN)" drop -f && $(MAKE) migrate-up

.PHONY: help
help:
	@awk '/^[a-zA-Z\-\_0-9]+:/ { msg=match(lastLine,/^# (.*)/); if (msg){cmd=substr($$1,0,index($$1,":")); printf "\033[36m%-20s\033[0m %s\n",cmd,substr(lastLine,RSTART+2,RLENGTH)} } {lastLine=$$0}' $(MAKEFILE_LIST)

.DEFAULT_GOAL := help
