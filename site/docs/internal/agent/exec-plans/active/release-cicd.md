# Release & CI/CD — first production release

Status: in progress (review done 2026-07-03; B1+B2 fixed 2026-07-03, B3/B4 open in the Lumen repos)
Scope: Lumilio-Photos (this repo) + coordination items in Lumen-SDK and Lumen-Hub.

## Product decision (fixed)

- Two supported distributions:
  1. **Desktop App** (macOS + Windows): Jupyter-Lab-style — Wails v3 tray supervises a private PostgreSQL + in-process `server/app`, UI in the user's real browser at `localhost:6680`. A lightweight Wails UI will later onboard storage location + external ML install.
  2. **Docker** (Linux recommended): compose stack (db / server / web-Caddy).
- ML is **external-only**: LAN nodes (Lumen-Hub, Rust/Burn) discovered over mDNS `_lumen._tcp`; discovery + connection management live in Lumen-SDK (Go).
- Windows/macOS are steered to Desktop because Docker there cannot host-network mDNS. Linux Docker can (`network_mode: host`). Nuance to keep in docs: only *discovery* needs multicast — the gRPC data plane works from a bridge network, so gateway-push or static node addresses can still serve Docker on any OS.

## Blockers found in review (fix before any release work)

- **B1 — `[lumen]` TOML is dead config.** ~~`initMLServices` calls `lumenconfig.LoadConfig("")`; `appConfig.Lumen` is never mapped into the SDK config.~~ **Fixed 2026-07-03**: `service.NewLumenServiceFromAppConfig` maps `config.LumenConfig` onto the SDK config (SDK defaults → `LUMEN_*` env for SDK-only knobs → app-owned discovery fields on top); the vestigial `connection_insecure` knob (no SDK TLS support, zero consumers) was removed.
- **B2 — disabling ML prevents boot.** ~~`NewLumenClient` error is fatal in `Run`.~~ **Fixed 2026-07-03**: `config.LumenConfig.Enabled()` (discovery on + a backend configured) gates a disabled no-op `LumenService` (`ErrLumenDisabled`, no tasks, zero nodes); server boots and degrades, classifier-prototype warmup is skipped. Verified live: boot with `LUMEN_DISCOVERY_ENABLED=false` serves `/api/v1/health`; default dev boot maps mDNS config into the real client.
- **B3 — Docker's default discovery path is dead end-to-end.** `server/Dockerfile` defaults `LUMEN_DISCOVERY_MDNS_ENABLED=false` + `LUMEN_DISCOVERY_HUB_URL=http://host.docker.internal:5866`; SDK `PushResolver` dials `ws://…/v1/nodes/watch`, which only legacy `cmd/lumenhubd` serves — the Wails **lumen-gateway** REST (`pkg/server/rest/routes.go`) has no watch route. Fix in Lumen-SDK: move the node-watch WS route into `pkg/server/rest` so gateway serves it (and/or PushResolver falls back to polling `GET /v1/nodes`).
- **B4 — mDNS TXT contract mismatch.** Hub advertises `uuid/status/version`; SDK reads `v/runtime/tasks/cap_hash`. Task hints are always empty; node Version/Runtime blank; everything rides on post-connect `StreamCapabilities`, which has **no retry** on failure. Fix in Lumen-Hub (emit `v`, `runtime`, `tasks` CSV) + Lumen-SDK (retry capability fetch with backoff).

## High-priority follow-ups (P1)

- **Static node config** (`lumen.nodes = ["host:port", …]`): a manual `NodeResolver` in Lumen-SDK; unblocks firewalled LANs, VLANs, WSL2, and Docker-anywhere. Wire a `discovery_static_nodes` field through Lumilio TOML.
- **Hub advertise-IP needs internet**: `detect_lan_ip()` UDP-connects `8.8.8.8`; offline LAN falls back to advertising loopback. Use `mdns-sd` addr-auto / interface enumeration.
- **macOS Local Network privacy**: the desktop `.app` runs mDNS queries in-process; generated Info.plist (`desktop/scripts/build-macos.sh`) lacks `NSLocalNetworkUsageDescription`. Add key + first-run UX note, verify on macOS 15.
- **Auth rate limiting** (tech-debt tracker): add shared limiter middleware for `login` / `passkeys/login` / `mfa/verify` before first public deployment.

## Workstreams

### W0 — contract + integration fixes (Lumen repos then Lumilio)
1. ~~Lumen-Hub: TXT keys `v/runtime/tasks`; addr-auto advertise.~~ **Done 2026-07-04** (`daemon/mdns.rs` `AdvertisedCapabilities`, `enable_addr_auto`, `ADVERTISE_IP` still overrides). Tag a beta.
2. ~~Lumen-SDK: watch route in shared REST; capability-fetch retry; static resolver.~~ **Done 2026-07-04**: `pkg/server/rest/node_watch.go` (shared by hubd + gateway; gateway go.mod now `replace => ../../`), `fetchCapabilitiesWithRetry` (5 attempts, Ready nodes published immediately), `StaticResolver` + `CompositeResolver` (backends now additive), `static_nodes` config + `LUMEN_DISCOVERY_STATIC_NODES`. Naming collision resolved 2026-07-04: `lumenhubd`→`lumengatewayd`, `lumenhub` CLI→`lumengateway` (dirs, binaries, ldflags paths, workflows, docs); "Lumen Hub" now only refers to the Rust inference server. Still open: remove dead `c.cancel` in `client.Start`. **Tag v1.3.0.**
3. ~~Lumilio: bump SDK, wire static nodes.~~ **Done 2026-07-04** (SDK v1.3.0 + Hub v0.1.0-alpha.2 released): `server/go.mod` → v1.3.0, `[lumen] discovery_static_nodes` + `LUMEN_DISCOVERY_STATIC_NODES`, `Enabled()` counts static as a backend, desktop go.mod tidied. Remaining for W2: Dockerfile discovery-mode docs + e2e smoke against a real hub. (B1 + B2 fixed 2026-07-03.)

