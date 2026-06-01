SHELL := /bin/sh

WEB_DIR := web
SERVER_DIR := server

GO := go
VP := vp
DOCKER := docker

API_URL ?= http://localhost:6680
VITE_API_URL ?= $(API_URL)

COMPOSE_FILE ?= .devcontainer/docker-compose.yml
COMPOSE_PROJECT ?= lumilio-photos-devcontainer
COMPOSE_BIN := $(shell \
	if $(DOCKER) compose version >/dev/null 2>&1; then \
		printf '%s compose' '$(DOCKER)'; \
	elif command -v docker-compose >/dev/null 2>&1; then \
		printf '%s' docker-compose; \
	fi)
ifeq ($(COMPOSE_BIN),)
  $(error Docker Compose V2 is required. Install docker-compose-plugin or docker compose)
endif
COMPOSE := $(COMPOSE_BIN) -f $(COMPOSE_FILE) -p $(COMPOSE_PROJECT)

IN_DEV_CONTAINER := $(shell [ -f /.dockerenv ] && echo 1 || echo 0)
ifeq ($(IN_DEV_CONTAINER),1)
  DB_HOST ?= db
  DB_PORT ?= 5432
else
  DB_HOST ?= localhost
  DB_PORT ?= 5433
endif
DB_NAME ?= lumiliophotos
DB_USER ?= postgres
DB_PASSWORD ?= postgres
DB_VOLUME ?= $(COMPOSE_PROJECT)_db_data

.PHONY: setup dev server-dev web-dev test server-test web-test dto db db-reset clean \
	.server-env .web-env

setup:
	@echo "==> Installing Go dependencies"
	cd $(SERVER_DIR) && $(GO) mod download
	@echo "==> Installing web dependencies"
	cd $(WEB_DIR) && $(VP) install
	@echo "==> Ensuring wasm-pack is installed"
	@command -v wasm-pack >/dev/null 2>&1 || { curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh; }
	@echo "==> Ensuring river CLI is installed"
	@command -v river >/dev/null 2>&1 || { cd $(SERVER_DIR) && $(GO) install github.com/riverqueue/river/cmd/river@v0.24.0; }
	@echo "==> Ensuring swag CLI is installed"
	@command -v swag >/dev/null 2>&1 || { $(GO) install github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5; }
	@echo "==> Setup complete"

db:
	@echo "==> Starting database and waiting for healthy status"
	@$(COMPOSE) up -d --wait db

dev: db
	@echo "==> Starting server and web"
	$(MAKE) -j2 server-dev web-dev

server-dev: .server-env
	@echo "==> Starting server"
	cd $(SERVER_DIR) && SERVER_ENV=development $(GO) run ./cmd

web-dev: .web-env
	@echo "==> Starting web"
	cd $(WEB_DIR) && $(VP) dev --host --port 6657

test: server-test web-test

server-test:
	cd $(SERVER_DIR) && $(GO) test ./...

web-test:
	cd $(WEB_DIR) && $(VP) check --no-fmt --no-lint && $(VP) lint && $(VP) test

dto:
	@echo "==> Generating OpenAPI spec and TypeScript types"
	cd $(SERVER_DIR) && swag init --v3.1 -g cmd/main.go -o docs/
	cd $(WEB_DIR) && ./node_modules/.bin/openapi-typescript ../server/docs/swagger.yaml -o ./src/lib/http-commons/schema.d.ts

db-reset:
	@echo "==> Resetting database volume"
	@$(COMPOSE) stop db
	@$(COMPOSE) rm -f db
	@$(DOCKER) volume rm -f $(DB_VOLUME) 2>/dev/null || true

clean:
	@echo "==> Cleaning generated local development state"
	rm -f $(SERVER_DIR)/.env.development
	rm -f $(WEB_DIR)/.env.development
	rm -rf $(SERVER_DIR)/data

.server-env:
	@mkdir -p $(SERVER_DIR)/data/storage/primary
	@if [ ! -f $(SERVER_DIR)/.env.development ]; then \
		echo "==> Creating $(SERVER_DIR)/.env.development"; \
		printf '%s\n' \
		"SERVER_ENV=development" \
		"SERVER_CONFIG_FILE=config/server.development.toml" \
		"SERVER_PORT=6680" \
		"" \
		"DB_HOST=$(DB_HOST)" \
		"DB_PORT=$(DB_PORT)" \
		"DB_USER=$(DB_USER)" \
		"DB_PASSWORD=$(DB_PASSWORD)" \
		"DB_NAME=$(DB_NAME)" \
		"DB_SSL=disable" \
		"" \
		"STORAGE_PATH=./data/storage" \
		"" \
		"LUMILIO_SECRET_KEY=./data/storage/.secrets/lumilio_secret_key" \
		> $(SERVER_DIR)/.env.development; \
	fi

.web-env:
	@printf '%s\n' \
	"VITE_API_URL=$(API_URL)" \
	"API_URL=$(API_URL)" \
	> $(WEB_DIR)/.env.development
