---
name: lumilio-add-task-target
description: Use when adding or changing a Taskfile target or its CI wiring in
  Lumilio Photos — module vs root placement, naming, when a ci:* contract is
  earned, and the workflow path-filter update that must land in the same PR.
---

# Add Or Change A Task Target

The ownership boundary is fixed: `.github/workflows/*.yml` owns triggers, path
filters, runners, native dependencies, credentials, caches, Buildx, and
artifacts; Taskfiles own repository commands, working directories,
sequencing, flags, and CI environment variables. Workflows call **module
targets directly** (`task web:test`, `task server:test:ci`,
`task web:e2e:up`). A root `ci:*` name exists only when it orchestrates more
than one module target. One-to-one `ci:` wrappers are forbidden — they were
the previous source of Taskfile bloat.

Keep the root Taskfile from growing one wrapper per convenience. Every
target must earn its placement.

## Placement decision

1. **Module Taskfile** (`server/`, `web/`, `desktop/`, `site/` — plain names):
   the command concerns one module and runs in its directory. This is the
   default home.
2. **Root Taskfile** (colon-namespaced): only cross-module orchestration
   (`dto`, `dev`, `verify:generated`), repository-level tools (locks,
   devstate, wasm, `lumen:record`), and the few `ci:*` orchestrators.
3. **`ci:*` contract**: create one only when a workflow needs a *sequence* no
   module target already is. Current set: `ci:architecture`, `ci:site`,
   `ci:desktop:panel`, `ci:desktop:native`. Do not add `ci:web:test` for
   `web:test`.

## Naming

`namespace:action[:variant]` in kebab-case (`web:test:auth-totp`,
`assets:reconcile`). Destructive targets take a `prompt:` and, where the tool
supports it, a confirmation env (`dev:purge` is the template). Internal
helpers set `internal: true` and stay hidden from `task --list`.

Do not wrap a `package.json` / `vp run` script that is already the right
local command (`e2e:logs`, `test:watch`). Agents and humans run those with
`vp run` from `web/`.

## Same-PR obligations

- A new or renamed CI-relevant target updates the affected workflow path
  filters in the same change. Web E2E filters must include `web/taskfile.yml`;
  Site/Server/Web filters must include the root `taskfile.yml` when its
  orchestration changes.
- A removed target: search workflows, skills, AGENTS.md, CONTRIBUTING.md, and
  `site/docs` for the exact name before deleting.
- `compose:test` must cover every Compose file the repo ships, including
  overlays (`compose.ci.yml`, `compose.record.yml`).

## Verify

- `task --list` from the root renders the target with a sensible description.
- Run the target once (or its cheapest variant) and confirm the working
  directory and env are right.
- If CI-wired: the workflow references the exact target name, and the path
  filter reaches every file the target reads.
