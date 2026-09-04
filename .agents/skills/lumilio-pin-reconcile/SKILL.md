---
name: lumilio-pin-reconcile
description: Use when bumping, verifying, or reviewing assets.lock.json or
  lumen.lock.json in Lumilio Photos — the pin, the generator, the CI gate,
  and the PR shape for a lock update.
---

# Reconcile A Cross-Repository Pin

Two producer pins live at the repository root. Never edit the lock files or
their generated consumers by hand. Contracts:
[test-assets.md](../../../docs/test-assets.md),
[lumen-catalog.md](../../../docs/lumen-catalog.md).

| Lock | Offline CI gate | Prove against GitHub | Reconcile |
| --- | --- | --- | --- |
| `assets.lock.json` | `task assets:check` | `task assets:verify` | `task assets:reconcile` (`RELEASE=` optional) |
| `lumen.lock.json` | `task lumen:check` | `task lumen:verify` | `task lumen:sync` (`RELEASE=vX.Y.Z` to select) |

`task dependencies:verify` runs both remote proofs. GitHub Actions runs it
from `.github/workflows/dependency-contracts.yml` on lock/tool/catalog diffs.

## Assets

`assets.lock.json` pins `repository`, `revision` (full 40-char SHA),
`release`, default `profile`, and `manifestSha256`. Only `release` is chosen
by hand: reconcile picks the newest stable `assets-vX.Y.Z` (or the explicit
`RELEASE`), verifies that release's `release.json`, and writes
`release` + `revision` + `manifestSha256` together. Downgrades are rejected;
re-running is a no-op.

Materialization is on demand: `vp run assets:sync` (from `web/`, optional
`--profile`) shallow-fetches, sparse-checkouts, LFS-pulls, and verifies
sha256+bytes into `.cache/lumilio-assets/`. Never commit media files here.

## Lumen

`lumen.lock.json` is schema version 2. `release` is the selected Hub tag;
`revision`, manifest hash, generated catalog hash, and vendored control-proto
hash are derived. `task lumen:sync` writes the lock and
`desktop/internal/lumen/release_catalog.go` atomically after remote checks
succeed. If `control.proto` changed, regenerate Go bindings:

```sh
task desktop:proto:gen
```

`lumen:check` verifies catalog hash, vendored proto hash, and Compose preset
references with no network. It is the every-PR gate.

Upgrade order: publish the Hub tag, then `task lumen:sync RELEASE=<new-tag>`,
then commit lock + catalog + proto together.

## Verify

- Lock-only change: `task assets:check` and/or `task lumen:check`.
- After reconcile: `task assets:verify` / `task lumen:verify`, and include
  every generated file the tool rewrote in the same PR.
- Do not hand-edit `desktop/internal/lumen/release_catalog.go` or
  `desktop/internal/lumen/controlv1/control.proto`.
