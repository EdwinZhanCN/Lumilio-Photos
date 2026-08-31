# Agent Harness

The loops, memories, and gates that let coding agents work in this repository
without losing decisions, repeating incidents, or skipping verification. This
file is the reference for the harness itself. Repository rules stay in
[AGENTS.md](../AGENTS.md); module contracts stay in their owning docs.

## Design rules

- **One home per fact.** Every rule, contract, or procedure lives in exactly
  one place; everything else links to it. A rule stated twice will drift.
- **A load-bearing rule is either machine-gated or written in a memory agents
  read.** Prose-only rules decay across sessions; promote the ones that must
  hold to CI gates.
- **Three memories plus skills, nothing else.** Exec plans track work,
  decision records keep the why, postmortems keep escaped failures, skills
  keep procedures. Resist inventing new categories.
- **Agents cannot read the owner's Obsidian vault.** Strategic and personal
  ADRs live there by design. Any rationale a future agent needs in order not
  to re-litigate a settled decision must exist in this repository.
- **Skills own the how; internal docs own the what must hold.** When a
  reference doc carries a step sequence, move it into a skill and leave the
  contract plus a link.

## Layout

```
.agents/
  skills/       procedures: how to run a recurring workflow, end to end
  decisions/    project-coupled decision records: why things are this way
  postmortems/  escaped bugs: why the process let them through, and the
                guardrails added
```

`.claude/skills` is a symlink to `.agents/skills` so Claude Code loads the
same tree. `.agents/` stays the real home.

Existing homes keep their jobs: `docs/` holds current-state
reference docs, `docs/exec-plans/active/` tracks large
prospective work (plans are deleted at completion after decision extraction;
there is no completed archive), and root `AGENTS.md` remains the single entry
point that links everything here.

| Content | Home |
| --- | --- |
| Standing orders for every session | root `AGENTS.md` |
| Current-state contracts and maps | `docs/*.md`, `web/ARCHITECTURE.md` |
| Large prospective work | `exec-plans/active/`, deleted at completion |
| How to run a recurring workflow | `.agents/skills/lumilio-<name>/SKILL.md` |
| Why a project-coupled decision stands, and what it beat | `.agents/decisions/` |
| Why an escaped bug got through, and what now stops it | `.agents/postmortems/` |
| Strategic and personal ADRs | owner's Obsidian vault, outside the repo |

## Skills

A skill is one procedure an agent follows end to end: trigger, steps,
verification, and the mistakes it prevents. Each skill is
`.agents/skills/lumilio-<name>/SKILL.md` with YAML frontmatter whose
`description` states *when to invoke*. The body is imperative: numbered
steps, exact commands, the verification that proves the step worked, and
known failure modes.

`AGENTS.md` lists every skill with its one-line trigger.

### Current set

| Skill | Procedure it owns |
| --- | --- |
| [lumilio-select-checks](../.agents/skills/lumilio-select-checks/SKILL.md) | map a diff to the narrowest `task` targets before push |
| [lumilio-api-contract-change](../.agents/skills/lumilio-api-contract-change/SKILL.md) | DTO/annotation change, `task dto`, `schema.d.ts` verification, cast triage |
| [lumilio-write-a-test](../.agents/skills/lumilio-write-a-test/SKILL.md) | pick the test layer, file name, GPU self-skip, prove it can fail |
| [lumilio-integration-spec](../.agents/skills/lumilio-integration-spec/SKILL.md) | Vitest component/flow specs: MSW, helpers, locators, import gotchas |
| [lumilio-e2e-spec](../.agents/skills/lumilio-e2e-spec/SKILL.md) | Playwright locator order, i18n-safe names, forbidden aria-label hooks |
| [lumilio-frontend-i18n](../.agents/skills/lumilio-frontend-i18n/SKILL.md) | extract-then-fill workflow and canonical bilingual product terminology registry |
| [lumilio-e2e-environment](../.agents/skills/lumilio-e2e-environment/SKILL.md) | E2E stack lifecycle, seed variants, slice selection, readiness |
| [lumilio-remote-qualification](../.agents/skills/lumilio-remote-qualification/SKILL.md) | remote hardware qualification (Radxa X4), execution budgets, and destructive resilience |
| [lumilio-lumen-fixtures](../.agents/skills/lumilio-lumen-fixtures/SKILL.md) | record real Hub responses, replay them keyless in CI |
| [lumilio-z-index](../.agents/skills/lumilio-z-index/SKILL.md) | three-rule stacking strategy and token scale |
| [lumilio-add-task-target](../.agents/skills/lumilio-add-task-target/SKILL.md) | Taskfile placement, naming, when a `ci:*` orchestrator is earned |
| [lumilio-feature-doc](../.agents/skills/lumilio-feature-doc/SKILL.md) | `doc.ts` authoring, `{@link}`/import pairing, `doc.md` regeneration |
| [lumilio-pin-reconcile](../.agents/skills/lumilio-pin-reconcile/SKILL.md) | assets.lock / lumen.lock bump, verify, PR shape |
| [lumilio-exec-plan](../.agents/skills/lumilio-exec-plan/SKILL.md) | plan creation criteria, skeleton, completion extraction |

