# Decision: Frontend test layers and the core-browsing E2E assignment

Status: implemented

This record makes the test-layering decision resolvable in-repo; earlier
prose cited out-of-repo ADRs (ADR-005/006 in the owner's vault). The layer
table itself lives in
[FRONTEND.md](../../docs/FRONTEND.md); the placement procedure
is [lumilio-write-a-test](../skills/lumilio-write-a-test/SKILL.md).

## Problem

A React SPA over real browser capabilities (Workers, WASM layout, WebGL) and
a real backend needs a test taxonomy that says where any given test belongs.
The expensive failure mode is a middle layer that renders the heaviest UI
with mocked everything: high effort, brittle, low fidelity.

## Decision

Five layers, where file name, extension, and directory select the runner and
dependency boundary (`web/vite.config.ts` `test.projects`): `unit`
(`*.test.ts`, Node, no DOM), `integration` components (`*.test.tsx`, real
Chromium), `integration` flows (`flows/<flow>/*.spec.tsx`, Chromium + MSW),
`browser` capabilities (`*.browser.test.ts`), and Playwright E2E
(`web/e2e/specs/*.spec.ts`, real services).

Core-browsing UI — the full `AssetBrowser` with WASM justified layout,
viewport virtualization, URL/route state, and real selection — is assigned to
Playwright E2E against real services. It is deliberately kept out of the
Vitest integration project; pure logic extracted from it tests at the unit
layer. Component and flow tests run in real Chromium (no jsdom/happy-dom), so
layout, CSS, observers, storage, and events are real implementations.

## Alternatives considered

**Render `AssetBrowser` in the Vitest integration project** — rejected: a
real render there is high-effort and brittle for low fidelity (WASM layout,
virtualization, and route state all need the real stack to mean anything).

**jsdom/happy-dom environments** — rejected: they fake the exact surfaces
these tests exist to exercise; real Chromium via the Playwright provider
keeps component semantics honest.

**One generic `*.test.tsx` convention with per-test configuration** —
rejected: encoding the layer in the file name and directory lets the runner
enforce the dependency boundary (the `unit` project excludes browser
dependencies so an accidental import fails instead of hiding).
