SHELL := /bin/sh

WEB_DIR := web
SERVER_DIR := server

GO := go
NPM := npm
DOCKER := docker

API_URL ?= http://localhost:8080
VITE_API_URL ?= $(API_URL)

DEV_DB_IMAGE ?= lumilio-dev-db
DEV_DB_CONTAINER ?= lumilio-dev-db
DEV_DB_VOLUME ?= lumilio-dev-db-data
DEV_DB_PORT ?= 5433
DEV_DB_NAME ?= lumiliophotos
DEV_DB_USER ?= postgres
DEV_DB_PASSWORD ?= postgres

.PHONY: setup env-dev env-web-dev db-build db-up db-down db-reset db-logs db-shell db-wait server-dev server-test server-build web-dev dev clean dto

setup:
	@echo "==> Installing Go dependencies"
	cd $(SERVER_DIR) && $(GO) mod download
	@echo "==> Installing web dependencies"
	cd $(WEB_DIR) && $(NPM) install
	@echo "==> Ensuring wasm-pack is installed"
	@command -v wasm-pack >/dev/null 2>&1 || { curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh; }
	@echo "==> Ensuring river CLI is installed"
	@command -v river >/dev/null 2>&1 || { cd $(SERVER_DIR) && $(GO) install github.com/riverqueue/river/cmd/river@v0.24.0; }
	@echo "==> Setup complete"

env-dev:
	@mkdir -p $(SERVER_DIR)/data/storage/primary
	@if [ ! -f $(SERVER_DIR)/.env.development ]; then \
		echo "==> Creating $(SERVER_DIR)/.env.development"; \
		printf '%s\n' \
		"SERVER_ENV=development" \
		"SERVER_PORT=8080" \
		"SERVER_LOG_LEVEL=debug" \
		"SERVER_CORS_ALLOWED_ORIGINS=http://localhost:6657,https://localhost:6657" \
		"" \
		"DB_HOST=localhost" \
		"DB_PORT=$(DEV_DB_PORT)" \
		"DB_USER=$(DEV_DB_USER)" \
		"DB_PASSWORD=$(DEV_DB_PASSWORD)" \
		"DB_NAME=$(DEV_DB_NAME)" \
		"DB_SSL=disable" \
		"" \
		"STORAGE_PATH=./data/storage" \
		"STORAGE_STRATEGY=date" \
		"STORAGE_PRESERVE_FILENAME=true" \
		"STORAGE_DUPLICATE_HANDLING=rename" \
		"" \
		"WATCHMAN_ENABLED=false" \
		"WATCHMAN_SOCK=" \
		"WATCHMAN_SETTLE_SECONDS=3" \
		"WATCHMAN_INITIAL_SCAN=true" \
		"WATCHMAN_POLL_FALLBACK_SECONDS=0" \
		"" \
		"ML_AUTO=disable" \
		"ML_CLIP_ENABLED=false" \
		"ML_OCR_ENABLED=false" \
		"ML_CAPTION_ENABLED=false" \
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
		echo "==> Created $(SERVER_DIR)/.env.development"; \
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

db-build:
	@echo "==> Building development database image..."
	@$(DOCKER) build -f $(SERVER_DIR)/db.Dockerfile -t $(DEV_DB_IMAGE) .

db-up: db-build
	@echo "==> Starting development database container..."
	@if [ "$$($(DOCKER) ps -aq -f name=^$(DEV_DB_CONTAINER)$$)" != "" ]; then \
		if [ "$$($(DOCKER) inspect -f '{{.State.Running}}' $(DEV_DB_CONTAINER))" = "true" ]; then \
			echo "==> Container $(DEV_DB_CONTAINER) already running"; \
		else \
			$(DOCKER) start $(DEV_DB_CONTAINER) >/dev/null; \
			echo "==> Container $(DEV_DB_CONTAINER) started"; \
		fi; \
	else \
		$(DOCKER) run -d \
			--name $(DEV_DB_CONTAINER) \
			-e POSTGRES_DB=$(DEV_DB_NAME) \
			-e POSTGRES_USER=$(DEV_DB_USER) \
			-e POSTGRES_PASSWORD=$(DEV_DB_PASSWORD) \
			-p $(DEV_DB_PORT):5432 \
			-v $(DEV_DB_VOLUME):/var/lib/postgresql/data \
			$(DEV_DB_IMAGE) >/dev/null; \
		echo "==> Container $(DEV_DB_CONTAINER) created"; \
	fi

db-wait: db-up
	@echo "==> Waiting for database to be ready..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		status=$$($(DOCKER) inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}unknown{{end}}' $(DEV_DB_CONTAINER) 2>/dev/null || echo missing); \
		if [ "$$status" = "healthy" ]; then \
			echo "==> Database is ready!"; \
			exit 0; \
		fi; \
		echo "==> Waiting for database... ($$timeout seconds remaining, status=$$status)"; \
		sleep 2; \
		timeout=$$((timeout - 2)); \
	done; \
	echo "==> Database failed to start within timeout"; \
	exit 1

db-down:
	@echo "==> Stopping development database container..."
	@if [ "$$($(DOCKER) ps -q -f name=^$(DEV_DB_CONTAINER)$$)" != "" ]; then \
		$(DOCKER) stop $(DEV_DB_CONTAINER) >/dev/null; \
		echo "==> Container $(DEV_DB_CONTAINER) stopped"; \
	else \
		echo "==> Container $(DEV_DB_CONTAINER) is not running"; \
	fi

db-reset:
	@echo "==> Resetting development database container and volume..."
	@if [ "$$($(DOCKER) ps -aq -f name=^$(DEV_DB_CONTAINER)$$)" != "" ]; then \
		$(DOCKER) rm -f $(DEV_DB_CONTAINER) >/dev/null; \
		echo "==> Container $(DEV_DB_CONTAINER) removed"; \
	else \
		echo "==> Container $(DEV_DB_CONTAINER) does not exist"; \
	fi
	@if [ "$$($(DOCKER) volume ls -q -f name=^$(DEV_DB_VOLUME)$$)" != "" ]; then \
		$(DOCKER) volume rm $(DEV_DB_VOLUME) >/dev/null; \
		echo "==> Volume $(DEV_DB_VOLUME) removed"; \
	else \
		echo "==> Volume $(DEV_DB_VOLUME) does not exist"; \
	fi

db-logs:
	$(DOCKER) logs -f $(DEV_DB_CONTAINER)

db-shell:
	$(DOCKER) exec -it $(DEV_DB_CONTAINER) psql -U $(DEV_DB_USER) -d $(DEV_DB_NAME)

server-dev: env-dev
	@echo "==> Starting server in development mode..."
	cd $(SERVER_DIR) && SERVER_ENV=development $(GO) run ./cmd

server-test:
	cd $(SERVER_DIR) && $(GO) test ./...

server-build:
	cd $(SERVER_DIR) && mkdir -p bin && $(GO) build -o bin/server ./cmd

web-dev: env-web-dev
	@echo "==> Starting web development server..."
	cd $(WEB_DIR) && $(NPM) run dev -- --host --port 6657

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
	cd $(WEB_DIR) && npx openapi-typescript ../server/docs/swagger.yaml -o ./src/lib/http-commons/schema.d.ts
	@echo "==> DTO synchronization complete!"
	@echo "==> Backend: $(SERVER_DIR)/docs/swagger.yaml"
	@echo "==> Frontend: $(WEB_DIR)/src/lib/http-commons/schema.d.ts"
