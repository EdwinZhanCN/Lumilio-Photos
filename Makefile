SHELL := /bin/sh

WEB_DIR := web
SITE_DIR := site
SERVER_DIR := server
DESKTOP_DIR := desktop
DEV_ROOT := $(CURDIR)/.local/dev
DEV_CONFIG := $(DEV_ROOT)/config/server.toml
DEV_STATE_SCRIPT := scripts/dev-state.sh

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

# Where the Vite dev server proxies /api. The browser never sees this address:
# it talks only to the Vite origin, so the SPA uses relative URLs.
API_URL ?= http://127.0.0.1:6680
WEB_DEV_PORT ?= 6657
DEV_ORIGIN ?= http://localhost:$(WEB_DEV_PORT)

.PHONY: setup dev dev-config dev-clean dev-reset dev-purge server-dev .server-dev web-dev test server-test web-test web-browser-test web-auth-hardening-test web-video-semantic-test web-backup-recovery-test dto architecture-check compose-test config-examples \
	desktop-dev desktop-build desktop-test desktop-panel

setup:
	@echo "==> Installing Go dependencies"
	cd $(SERVER_DIR) && $(GO) mod download
	$(MAKE) dev-config
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

dev: dev-config
	@echo "==> Starting server and web"
	$(MAKE) -j2 .server-dev web-dev

server-dev: dev-config
	$(MAKE) .server-dev

.server-dev:
	@echo "==> Starting server"
	cd $(SERVER_DIR) && $(GO) run $(GO_TAG_FLAGS) ./cmd --config "$(DEV_CONFIG)"

web-dev:
	@echo "==> Starting web"
	cd $(WEB_DIR) && API_URL="$(API_URL)" $(VP) dev --host --port $(WEB_DEV_PORT)

test: server-test web-test

architecture-check:
	./scripts/check-architecture.sh

compose-test:
	LUMILIO_STORAGE=/srv/lumilio/media LUMILIO_STATE=/srv/lumilio/state LUMILIO_DOMAIN=photos.example.com docker compose -f deploy/compose/compose.caddy.yml config --quiet
	LUMILIO_STORAGE=/srv/lumilio/media LUMILIO_STATE=/srv/lumilio/state docker compose -f deploy/compose/compose.acme.yml config --quiet
	LUMILIO_STORAGE=/srv/lumilio/media LUMILIO_STATE=/srv/lumilio/state docker compose -f deploy/compose/compose.proxy.yml config --quiet
	docker compose -f web/e2e/compose.yml -f web/e2e/compose.ci.yml config --quiet

server-test: architecture-check
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

desktop-test: architecture-check desktop-panel
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

dev-config:
	@sh $(DEV_STATE_SCRIPT) init "$(CURDIR)"
	@echo "==> Generating $(DEV_CONFIG)"
	cd $(SERVER_DIR) && $(GO) run $(GO_TAG_FLAGS) ./cmd config init \
		--profile dev-vite \
		--origin "$(DEV_ORIGIN)" \
		--state-dir ../state \
		--storage-dir ../storage \
		--output "$(DEV_CONFIG)" \
		--force

dev-clean:
	@sh $(DEV_STATE_SCRIPT) clean "$(CURDIR)"

dev-reset:
	@sh $(DEV_STATE_SCRIPT) reset "$(CURDIR)"

dev-purge:
	@CONFIRM_DEV_PURGE="$(CONFIRM)" sh $(DEV_STATE_SCRIPT) purge "$(CURDIR)"

# Regenerate the manifest JSON Schema and every example from the profile table
# in server/config/profiles.go. The schema is embedded, so a change to the doc
# comments needs the second pass to reach the generated TOML.
config-examples:
	@echo "==> Regenerating config schema and examples"
	cd $(SERVER_DIR) && $(GO) run ./tools/configgen >/dev/null && $(GO) run ./tools/configgen
