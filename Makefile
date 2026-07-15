include .env
export

# ── Variables ─────────────────────────────────────────────────────────────────
MIGRATIONS_DIR := migrations
MIGRATE         := migrate
DB_URL          := postgres://$(DATABASE_USER):$(DATABASE_PASSWORD)@$(DATABASE_HOST):$(DATABASE_PORT)/$(DATABASE_NAME)?sslmode=$(DATABASE_SSLMODE)

# ── Application ───────────────────────────────────────────────────────────────
.PHONY: run
run:
	go run ./cmd/api

.PHONY: build
build:
	go build -o bin/api ./cmd/api

# ── Migrations ────────────────────────────────────────────────────────────────

## migrate-up: apply all pending migrations
.PHONY: migrate-up
migrate-up:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" up

## migrate-down: roll back the last applied migration
.PHONY: migrate-down
migrate-down:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" down 1

## migrate-drop: drop everything — destructive, dev use only
.PHONY: migrate-drop
migrate-drop:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" drop -f

## migrate-version: print the current schema version
.PHONY: migrate-version
migrate-version:
	$(MIGRATE) -path $(MIGRATIONS_DIR) -database "$(DB_URL)" version

## migrate-create name=<migration_name>: scaffold a new migration pair
.PHONY: migrate-create
migrate-create:
	$(MIGRATE) create -ext sql -dir $(MIGRATIONS_DIR) -seq $(name)

# ── Code quality ──────────────────────────────────────────────────────────────
.PHONY: vet
vet:
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy
