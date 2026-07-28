SHELL := /bin/sh

WEB_DIR := web
SITE_DIR := site
SERVER_DIR := server
DESKTOP_DIR := desktop
SERVER_CONFIG_EXAMPLE := $(SERVER_DIR)/config/examples/dev/vite.toml
SERVER_CONFIG_LOCAL := $(SERVER_DIR)/config/server.local.toml
DEV_DATABASE := $(SERVER_DIR)/.local/lumilio/library.sqlite3
DEV_DERIVED := $(SERVER_DIR)/.local/lumilio/derived
# Every root the development manifest owns. These mirror the dev-vite profile
# layout in server/config/profiles.go: .local holds the catalog and derived
# indexes, data holds storage plus machine-bound app-state (secrets, backups,
# cloud), and logs holds the rotated log files. Change the profile layout and
# this list has to follow.
DEV_STATE_DIRS := $(SERVER_DIR)/.local $(SERVER_DIR)/data $(SERVER_DIR)/logs

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

.PHONY: setup dev server-dev web-dev test server-test web-test web-browser-test web-auth-hardening-test web-video-semantic-test web-backup-recovery-test dto db-reset dev-reset architecture-check compose-test config-examples \
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

# Narrow reset: the catalog only. Media, secrets and backups survive, so the
# next boot re-indexes the existing storage instead of starting from nothing.
db-reset:
	@echo "==> Removing the known development SQLite catalog and derived indexes"
	rm -f "$(DEV_DATABASE)" "$(DEV_DATABASE)-wal" "$(DEV_DATABASE)-shm"
	rm -rf "$(DEV_DERIVED)"

# Full reset: every root the development manifest owns, plus the generated
# config. Resetting only the catalog left the old secret key, backups and
# storage repositories behind, which is an inconsistent state the server has no
# way to reconcile — so this removes all of it and lets `make dev` rebuild.
#
# This deletes the development media library. Use db-reset to keep it.
dev-reset:
	@echo "==> Removing development state:"
	@for dir in $(DEV_STATE_DIRS); do \
		if [ -e "$$dir" ]; then \
			printf '      %s (%s)\n' "$$dir" "$$(du -sh "$$dir" 2>/dev/null | cut -f1)"; \
		fi; \
	done
	rm -rf $(DEV_STATE_DIRS)
	@echo "==> Recreating local development config"
	rm -f $(WEB_DIR)/.env.development
	rm -f $(SERVER_CONFIG_LOCAL)
	$(MAKE) .server-config .web-env

.server-config:
	@if [ ! -f "$(SERVER_CONFIG_LOCAL)" ]; then \
		echo "==> Creating $(SERVER_CONFIG_LOCAL) from $(SERVER_CONFIG_EXAMPLE)"; \
		sed 's|^#:schema \.\./\.\./schema/|#:schema schema/|' \
			"$(SERVER_CONFIG_EXAMPLE)" > "$(SERVER_CONFIG_LOCAL)"; \
	fi

# Regenerate the manifest JSON Schema and every example from the profile table
# in server/config/profiles.go. The schema is embedded, so a change to the doc
# comments needs the second pass to reach the generated TOML.
config-examples:
	@echo "==> Regenerating config schema and examples"
	cd $(SERVER_DIR) && $(GO) run ./tools/configgen >/dev/null && $(GO) run ./tools/configgen

.web-env:
	@printf '%s\n' \
	"# Proxy target for /api in the Vite dev server. Not exposed to the browser:" \
	"# the SPA uses relative URLs so dev is single-origin like production." \
	"API_URL=$(API_URL)" \
	> $(WEB_DIR)/.env.development
