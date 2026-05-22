SHELL := /bin/sh

WEB_DIR := web
SERVER_DIR := server

GO := go
PNPM := pnpm
DOCKER := docker

API_URL ?= http://localhost:6680
VITE_API_URL ?= $(API_URL)

# ── Dev Container Compose ────────────────────────────────────────────
# All db / geodata lifecycle is managed through docker compose so the
# Makefile targets work consistently both inside and outside the
# devcontainer.
# Requires the Compose V2 plugin (`docker compose`), not legacy `docker-compose`.
COMPOSE_FILE     ?= .devcontainer/docker-compose.yml
COMPOSE_PROJECT  ?= lumilio-photos-devcontainer
COMPOSE_BIN      := $(shell \
	if $(DOCKER) compose version >/dev/null 2>&1; then \
		printf '%s compose' '$(DOCKER)'; \
	elif command -v docker-compose >/dev/null 2>&1; then \
		printf '%s' docker-compose; \
	fi)
ifeq ($(COMPOSE_BIN),)
  $(error Docker Compose V2 is required. Install docker-compose-plugin, then rebuild the devcontainer or run: apt-get install -y docker-compose-plugin)
endif
COMPOSE          := $(COMPOSE_BIN) -f $(COMPOSE_FILE) -p $(COMPOSE_PROJECT)

# Detect whether we are running inside the dev container so we can
# pick the right DB host / port automatically.
IN_DEV_CONTAINER := $(shell [ -f /.dockerenv ] && echo 1 || echo 0)

# DB connection defaults.
#   Inside devcontainer  →  compose-network address  db:5432
#   On host              →  published port           localhost:5433
DB_SERVICE ?= db
ifeq ($(IN_DEV_CONTAINER),1)
  DB_HOST ?= db
  DB_PORT ?= 5432
else
  DB_HOST ?= localhost
  DB_PORT ?= 5433
endif
DB_NAME     ?= lumiliophotos
DB_USER     ?= postgres
DB_PASSWORD ?= postgres
DB_VOLUME   ?= $(COMPOSE_PROJECT)_db_data

# ── Phony targets ────────────────────────────────────────────────────

.PHONY: setup env-dev env-web-dev \
        db-build db-up db-down db-reset db-logs db-shell db-wait \
        geodata-build geodata-import geodata-reset \
        server-dev server-test server-build web-dev dev clean dto

# ══════════════════════════════════════════════════════════════════════
# Project setup
# ══════════════════════════════════════════════════════════════════════

setup:
	@echo "==> Installing Go dependencies"
	cd $(SERVER_DIR) && $(GO) mod download
	@echo "==> Installing web dependencies"
	cd $(WEB_DIR) && $(PNPM) install
	@echo "==> Ensuring wasm-pack is installed"
	@command -v wasm-pack >/dev/null 2>&1 || { curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh; }
	@echo "==> Ensuring river CLI is installed"
	@command -v river >/dev/null 2>&1 || { cd $(SERVER_DIR) && $(GO) install github.com/riverqueue/river/cmd/river@v0.24.0; }
	@echo "==> Ensuring swag CLI is installed"
	@command -v swag >/dev/null 2>&1 || { $(GO) install github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5; }
	@echo "==> Ensuring sqlc is installed"
	@command -v sqlc >/dev/null 2>&1 || { $(GO) install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0; }
	@echo "==> Setup complete"

# ── Environment files ────────────────────────────────────────────────

env-dev:
	@mkdir -p $(SERVER_DIR)/data/storage/primary
	@if [ ! -f $(SERVER_DIR)/.env.development ]; then \
		echo "==> Creating $(SERVER_DIR)/.env.development"; \
		printf '%s\n' \
		"SERVER_ENV=development" \
		"SERVER_PORT=6680" \
		"SERVER_LOG_LEVEL=debug" \
		"SERVER_CORS_ALLOWED_ORIGINS=http://localhost:6657,https://localhost:6657" \
		"" \
		"DB_HOST=$(DB_HOST)" \
		"DB_PORT=$(DB_PORT)" \
		"DB_USER=$(DB_USER)" \
		"DB_PASSWORD=$(DB_PASSWORD)" \
		"DB_NAME=$(DB_NAME)" \
		"DB_SSL=disable" \
		"" \
		"STORAGE_PATH=./data/storage" \
		"STORAGE_STRATEGY=date" \
		"STORAGE_PRESERVE_FILENAME=true" \
		"STORAGE_DUPLICATE_HANDLING=rename" \
		"" \
		"REPOSITORY_SCAN_ENABLED=false" \
		"REPOSITORY_SCAN_INTERVAL_SECONDS=300" \
		"REPOSITORY_SCAN_SETTLE_SECONDS=5" \
		"REPOSITORY_SCAN_MAX_CONCURRENT_REPOS=1" \
		"REPOSITORY_SCAN_BATCH_SIZE=500" \
		"" \
		"GEOCODING_PROVIDER=disabled" \
		"GEOCODING_NOMINATIM_ENDPOINT=" \
		"GEOCODING_LANGUAGE=en" \
		"GEOCODING_USER_AGENT=Lumilio-Photos/1.0" \
		"GEOCODING_NATURALEARTH_CITY_RADIUS_METERS=50000" \
		"" \
		"ML_CLIP_ENABLED=false" \
		"ML_BIOCLIP_ENABLED=false" \
		"ML_OCR_ENABLED=false" \
		"ML_FACE_ENABLED=false" \
		"" \
		"LUMILIO_SECRET_KEY=./data/storage/.secrets/lumilio_secret_key" \
		"" \
		"LLM_AGENT_ENABLED=false" \
		"LLM_PROVIDER=" \
		"LLM_API_KEY=" \
		"LLM_MODEL_NAME=" \
		"LLM_BASE_URL=" \
		> $(SERVER_DIR)/.env.development; \
		echo "==> Created $(SERVER_DIR)/.env.development (DB_HOST=$(DB_HOST) DB_PORT=$(DB_PORT))"; \
	else \
		echo "==> $(SERVER_DIR)/.env.development already exists, skipping..."; \
	fi
	@echo "==> Created storage directory at $(SERVER_DIR)/data/storage/primary"

