# rc.1 sequence: the order of every session before the tag

Status: active, created 2026-10-28. The owner asked for one recorded order
across all open plans after the fix-before-release rule moved the storage
plans and server-structure Phase B in front of rc.1. Q0 done 2026-10-29
(#267). A1 done 2026-11-14 (#270). A2 done 2026-11-14 (#271). A3 done 2026-11-14 (#272). A4 done 2026-11-14 (#273). Section-A full run done 2026-11-14 on the Radxa
(Server gate and Docker E2E slices; the gate fix it needed is #274). B1 done
2026-11-14 (#275). B2 done 2026-11-14 (#276). Section-B2 full run done
2026-11-14 on the Radxa (Server gate and Docker E2E slices); two music
specs were flaky there, and the owner made fixing them row F1 (2026-11-15).
F1 did not reproduce in 23 `music.spec` and 18 `music-agent.spec` runs on
the Radxa (2026-11-16), and the owner set it aside for now; RL0 went ahead
of it. RL0 done 2026-11-16 (#278). RL1 done 2026-11-16 (#279). RL2 done 2026-11-16 (#PR). Next step: **RL3**.
Update the row of each step in the PR that lands it.

Goal: every open plan lands on `dev` in the order below, then the
release-hardening promotion sessions S13–S15 tag rc.1. This plan owns the
order and only the order. Each step's scope, contracts, and evidence stay in
the plan it names.

## Non-goals

- Changing the scope of any plan. A step that finds missing work adds it to
  the plan that owns it, or adds a new row here with the owner's agreement.
- Parallel work. No two steps run at the same time (see Fixed contracts).

## Fixed contracts

- **Strictly serial.** Each step is one session and one PR into `dev`,
  unless its row says otherwise. Each step branches from the latest `dev`
  after the previous step has merged. Phase B moves import paths of the same
  packages the storage plans edit, so any overlap is a conflict.
- **Rules before the code they guard.**
  - The Atlas rule kinds from A2 exist before the storage plans need their
    guardrails. A storage guardrail is never written as a text snippet
    check; it is an Atlas rule or a test.
  - The database boundary from B2 is in force before the storage plans add
    SQL. That covers rule 1 and the rule 4 ceilings.
- **Plans name packages by their paths before B1.** After B1, a session reads
  those names through the B1 move table in
  [server-structure.md](server-structure.md). For example, `internal/queue`
  becomes `internal/ingest/queue`, `internal/artifact` becomes
  `internal/storage/artifact`, and `internal/db` becomes `internal/catalog`.
- **Status travels with the work.** The PR that lands a step marks its row
  here "Done YYYY-MM-DD (#PR)" and updates the owning plan's `Status:`.
- **Release notes travel with the work.** A step with a user-visible change
  appends its lines to `docs/releases/v26.1.0-rc.1.md` and `.zh-cn.md`, using
  registry terms.
- **Found problems are fixed, not deferred.**
  ([rc-release.md](rc-release.md), owner rule 2026-10-28.) A problem a step
  finds is fixed in that step, or it becomes a new row here with the owner's
  agreement. It is never pushed past the tag.
- **Local evidence per step; PR CI once, at the end** (owner decision
  2026-11-14). PR CI is too slow to gate every step, and so is the full
  suite.
  - *Per step:* the narrowest checks for the diff
    ([lumilio-select-checks](../../../.agents/skills/lumilio-select-checks/SKILL.md))
    and `task atlas:check` on the Mac, plus every test the step adds or
    changes, run on the Linux test host when the Mac cannot run it.
  - *Per section:* the full Server gate (`server/app` and the Linux
    volume-semantics tests included) and the Docker E2E slices on the
    owner's amd64 Linux test host through a Docker context, with images
    built there. Sections end at A4, B2, RL4, RA4, SC4, B8, and B14. A
    failure is bisected over that section's squash commits and fixed on top
    before the next section starts.
  - A red PR check does not block a merge.
  After B14, PR CI runs once on the final head, and S13 fixes everything it
  finds, including what only CI can see (Windows Desktop, the macOS
  volume-semantics job, runner timeouts such as `server/app` on Windows,
  480 s of Go's 600 s default on 2026-11-14). A step whose local gate
  cannot run a package says so in its PR.
- **The order changes only with the owner.** A session that finds a
  dependency this order misses stops and asks.
- **No data migration.** No production install exists. Dev and demo
  environments are rebuilt (`.local/dev`, then `task web:demo:seed`) after
  RL3 and again whenever a later step changes the baseline or the private
  tree.

## Order

| Step | Session | Plan | Waits for | Why here |
| --- | --- | --- | --- | --- |
| Q0 | **Done 2026-10-29 (#267).** Merge #267: lease mitigation, postmortem 0004, the storage and sequence plans, and the owner's server-structure revision | — | — | Every later step reads these plans from `dev` |
| A1 | **Done 2026-11-14 (#270).** Typed facts and path-based groups | [atlas-rules.md](atlas-rules.md) | Q0 | Tools only; starts the rule engine |
| A2 | **Done 2026-11-14 (#271).** Static rules: `calls`, `constructs`, `forbid-types`, `forbid-imports`, `tables`, `tx-purity`, `budgets` | [atlas-rules.md](atlas-rules.md) | A1 | `tx-purity` and `calls`/`constructs` are the guardrails RA and RL need |
| A3 | **Done 2026-11-14 (#272).** Behaviour tests, relocated lints, switch; `architecturecheck` deleted | [atlas-rules.md](atlas-rules.md) | A2 | Leaves one gate before any code moves |
| A4 | **Done 2026-11-14 (#273).** Views that verify arrows; `movecheck` and `split-report` | [atlas-rules.md](atlas-rules.md) | A3 | Every B session's evidence |
| B1 | **Done 2026-11-14 (#275).** Infrastructure moves, `server/platform` gone | [server-structure.md](server-structure.md) | A4 | Storage code is then written once, at its final path (`catalog`, `storage/artifact`, `ingest`, `work`) |
| B2 | **Done 2026-11-14 (#276).** Typed catalog handles; `catalog` builds the River driver; database boundary rule 1; rule 4 ceilings recorded | [server-structure.md](server-structure.md) | B1 | New storage SQL obeys the boundary from its first line |
| F1 | **Set aside by the owner 2026-11-16; open.** No reproduction in 23 `music.spec` and 18 `music-agent.spec` runs on the Radxa: on a fresh stack, under concurrent `@smoke` load, and while the Mac's audio device list changed. Probes ruled out a buffering stall (the whole track was buffered every time) and a slow refine (0.4–0.9 s). Lead: the Playwright browser on the Mac plays through the real audio device, which CI runners do not have. Music E2E flakes from the section-B2 run, both on the first Radxa stack, both passing on retry and clean on a fresh stack: (1) `web/e2e/specs/music.spec.ts:136` (`@smoke`, slice `browser`): after the SPA link back to `/music` the `<audio>` `src` is kept but `currentTime` stays exactly at the pre-navigation position (0.02322 s) for the whole 5 s poll, which is the stalled-playback symptom of [postmortem 0003](../../../.agents/postmortems/0003-web-media-url-switched-representation.md); (2) `web/e2e/specs/music-agent.spec.ts:135` (`@agent-runtime`): after Refine, the second "Music selection" region does not render within 5 s. Find each cause before touching a timeout: a product race is fixed in the product with a failing-first test, and a test that waits on the wrong signal waits on the right one. Evidence: both slices run repeatedly on the Radxa (`--repeat-each`) with no flaky result | — | B2 | A section-end failure is fixed before the next section starts; owner decision 2026-11-15: fixed before rc.1 |
| RL0 | **Done 2026-11-16 (#278).** Layout Phase 0: session-leak test, single-owner rule (Atlas `calls`/`constructs`, report mode), private-path inventory | [repository-layout.md](repository-layout.md) | F1 | Locks the failures first |
| RL1 | **Done 2026-11-16 (#279).** Layout Phase 1: the `storage/layout` package | [repository-layout.md](repository-layout.md) | RL0 | |
| RL2 | **Done 2026-11-16 (#PR).** Layout Phase 2: every caller moved; single-owner rule fails on violations | [repository-layout.md](repository-layout.md) | RL1 | |
| RL3 | Layout Phase 3: catalog stores artifact identities; face crops published as artifacts; rebuild `.local/dev` | [repository-layout.md](repository-layout.md) | RL2 | RA3 and SC need `cache/artifacts/` and `user/` |
| RL4 | Layout Phases 4–5: policies (rotation, cleaner, `work/` sweep, validation) and docs | [repository-layout.md](repository-layout.md) | RL3 | Two small phases, one PR |
| RA0 | Access Phase 0: a failing test per audited path, plus the stress test | [repository-access-simplification.md](repository-access-simplification.md) | RL4 | The mitigation still ships until here |
| RA1 | Access Phase 1: lock order in `architecture.md`; context-free acquisition removed; Atlas rule against unbounded waits | [repository-access-simplification.md](repository-access-simplification.md) | RA0 | |
| RA2 | Access Phase 2: busy maps to retry; fail-fast handles; maintenance barrier | [repository-access-simplification.md](repository-access-simplification.md) | RA1 | SC's drain is written against fail-fast handles |
| RA3 | Access Phase 3: nothing waits inside the catalog transaction; `tx-purity` clean for face apply | [repository-access-simplification.md](repository-access-simplification.md) | RA2, RL3 | Face code is fixed before B6 moves it |
| RA4 | Access Phases 4–5: short handle lifetimes; startup requeue of orphaned River jobs | [repository-access-simplification.md](repository-access-simplification.md) | RA3 | |
| SC0 | Sidecars Phase 0: fresh-catalog loss test; path-limit rule | [portable-asset-sidecars.md](portable-asset-sidecars.md) | RA4 | |
| SC1 | Sidecars Phase 1: format, writer, reader | [portable-asset-sidecars.md](portable-asset-sidecars.md) | SC0 | |
| SC2 | Sidecars Phase 2: outbox triggers in the baseline, commit-woken drain under `work/` | [portable-asset-sidecars.md](portable-asset-sidecars.md) | SC1 | Rebuild `.local/dev` (baseline changed) |
| SC3 | Sidecars Phase 3: read when an asset is created; the Phase 0 test is inverted | [portable-asset-sidecars.md](portable-asset-sidecars.md) | SC2 | |
| SC4 | Sidecars Phases 4–5: lifecycle integration; copy, docs, and the superseding decision | [portable-asset-sidecars.md](portable-asset-sidecars.md) | SC3 | User-visible: release notes and adopt/open disclosures |
| B3 | `music`, `species` | [server-structure.md](server-structure.md) | SC4 | Capability moves start once the storage code is settled |
| B4 | `auth` | [server-structure.md](server-structure.md) | B3 | |
| B5 | `system`, `lumen` | [server-structure.md](server-structure.md) | B4 | |
| B6 | `places`, `people` | [server-structure.md](server-structure.md) | B5 | Moves the face code RA3 fixed |
| B7 | `search/indexing` | [server-structure.md](server-structure.md) | B6 | |
| B8 | `duplicates`, `stacks`, `sharing`; the rest becomes `assets`; `internal/service` deleted | [server-structure.md](server-structure.md) | B7 | Moves the rating and tag writers after SC2 put triggers on them |
| B9 | OpenAPI schema names; `tools/openapi` | [server-structure.md](server-structure.md) | B8 | The contract settles before rc.1 freezes it |
| B10 | Huma foundation; pilot of 11 operations | [server-structure.md](server-structure.md) | B9 | |
| B11a–g | One transport package per capability, one PR each | [server-structure.md](server-structure.md) | B10 | B11e and B11f carry RA2's busy mapping and RA4's upload streaming into Huma |
| B11h | swag deleted; boundary rule 2 for all of `api/` | [server-structure.md](server-structure.md) | B11g | |
| B12 | Storage root split, cut chosen from `split-report` on the post-plan code | [server-structure.md](server-structure.md) | B11h | `storage/layout` is already the cleanest leaf |
| B13 | Generated rows leave capability APIs, one PR per capability; the first PR splits `dbtypes`; `storage` goes after B12 | [server-structure.md](server-structure.md) | B12 | |
| B14 | Size ceilings; decisions extracted; server-structure and atlas-rules completed | [server-structure.md](server-structure.md) | B13 | |
| S13 | Promote: blocker gate, smoke checklist, #210 body, CI green on the final head; **Edwin merges** | [release-hardening.md](release-hardening.md), [rc-release.md](rc-release.md) Phase 2 | B14 | Everything above is on `dev` |
| S14 | Tag `v26.1.0-rc.1` after Edwin confirms in writing | [rc-release.md](rc-release.md) Phase 3 | S13 | |
| S15 | Post-tag smoke and compatibility fixture; release plans and this plan completed | [rc-release.md](rc-release.md) Phases 3–4 | S14 | |

## Why this order (decided 2026-10-28)

- **Atlas Phase A comes first.** It touches only tools, and it supplies three
  of the rule kinds the storage plans rely on:
  - `tx-purity` would have caught the face-crop open inside the writer
    transaction;
  - `calls` and `constructs` make `storage/layout` the only builder of
    private paths;
  - `calls` also limits who may wait on a lease.

  Writing these as `architecturecheck` snippets first would be throwaway
  work.
- **B1 and B2 come before the storage plans; B3–B14 come after them.**
  - B1 and B2 are mechanical: import paths and typed handles. Once they
    land, the storage plans put new code in its final packages and under the
    database boundary.
  - The capability moves (B3–B8) and the transport rewrite (B9–B11) move code
    that the storage plans are still changing, so they wait until it settles.
  - B12 was designed to cut the post-plan storage root, and B13 for
    `storage` needs B12.
- **Repository access comes before sidecars.** This reverses the earlier
  layout-plan note. RA2 makes handles fail fast, so the sidecar drain is
  written once against the final handle model. The deadlock mitigation from
  #267 covers the gap until RA lands.
- **Phase B still lands before rc.1.** B9–B11 change OpenAPI schema names and
  tighten request and response schemas. rc.1 freezes the public contract, so
  those changes belong before it.

## Validation boundaries

- Every row from Q0 to B14 is marked Done with its PR, in order.
- The validation boundaries of each named plan hold on `dev` at B14. Each
  plan except `rc-release.md`, `release-hardening.md`, and
  `rc-smoke-checklist.md` has been completed: its decisions are extracted and
  its file is deleted.
- S13–S15 run per their plans. This plan is deleted at S15.