### W1 — CI baseline — **Done 2026-07-05** (pending first Actions run to confirm green)
- `ci.yml`: dorny/paths-filter gates four jobs — server (ubuntu, libvips/libraw, `make server-test`), web (node 24 + corepack pnpm + Vite+ installer, `make web-test`), desktop (macos-15, brew vips/libraw, `make desktop-test`; PG test auto-skips), site (vitepress build).
- Makefile compose check is now lazy (`$(COMPOSE)` errors only when a docker target runs) so docker-free macOS runners work.
- Version stamping: `server/internal/version.Version` (ldflags `-X`), surfaced in `GET /api/v1/health`; `server/Dockerfile` + `web/Dockerfile` take `ARG VERSION=dev` (web → `VITE_APP_VERSION`); `build-macos.sh` stamps the same version it writes to the plist. Release workflows (W2/W3) must pass `VERSION=<git tag>`.

### W2 — Docker release (Linux) — **Done 2026-07-05** (pending first tag run)
- `release-docker.yml`: native per-arch builds (ubuntu-24.04 + ubuntu-24.04-arm, no QEMU) push `lumilio-{server,web,db}` to GHCR by digest, then a merge job assembles multi-arch manifests. Tags: `vX.Y.Z` (+ `X.Y` + `latest` for stable), `edge` on manual dispatch.
- `docker-compose.release.yml` (GHCR images, `LUMILIO_STORAGE` required, `LUMILIO_VERSION` pin) + `docker-compose.host-mdns.yml` Linux overlay (`network_mode: host` on server, db via `127.0.0.1:5433`, web upstream via host-gateway, needs compose ≥ 2.24 for `!reset`). All three discovery modes documented in the compose header.
- **Fixed en route**: root `docker-compose.yml` preloaded `pg_textsearch` which the db image never contained — db could not boot; removed. Search actually uses pg_trgm + `to_tsvector('simple')` only, so `lumilio-db` = pgvector base image, and `build-postgres.yml` dropped its pg_textsearch/SCWS/zhparser steps (desktop bundle likewise only needs PG + pgvector).

### W3 — macOS desktop release — **Done 2026-07-05** (arm64; pending first tag run)
- `release-desktop-macos.yml` on tag: calls `build-postgres.yml` (`workflow_call` added) for a fresh PG artifact → `fetch-resources.sh` → web SPA (needs GH Packages token for docts) → `build-macos.sh arm64 --dmg` (ad-hoc sign inside) → DMG attached to the GitHub Release with Gatekeeper + local-network instructions.
- Info.plist now carries `NSLocalNetworkUsageDescription` + `NSBonjourServices _lumen._tcp` + `LSUIElement` (tray-only, matches ActivationPolicyAccessory).
- amd64 deferred: `fetch-resources.sh` pins arm64 ffmpeg/ffprobe only; add Intel URL+SHA pins + `macos-15-intel` matrix entry when wanted. Notarization = future paid-account upgrade.

### W4 — Windows desktop (unblocked: BM25/zh-seg removal killed the PG spike)
- ✅ **PG bundle (2026-07-05)**: `build-postgres.yml` `build-windows` job — EDB official binaries zip + pgvector via MSVC nmake; pg_trgm ships in the zip. Artifact `postgres-windows-amd64` matches the supervisor's `GOOS-GOARCH` resource lookup. Verify with a dispatch run.
- **Supervisor port** (audit 2026-07-05 — smaller than feared; quarantine + resources are already build-tagged):
  1. `lock_other.go` is an always-error stub → real Windows single-instance lock (`LockFileEx` or exclusive-create pidfile).
  2. Windows PostgreSQL has **no Unix sockets** → `paths.go` `SocketDir` + `config.NewDesktopConfig` (DB host = socket dir) must switch to TCP `127.0.0.1:<port>` on Windows; `postgres.go` pg_ctl/initdb args likewise.
  3. `paths.go` app-data root → `%LOCALAPPDATA%\Lumilio Photos` on Windows.
- **CGo toolchain (main risk now)**: libvips + libraw via MSYS2 mingw-w64 on a windows runner; add an experimental (continue-on-error) CI job to gather signal before committing to it.
- fetch-resources Windows variants (gyan.dev ffmpeg, exiftool Windows zip), Wails tray on Windows (proven by lumen-gateway), NSIS installer or portable zip, SmartScreen doc.
- Recommendation stands: v1.0 = macOS Desktop + Linux Docker; Windows = v1.x beta channel.

### W5 — release checklist (per-release)
- Lumen-Hub tag published + `manifest.json` reachable; `lumilio.org/lumen/install.{sh,ps1}` worker serving latest.
- registry.lumilio.org / cdn.lumilio.org (studio plugin registry) reachable or runtime flag off.
- Auth limiter enabled; setup wizard rotates DB password (verify on fresh volume).
- Fresh-install e2e on each artifact: boot → wizard → import → (with a hub on LAN) semantic/face/ocr jobs complete.
- Docs: install pages for both modes incl. discovery modes matrix + Gatekeeper/SmartScreen notes.

## Validation

- B1/B2: unit test that TOML `[lumen]` values reach the SDK config; boot test with `discovery_enabled=false`.
- B3: compose up (bridge) + gateway on host → nodes appear in `/api/v1/capabilities`.
- Host-network overlay on a Linux box with a real hub → mDNS nodes appear.
- macOS DMG on a clean machine: Open Anyway → local-network prompt → hub discovered.
