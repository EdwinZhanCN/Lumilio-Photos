SHELL := /bin/sh

WEB_DIR := web
SITE_DIR := site
SERVER_DIR := server
DESKTOP_DIR := desktop
SERVER_CONFIG_EXAMPLE := $(SERVER_DIR)/config/server.example.toml
SERVER_CONFIG_LOCAL := $(SERVER_DIR)/config/server.local.toml
DEV_DATABASE := $(SERVER_DIR)/.local/lumilio/library.sqlite3
DEV_DERIVED := $(SERVER_DIR)/.local/lumilio/derived

GO := go
VP := vp
GO_BUILD_TAGS ?= sqlite_fts5
GO_TAG_FLAGS := $(if $(strip $(GO_BUILD_TAGS)),-tags=$(strip $(GO_BUILD_TAGS)))

# Homebrew's libraw_r.pc emits `-Xpreprocessor -fopenmp`, which Go's cgo flag
# allowlist rejects ("invalid flag in pkg-config --libs: -Xpreprocessor"). Allow
# it so cgo can build the libraw binding (server/internal/utils/raw) and anything
# that imports it. Exported so every Go target (server-*, desktop-*) inherits it.
# Harmless on Linux/CI where the flag isn't present.
export CGO_LDFLAGS_ALLOW := -Xpreprocessor
export CGO_CFLAGS_ALLOW := -Xpreprocessor

API_URL ?= http://localhost:6680
VITE_API_URL ?= $(API_URL)

.PHONY: setup dev server-dev web-dev test server-test web-test web-browser-test web-auth-hardening-test web-video-semantic-test web-backup-recovery-test dto db-reset dev-reset sqlite-architecture-check \
	desktop-dev desktop-build desktop-test desktop-panel \
	.server-config .web-env

setup: .server-config
	@echo "==> Installing Go dependencies"
	cd $(SERVER_DIR) && $(GO) mod download
	@echo "==> Installing web dependencies"
	cd $(WEB_DIR) && CI=1 VITE_GIT_HOOKS=0 $(VP) install
	@echo "==> Installing documentation site dependencies"
	cd $(SITE_DIR) && CI=1 VITE_GIT_HOOKS=0 $(VP) install
	@echo "==> Ensuring wasm-pack is installed"
	@command -v wasm-pack >/dev/null 2>&1 || { curl https://rustwasm.github.io/wasm-pack/installer/init.sh -sSf | sh; }
	@echo "==> Ensuring swag CLI is installed"
	@command -v swag >/dev/null 2>&1 || { $(GO) install github.com/swaggo/swag/v2/cmd/swag@v2.0.0-rc5; }
	@if git rev-parse --git-dir >/dev/null 2>&1; then \
		echo "==> Installing repository commit hook"; \
		git config --local core.hooksPath .githooks; \
	fi
	@echo "==> Setup complete"

dev:
	@echo "==> Starting server and web"
	$(MAKE) -j2 server-dev web-dev

server-dev: .server-config
	@echo "==> Starting server"
	cd $(SERVER_DIR) && $(GO) run $(GO_TAG_FLAGS) ./cmd --config config/server.local.toml

web-dev: .web-env
	@echo "==> Starting web"
	cd $(WEB_DIR) && $(VP) dev --host --port 6657

test: server-test web-test

sqlite-architecture-check:
	./scripts/check-sqlite-architecture.sh

server-test: sqlite-architecture-check
	cd $(SERVER_DIR) && $(GO) test $(GO_TAG_FLAGS) ./...

web-test:
	cd $(WEB_DIR) && \
		$(VP) check --no-fmt --no-lint && \
		$(VP) lint && \
		$(VP) node scripts/check-source-boundaries.mjs && \
		$(VP) test

web-browser-test:
	cd $(WEB_DIR) && $(VP) run e2e:seed && $(VP) run test:browser

web-auth-hardening-test:
	cd $(WEB_DIR) && $(VP) run e2e:seed && $(VP) run test:auth-hardening

web-video-semantic-test:
	cd $(WEB_DIR) && $(VP) run e2e:seed:video-semantic && $(VP) run test:video-semantic

web-backup-recovery-test:
	cd $(WEB_DIR) && $(VP) run e2e:seed && $(VP) run test:backup-recovery

desktop-panel:
	@echo "==> Building desktop control panel (Svelte, embedded into the Go binary)"
	cd $(DESKTOP_DIR)/panel && CI=1 VITE_GIT_HOOKS=0 $(VP) install && $(VP) run build

desktop-dev: desktop-panel
	@echo "==> Running desktop app (dev)"
	@echo "    Serving the SPA from $(CURDIR)/$(WEB_DIR)/dist (run 'cd web && vp build' first)."
	cd $(DESKTOP_DIR) && \
		LUMILIO_WEB_ROOT=$(CURDIR)/$(WEB_DIR)/dist \
		$(GO) run $(GO_TAG_FLAGS) .

desktop-test: sqlite-architecture-check desktop-panel
	@echo "==> Testing desktop module (including SQLite first/second launch)"
	cd $(DESKTOP_DIR) && $(GO) test $(GO_TAG_FLAGS) ./...

desktop-build: desktop-panel
	@echo "==> Building macOS desktop app bundle"
	LUMILIO_PANEL_DIST_PREBUILT=1 $(DESKTOP_DIR)/scripts/build-macos.sh

dto:
	@echo "==> Generating OpenAPI spec, TypeScript types, and API documentation"
	cd $(SERVER_DIR) && swag init --v3.1 -g cmd/main.go -o docs/
	cd $(WEB_DIR) && $(VP) node scripts/generate-openapi-types.mjs
	cd $(SITE_DIR) && ./node_modules/.bin/redocly build-docs ../server/docs/swagger.yaml --output docs/public/redoc-static.html

db-reset:
	@echo "==> Removing the known development SQLite catalog and derived indexes"
	rm -f "$(DEV_DATABASE)" "$(DEV_DATABASE)-wal" "$(DEV_DATABASE)-shm"
	rm -rf "$(DEV_DERIVED)"

dev-reset: db-reset
	@echo "==> Recreating local development config"
	rm -f $(WEB_DIR)/.env.development
	rm -f $(SERVER_CONFIG_LOCAL)
	$(MAKE) .server-config .web-env

.server-config:
	@if [ ! -f "$(SERVER_CONFIG_LOCAL)" ]; then \
		echo "==> Creating $(SERVER_CONFIG_LOCAL) from $(SERVER_CONFIG_EXAMPLE)"; \
		cp "$(SERVER_CONFIG_EXAMPLE)" "$(SERVER_CONFIG_LOCAL)"; \
	fi

.web-env:
	@printf '%s\n' \
	"VITE_API_URL=$(API_URL)" \
	"API_URL=$(API_URL)" \
	> $(WEB_DIR)/.env.development
