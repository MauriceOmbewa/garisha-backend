include .env
export

# ── Variables ─────────────────────────────────────────────────────────────────
MIGRATIONS_DIR := migrations
MIGRATE        := migrate
DB_URL         := postgres://$(DATABASE_USER):$(DATABASE_PASSWORD)@$(DATABASE_HOST):$(DATABASE_PORT)/$(DATABASE_NAME)?sslmode=$(DATABASE_SSLMODE)

APP_NAME := garisha-backend
IMAGE    := $(APP_NAME)
VERSION  ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

# ── Application ───────────────────────────────────────────────────────────────

.PHONY: run
run:
	go run ./cmd/api

.PHONY: build
build:
	CGO_ENABLED=0 go build \
	  -ldflags="-s -w" \
	  -trimpath \
	  -o bin/api \
	  ./cmd/api

# ── Code quality ──────────────────────────────────────────────────────────────

.PHONY: vet
vet:
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: lint
lint:
	golangci-lint run ./...

# ── Tests ─────────────────────────────────────────────────────────────────────

.PHONY: test
test:
	go test -race -count=1 ./...

.PHONY: test-cover
test-cover:
	go test -race -count=1 -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

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

# ── Docker ────────────────────────────────────────────────────────────────────

## docker-build: build the production Docker image
.PHONY: docker-build
docker-build:
	docker build \
	  --build-arg VERSION=$(VERSION) \
	  -t $(IMAGE):$(VERSION) \
	  -t $(IMAGE):latest \
	  .

## docker-up: start all services with MinIO (dev)
.PHONY: docker-up
docker-up:
	docker compose up -d

## docker-down: stop all services
.PHONY: docker-down
docker-down:
	docker compose down

## docker-prod-up: start with production overrides (no MinIO)
.PHONY: docker-prod-up
docker-prod-up:
	docker compose -f docker-compose.yml -f docker-compose.prod.yml up -d

## docker-logs: tail api container logs
.PHONY: docker-logs
docker-logs:
	docker compose logs -f api

## docker-ps: list running containers
.PHONY: docker-ps
docker-ps:
	docker compose ps

# ── Utility ───────────────────────────────────────────────────────────────────

## health: curl the running API health endpoint
.PHONY: health
health:
	curl -s http://localhost:$(PORT)/api/v1/health | jq .

## clean: remove build artefacts
.PHONY: clean
clean:
	rm -rf bin/ coverage.out coverage.html

.PHONY: help
help:
	@grep -E '^## ' Makefile | sed 's/## //'
