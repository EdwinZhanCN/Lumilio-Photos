SHELL := /bin/sh

WEB_DIR := web
SITE_DIR := site
SERVER_DIR := server
DESKTOP_DIR := desktop
SERVER_CONFIG_EXAMPLE := $(SERVER_DIR)/config/server.example.toml
SERVER_CONFIG_LOCAL := $(SERVER_DIR)/config/server.local.toml

# PostgreSQL bin directory used when running the desktop app in development. The
# packaged app ships its own PostgreSQL; for local `make desktop-dev` point this
# at a locally installed PostgreSQL (override on the command line as needed,
# e.g. `make desktop-dev PG_BIN_DIR=/opt/homebrew/opt/postgresql@14/bin`).
PG_BIN_DIR ?= /opt/homebrew/opt/postgresql@17/bin

GO := go
VP := vp
DOCKER := docker

# Homebrew's libraw_r.pc emits `-Xpreprocessor -fopenmp`, which Go's cgo flag
# allowlist rejects ("invalid flag in pkg-config --libs: -Xpreprocessor"). Allow
# it so cgo can build the libraw binding (server/internal/utils/raw) and anything
# that imports it. Exported so every Go target (server-*, desktop-*) inherits it.
# Harmless on Linux/CI where the flag isn't present.
export CGO_LDFLAGS_ALLOW := -Xpreprocessor
export CGO_CFLAGS_ALLOW := -Xpreprocessor

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

.PHONY: setup dev server-dev web-dev test server-test web-test dto db db-reset dev-reset clean \
	desktop-dev desktop-build desktop-test \
	.server-config .server-env .web-env

setup: .server-config
	@echo "==> Installing Go dependencies"
	cd $(SERVER_DIR) && $(GO) mod download
	@echo "==> Installing web dependencies"
	cd $(WEB_DIR) && $(VP) install
	@echo "==> Installing documentation site dependencies"
	cd $(SITE_DIR) && $(VP) install
	@echo "==> Ensuring wasm-pack is installed"
	@command -v wasm-pack >/dev/null 2>&1 || { curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh; }
	@echo "==> Ensuring swag CLI is installed"
	@command -v swag >/dev/null 2>&1 || { $(GO) install github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5; }
	@echo "==> Setup complete"

db:
	@echo "==> Starting database and waiting for healthy status"
	@$(COMPOSE) up -d --wait db

dev: db
	@echo "==> Starting server and web"
	$(MAKE) -j2 server-dev web-dev

server-dev: .server-config .server-env
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

desktop-dev:
	@echo "==> Running desktop app (dev). PG_BIN_DIR=$(PG_BIN_DIR)"
	@echo "    Serving the SPA from $(CURDIR)/$(WEB_DIR)/dist (run 'cd web && vp build' first)."
	cd $(DESKTOP_DIR) && \
		LUMILIO_PG_BIN_DIR=$(PG_BIN_DIR) \
		LUMILIO_WEB_ROOT=$(CURDIR)/$(WEB_DIR)/dist \
		$(GO) run .

desktop-test:
	@echo "==> Testing desktop module (PostgreSQL lifecycle test skips if no PG)"
	cd $(DESKTOP_DIR) && $(GO) test ./...

desktop-build:
	@echo "==> Building macOS desktop app bundle"
	$(DESKTOP_DIR)/scripts/build-macos.sh

dto:
	@echo "==> Generating OpenAPI spec, TypeScript types, and API documentation"
	cd $(SERVER_DIR) && swag init --v3.1 -g cmd/main.go -o docs/
	cd $(WEB_DIR) && ./node_modules/.bin/openapi-typescript ../server/docs/swagger.yaml -o ./src/lib/http-commons/schema.d.ts
	cd $(SITE_DIR) && ./node_modules/.bin/redocly build-docs ../server/docs/swagger.yaml --output docs/public/redoc-static.html

db-reset:
	@echo "==> Resetting database volume"
	@$(COMPOSE) stop db
	@$(COMPOSE) rm -f db
	@if $(DOCKER) volume inspect $(DB_VOLUME) >/dev/null 2>&1; then \
		$(DOCKER) volume rm $(DB_VOLUME) >/dev/null; \
	fi
	@rm -f $(SERVER_DIR)/data/storage/.secrets/db_password

dev-reset: db-reset clean

clean:
	@echo "==> Cleaning generated local development state"
	rm -f $(SERVER_DIR)/.env.development
	rm -f $(WEB_DIR)/.env.development
	rm -rf $(SERVER_DIR)/data

.server-config:
	@if [ ! -f "$(SERVER_CONFIG_LOCAL)" ]; then \
		echo "==> Creating $(SERVER_CONFIG_LOCAL) from $(SERVER_CONFIG_EXAMPLE)"; \
		cp "$(SERVER_CONFIG_EXAMPLE)" "$(SERVER_CONFIG_LOCAL)"; \
	fi

.server-env:
	@mkdir -p $(SERVER_DIR)/data/storage/primary
	@echo "==> Writing $(SERVER_DIR)/.env.development"
	@printf '%s\n' \
	"SERVER_ENV=development" \
	"SERVER_CONFIG_FILE=config/server.local.toml" \
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
	> $(SERVER_DIR)/.env.development

.web-env:
	@printf '%s\n' \
	"VITE_API_URL=$(API_URL)" \
	"API_URL=$(API_URL)" \
	> $(WEB_DIR)/.env.development
