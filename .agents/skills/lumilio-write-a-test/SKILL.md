---
name: lumilio-write-a-test
description: Use when adding a test in Lumilio Photos (web/ or server/) —
  picks the correct test layer, file name, and directory so the right runner
  owns it, handles GPU/WebGL self-skip, and proves the new test can fail.
---

# Write A Test In The Right Layer

The frontend layer table is a contract owned by
[FRONTEND.md](../../../site/docs/internal/FRONTEND.md). The rationale is the
[test-layer assignment decision](../../decisions/2026-08-14-frontend-test-layer-assignment.md).
This skill is the selection, placement, and verification procedure. Do not
invent other file conventions.

Flow specs: [lumilio-integration-spec](../lumilio-integration-spec/SKILL.md).
Playwright specs: [lumilio-e2e-spec](../lumilio-e2e-spec/SKILL.md). E2E stack:
[lumilio-e2e-environment](../lumilio-e2e-environment/SKILL.md).

## Pick the web layer

Ask what the test must exercise, then let the file name and directory select
the runner (`web/vite.config.ts` `test.projects`):

| Must exercise | File | Runner |
| --- | --- | --- |
| React-free rules, codecs, reducers, algorithms | `*.test.ts` | `unit` (Node, no DOM) |
| One component or small tree | `*.test.tsx` | `integration` (real Chromium) |
| A flow: routes, Query, HTTP via MSW | `flows/<flow>/*.spec.tsx` | `integration` + MSW |
| Worker, WASM, SSE, Canvas/WebGL | `*.browser.test.ts` | `browser` (Chromium) |
| Real API, DB, storage, queues | `web/e2e/specs/*.spec.ts` | Playwright |

Placement rules that bite:

- There is no `*.integration.test.tsx`. Flow specs and E2E specs both say
  "spec" — directory, extension, and runner disambiguate.
- Core-browsing UI (full `AssetBrowser`: WASM layout, virtualization, URL
  state, real selection) belongs to Playwright E2E, not the integration
  project. Extract pure logic to a unit test instead of forcing a real render.
- The `unit` project excludes `*.browser.test.ts` and `src/workers/**` on
  purpose — an accidental browser import should fail, not hide.
- Tests stay beside the implementation they characterize. Feature-specific
  styles move with the implementation.

`src/workers/hash.test.ts` is a browser contract test selected through the
worker directory even though it uses the shorter `.test.ts` suffix. The
normal Web gate covers small files and the backend-compatible quick-hash path
for files over 100 MiB. The 20 × 50 MiB throughput case is intentionally
excluded; run it with `vp run test:hash-perf` from `web/` when changing
hash-worker performance.

## GPU / WebGL capability tests

A `*.browser.test.ts` gets a real Worker, WASM, Canvas and WebGL context, but
**headless Chromium falls back to SwiftShader, whose WebGL is disabled on
Apple Silicon**, so a WebGL2-dependent test cannot get a context headless on
an M-series Mac (and usually not on a GPU-less CI runner either). Such a test
must:

1. **Guard the capability** at runtime and skip, never fail: probe with a
   helper like `webgl2Available()`
   (`new OffscreenCanvas(1,1).getContext("webgl2")`) and wrap the suite in
   `describe.skipIf(!webgl2Available())`. A skipped capability is correct; a
   suite that fails to launch a browser is not.
2. **Run headed locally** to actually exercise it. The `browser` project reads
   `STUDIO_GPU=true` (see `vite.config.ts`) and switches to `headless: false`.
   `STUDIO_GPU=true vp test <files>` from `web/` runs them on the real GPU;
   the default stays headless so CI can always launch (those suites skip).

Non-GPU capability tests (Canvas 2D, Worker, WASM) run headless everywhere
and need no guard — keep them assertable in CI.

## Server tests

Go tests live beside the package they exercise and run through
`task server:test` (it exports the cgo allowlist media dependencies need on
macOS). Direct `go test` is acceptable only when you preserve the same
environment: `cd server && go test -tags=sqlite_fts5 ./...`. Run `gofmt` on
changed Go files.

Generated config examples are protected by a golden test — change
`server/config/profiles.go` and run `task config:examples`, never hand-edit.
Reproduce the CI Server gate with `task server:test:ci` (clean caches).

Lumen inference is never live in unit tests. Queue/service tests stub
`LumenService`; the isolated E2E stack talks to `fakelumen`
([lumilio-lumen-fixtures](../lumilio-lumen-fixtures/SKILL.md)). The tensor
conformance test is opt-in (`LUMILIO_LUMEN_CONFORMANCE_ADDR`) and is not a CI
gate.

## Prove the guard

A regression test must be able to fail for its mechanism:

1. Introduce the regression.
2. Watch the test go red.
3. Revert the regression, keep the test.
4. State that red run in the PR.

Assert external state (re-read the file, re-query the API, re-render the
page), not the implementation's self-report. A timeout or a keyword probe on
output is not proof. HTTP 200 is transport readiness, not application
readiness.

## Verify

Run the focused file while editing, then the gate for its layer:

```sh
cd web && vp test run path/to/changed.test.ts   # or .tsx / .browser.test.ts
task web:test                                   # typecheck, lint, boundaries, unit/integration/browser
task server:test                                # Go
```

Bundling or production-only browser paths also run `vp run test:bundle` from
`web/`. Playwright slices need the E2E stack — do not fold them into
`task web:test`.
