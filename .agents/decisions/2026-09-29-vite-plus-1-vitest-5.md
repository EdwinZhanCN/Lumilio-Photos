# Decision: Track Vite+ 1.0 with Vitest 5

Status: implemented — `web/` and `site/` are on Vite+ 1.0.0; `web/` is on
Vitest 5.0.1.

## Problem

Vite+ 0.2.x bundled Vitest 4 and predated the standalone `vp node` command that
`web/taskfile.yml` calls. Staying on the alpha line left the Taskfile targets
that use `vp node` unusable with an npm-installed `vp`.

## Decision

`web/pnpm-workspace.yaml` pins `vite-plus`, the `vite` alias to
`@voidzero-dev/vite-plus-core`, and `vitest` plus the `@vitest/*` packages to the
versions bundled by the Vite+ release, in one catalog. After any Vite+ bump, run
`vp migrate` from `web/` (with the newer global `vp`) before touching
dependencies by hand so the Vitest pin stays aligned.

Vitest 5 defaults changed and the specs follow the new meaning:

- `toHaveTextContent` is an exact match. Assert on a fragment of larger text
  with `toMatchTextContent`.
- Browser locators are strict (exact, case-sensitive). Pass `exact: false` when
  the accessible name is intentionally a fragment.
- `clearMocks: false` in `web/vite.config.ts` keeps v4 mock-history behavior
  until tests stop relying on calls from setup or earlier tests; remove it then.

`desktop/frontend` uses plain Vite and does not depend on `vite-plus`.

## Consequences

- Use the standalone `vp` (`curl -fsSL https://vite.plus | bash`, as
  `voidzero-dev/setup-vp` does in CI). The npm-installed `vp` has no `vp node`
  or `vp env`.
- `test.browser.api` is deprecated in Vitest 5; `vite.config.ts` line 191 builds
  projects dynamically, so review inheritance and `sharedViteServer` before
  removing compatibility settings.
