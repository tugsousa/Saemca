# ── SALUSDOMI Makefile ────────────────────────────────────────────────────────
# Run `make help` to see all available commands.
# All commands assume you are in the repo root.

BACKEND_DIR  := ./backend
MIGRATE_URL  := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=disable
MIGRATIONS   := ./infra/migrations

.PHONY: help dev dev-down migrate-up migrate-down migrate-status \
        run build lint test-unit test-integration test-e2e test-all

# ── Help ──────────────────────────────────────────────────────────────────────
help:
	@echo ""
	@echo "  SALUSDOMI — Available commands"
	@echo ""
	@echo "  Infrastructure:"
	@echo "    make dev              Start Docker services (db + minio)"
	@echo "    make dev-down         Stop and remove Docker containers"
	@echo ""
	@echo "  Database:"
	@echo "    make migrate-up       Apply all pending migrations"
	@echo "    make migrate-down     Roll back the last migration"
	@echo "    make migrate-status   Show migration state"
	@echo ""
	@echo "  Development:"
	@echo "    make run              Run the API server (hot-reload via air)"
	@echo "    make build            Build the API and worker binaries"
	@echo "    make lint             Run golangci-lint"
	@echo ""
	@echo "  Testing:"
	@echo "    make test-unit        Run unit tests (no Docker needed)"
	@echo "    make test-integration Run integration tests (requires Docker)"
	@echo "    make test-e2e         Run Playwright end-to-end tests"
	@echo "    make test-all         Run all three test suites"
	@echo ""

# ── Infrastructure ────────────────────────────────────────────────────────────
dev:
	docker compose -f infra/docker-compose.yml up -d
	@echo "  DB:    localhost:5432"
	@echo "  MinIO: localhost:9000 (API) | localhost:9001 (console)"

dev-down:
	docker compose -f infra/docker-compose.yml down

# ── Database Migrations ───────────────────────────────────────────────────────
# Requires: golang-migrate CLI — install with:
#   go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest

migrate-up:
	migrate -path $(MIGRATIONS) -database "$(MIGRATE_URL)" up

migrate-down:
	migrate -path $(MIGRATIONS) -database "$(MIGRATE_URL)" down 1

migrate-status:
	migrate -path $(MIGRATIONS) -database "$(MIGRATE_URL)" version

# ── Development ───────────────────────────────────────────────────────────────
# Requires: air — install with: go install github.com/air-verse/air@latest
run:
	cd $(BACKEND_DIR) && air

build:
	cd $(BACKEND_DIR) && go build -o bin/api  ./cmd/api
	cd $(BACKEND_DIR) && go build -o bin/worker ./cmd/worker
	@echo "Binaries built: backend/bin/api, backend/bin/worker"

lint:
	cd $(BACKEND_DIR) && golangci-lint run ./...

# ── Testing ───────────────────────────────────────────────────────────────────
test-unit:
	cd $(BACKEND_DIR) && go test -v -run "^Test(Register|Login|RefreshToken|Logout)" ./internal/...

test-integration:
	cd $(BACKEND_DIR) && go test -v -run "^TestRepository" ./internal/...

test-e2e:
	cd e2e && npx playwright test

test-all: test-unit test-integration test-e2e