## Decision records

A decision record keeps a project-coupled decision from being silently
re-litigated: what was decided, and — mandatory — what it beat. Important
strategic decisions live in the owner's Obsidian vault; a decision earns a
repo record when future work in this codebase depends on its rationale.

One flat directory, `.agents/decisions/YYYY-MM-DD-topic.md` (date of the
decision), no lifecycle folders:

```markdown
# Decision: <title>

Status: implemented | rejected — <one line>

## Problem
## Decision
## Alternatives considered
```

- `## Decision` states shipped reality in the present tense.
- `## Alternatives considered` is mandatory: each genuine alternative and why
  it lost. A decision recorded without what it beat invites re-litigation.
- A decision is never edited into a different decision. Supersede with a new
  file and cross-link both.
- `Status: rejected` records a proposal that was considered and declined,
  kept while its rationale prevents a tempting mistake.

When to write one: a non-trivial, project-coupled change — behavior, a
cross-module contract, an on-disk or wire format, process or tooling, a
dependency or pin policy — records its decision in the same PR. Work driven
by an exec plan keeps interim decisions inside the active plan; completing
the plan extracts the durable ones here
([lumilio-exec-plan](../.agents/skills/lumilio-exec-plan/SKILL.md)).
Mechanical or local edits are exempt.

## Postmortems

A postmortem is written when a bug escaped to a place it should not have
reached — a user, `main`, a release — and the escape is systemic: the
interesting part is why every safety net missed it, not the one-line fix.

`.agents/postmortems/NNNN-slug.md`, sequentially numbered:

```markdown
# Postmortem NNNN: <title>

## Executive summary
## What broke
## Why every net missed it
## Guardrails added
```

- The executive summary is one paragraph a busy reader absorbs in thirty
  seconds.
- `## Guardrails added` must link the real artifacts — the regression test,
  CI gate, or AGENTS.md rule merged because of this incident. A postmortem
  without a merged guardrail is not finished.
- An empty directory is a good sign, not a gap.

## Generated-artifact freshness

Rule: every checked-in generated artifact either has a CI freshness gate or a
named exception with a reason. `task verify:generated` regenerates and
asserts a clean tree (`git diff --exit-code`) in the `generated` CI job:

| Artifact | Generator | Gate |
| --- | --- | --- |
| `web/src/lib/http-commons/schema.d.ts`, OpenAPI spec and docs | `task dto` | `verify:generated` |
| `doc.md` siblings of feature `doc.ts` | `task web:docs` | `verify:generated` |
| Server config schema and examples | `task config:examples` | `verify:generated` |
| `assets.lock.json`, `lumen.lock.json` | reconcile/sync tools | already gated: `assets:check`, `lumen:check` |
| `web/src/wasm/*` bundles | `task wasm:blake3` | **exception**: Rust toolchain is not a CI baseline; PR review owns freshness |

## Testing doctrine

Linked from `AGENTS.md`; this section is the home.

- **Verify the world, not the self-report.** An acceptance check re-reads the
  file, re-queries the API, or re-renders the page externally. Grepping the
  agent's own output lets a cheating or confused agent pass. HTTP 200 is
  transport readiness, not application readiness.
- **Test the real entry path.** The built server binary boots a real TOML
  manifest and a genuinely missing config exits non-zero; the desktop build
  compiles and launches its embedded server. Hand-assembled compositions do
  not substitute for the shipped assembly.
- **A guard must be proven to fail.** When adding a regression test, introduce
  the regression, watch the test go red, revert. A test that cannot fail for
  its mechanism is a false guard; state the red run in the PR.
- **Record/replay at the expensive nondeterministic boundary.** Lumen
  inference (`siglip`, `face`, `ocr`, `bioclip`) records real Hub responses
  into reviewed fixtures and replays them keyless and GPU-less through
  `fakelumen`. Recording is explicit (`task lumen:record`), never implicit in
  CI. Until a recorded set is committed, replay serves the deterministic
  builtin embedding — same as the previous constant-vector fixture.
  Procedure: [lumilio-lumen-fixtures](../.agents/skills/lumilio-lumen-fixtures/SKILL.md).

## The obligation

The single rule that closes the loop, carried in `AGENTS.md`:

> A non-trivial change updates one memory in the same PR — the owning
> decision record, exec plan, or postmortem — and passes the gates its diff
> touches. Mechanical or local edits are exempt.

## Current state

The directories, the skills listed above, `.claude/skills` symlink, the
`AGENTS.md` routing, `verify:generated`, and Lumen record/replay exist.
`ci:*` names remain only for real orchestrators; workflows call module
targets directly. Postmortems accumulate as incidents happen.
