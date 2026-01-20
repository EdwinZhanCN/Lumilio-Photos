SHELL := /bin/sh

WEB_DIR := web
SERVER_DIR := server

GO := go
NPM := npm

API_URL ?= http://localhost:8080
VITE_API_URL ?= $(API_URL)

.PHONY: setup env-dev env-web-dev db-up db-down db-reset db-logs db-shell db-wait server-dev server-test server-build web-dev dev clean dto

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
	@mkdir -p $(SERVER_DIR)/data/storage
	@if [ ! -f $(SERVER_DIR)/.env.development ]; then \
		echo "==> Creating $(SERVER_DIR)/.env.development"; \
		printf '%s\n' \
		"SERVER_ENV=development" \
		"SERVER_PORT=8080" \
		"SERVER_LOG_LEVEL=debug" \
		"" \
		"DB_HOST=localhost" \
		"DB_PORT=5433" \
		"DB_USER=postgres" \
		"DB_PASSWORD=postgres" \
		"DB_NAME=lumiliophotos" \
		"DB_SSL=disable" \
		"" \
		"STORAGE_PATH=./data/storage" \
		"STORAGE_STRATEGY=date" \
		"STORAGE_PRESERVE_FILENAME=true" \
		"STORAGE_DUPLICATE_HANDLING=rename" \
		"" \
		"ML_CLIP_ENABLED=false" \
		"ML_OCR_ENABLED=false" \
		"ML_CAPTION_ENABLED=false" \
		"ML_FACE_ENABLED=false" \
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
	@echo "==> Created storage directory at $(SERVER_DIR)/data/storage"

env-web-dev:
	@echo "==> Writing $(WEB_DIR)/.env.development"
	@printf '%s\n' \
	"VITE_API_URL=$(API_URL)" \
	"API_URL=$(API_URL)" \
	> $(WEB_DIR)/.env.development

db-up:
	@echo "==> Starting database..."
	docker compose up db -d

db-wait: db-up
	@echo "==> Waiting for database to be ready..."
	@timeout=60; \
	while [ $$timeout -gt 0 ]; do \
		if docker compose exec -T db pg_isready -U postgres -d lumiliophotos >/dev/null 2>&1; then \
			echo "==> Database is ready!"; \
			exit 0; \
		fi; \
		echo "==> Waiting for database... ($$timeout seconds remaining)"; \
		sleep 2; \
		timeout=$$((timeout - 2)); \
	done; \
	echo "==> Database failed to start within timeout"; \
	exit 1

db-down:
	docker compose stop db

db-reset:
	docker compose down -v

db-logs:
	docker compose logs -f db

db-shell:
	docker compose exec db psql -U postgres -d lumiliophotos

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