env-web-dev:
	@echo "==> Writing $(WEB_DIR)/.env.development"
	@printf '%s\n' \
	"VITE_API_URL=$(API_URL)" \
	"API_URL=$(API_URL)" \
	> $(WEB_DIR)/.env.development

# ══════════════════════════════════════════════════════════════════════
# Database (managed via devcontainer compose)
# ══════════════════════════════════════════════════════════════════════

db-build:
	@echo "==> Building database image..."
	@$(COMPOSE) build $(DB_SERVICE)

db-up: db-build
	@echo "==> Starting database service..."
	@$(COMPOSE) up -d $(DB_SERVICE)

db-down:
	@echo "==> Stopping database service..."
	@$(COMPOSE) stop $(DB_SERVICE)

db-reset:
	@echo "==> Resetting database (removing container and volume)..."
	@$(COMPOSE) stop $(DB_SERVICE)
	@$(COMPOSE) rm -f $(DB_SERVICE)
	@$(DOCKER) volume rm -f $(DB_VOLUME) 2>/dev/null || true
	@echo "==> Database reset complete (run make db-wait to recreate)"

db-logs:
	@$(COMPOSE) logs -f $(DB_SERVICE)

db-shell:
	@$(COMPOSE) exec $(DB_SERVICE) psql -U $(DB_USER) -d $(DB_NAME)

db-wait:
	@echo "==> Ensuring database is up and waiting for healthy status..."
	@$(COMPOSE) up -d --wait $(DB_SERVICE)

# ══════════════════════════════════════════════════════════════════════
# Natural Earth geodata import (managed via devcontainer compose)
# ══════════════════════════════════════════════════════════════════════

geodata-build:
	@echo "==> Building Natural Earth import image..."
	@$(COMPOSE) build naturalearth-import

geodata-import: db-wait geodata-build
	@echo "==> Importing Natural Earth data..."
	@$(COMPOSE) --profile geodata run --rm naturalearth-import

geodata-reset: db-wait geodata-build
	@echo "==> Reimporting Natural Earth data (--force)..."
	@$(COMPOSE) --profile geodata run --rm naturalearth-import \
		/bin/bash /import-natural-earth.sh --force

# ══════════════════════════════════════════════════════════════════════
# Server & Web
# ══════════════════════════════════════════════════════════════════════

server-dev: env-dev
	@echo "==> Starting server in development mode..."
	cd $(SERVER_DIR) && SERVER_ENV=development $(GO) run ./cmd

server-test:
	cd $(SERVER_DIR) && $(GO) test ./...

server-build:
	cd $(SERVER_DIR) && mkdir -p bin && $(GO) build -o bin/server ./cmd

web-dev: env-web-dev
	@echo "==> Starting web development server..."
	cd $(WEB_DIR) && $(PNPM) run dev -- --host --port 6657

dev: db-wait
	@echo "==> Starting development environment..."
	@echo "==> Database is ready, starting server and web..."
	$(MAKE) -j2 server-dev web-dev

clean:
	@echo "==> Cleaning up..."
	rm -f $(SERVER_DIR)/.env.development
	rm -f $(WEB_DIR)/.env.development
	rm -rf $(SERVER_DIR)/data
	@echo "==> Clean complete"

dto:
	@echo "==> Generating OpenAPI spec and TypeScript types..."
	@echo "==> Step 1: Generating OpenAPI spec from Go code..."
	cd $(SERVER_DIR) && swag init --v3.1 -g cmd/main.go -o docs/
	@echo "==> Step 2: Generating TypeScript types from OpenAPI spec..."
	cd $(WEB_DIR) && ./node_modules/.bin/openapi-typescript ../server/docs/swagger.yaml -o ./src/lib/http-commons/schema.d.ts
	@echo "==> DTO synchronization complete!"
	@echo "==> Backend: $(SERVER_DIR)/docs/swagger.yaml"
	@echo "==> Frontend: $(WEB_DIR)/src/lib/http-commons/schema.d.ts"
