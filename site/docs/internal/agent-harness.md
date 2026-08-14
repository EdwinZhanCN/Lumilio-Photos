# Agent Harness

Status: proposed; the bootstrap directories and entry-point links described
below are not implemented yet.

The loops, memories, and gates that let coding agents work in this repository
without losing decisions, repeating incidents, or skipping verification. This
file is the reference for the harness itself. Repository rules stay in
[AGENTS.md](../../../AGENTS.md); module contracts stay in their owning docs.

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

## Layout

```
.agents/
  skills/       procedures: how to run a recurring workflow, end to end
  decisions/    project-coupled decision records: why things are this way
  postmortems/  escaped bugs: why the process let them through, and the
                guardrails added
```

Existing homes keep their jobs: `site/docs/internal/` holds current-state
reference docs, `site/docs/internal/exec-plans/` tracks large prospective
work, and root `AGENTS.md` remains the single entry point that links
everything here.

| Content | Home |
| --- | --- |
| Standing orders for every session | root `AGENTS.md` |
| Current-state contracts and maps | `site/docs/internal/*.md`, `web/ARCHITECTURE.md` |
| Large prospective work | `exec-plans/active/` |
| How to run a recurring workflow | `.agents/skills/<name>/SKILL.md` |
| Why a project-coupled decision stands, and what it beat | `.agents/decisions/` |
| Why an escaped bug got through, and what now stops it | `.agents/postmortems/` |
| Strategic and personal ADRs | owner's Obsidian vault, outside the repo |

## Skills

A skill is one procedure an agent follows end to end: trigger, steps,
verification, and the mistakes it prevents. Skills own the *how*; reference
docs own the *what must hold* and link to the skill instead of inlining
steps.

Each skill is `.agents/skills/<name>/SKILL.md` with YAML frontmatter:

```markdown
---
name: api-contract-change
description: Use when changing any server DTO, handler annotation, or API
  response so the OpenAPI spec, frontend types, and docs regenerate together.
---
```

The `description` states *when to invoke*, not just what it is — agents match
on it. The body is imperative: numbered steps, exact commands, the
verification that proves the step worked, and known failure modes. Keep one
skill per workflow; link contracts rather than restating them.

`AGENTS.md` lists every skill with its one-line trigger. Tool-specific
loaders may be pointed at the same tree via symlink (for example
`.claude/skills -> ../.agents/skills`); `.agents/` stays the real home.

### Extraction rule

When a reference doc carries a step sequence that agents repeatedly follow,
move the procedure into a skill and leave the contract plus a link. This is
how scattered workflow prose in `internal/` docs drains into `.agents/skills/`
over time — extraction happens when a doc is next touched, not as a big-bang
migration.

### Initial set

| Skill | Procedure it owns | Extracted from |
| --- | --- | --- |
| `api-contract-change` | backend DTO/annotation change, `task dto`, regenerated `schema.d.ts` verification | AGENTS.md OpenAPI-first rule |
| `frontend-i18n` | `t("key", "default")`, extract, fill Chinese values | AGENTS.md i18n rule |
| `feature-doc` | `doc.ts` authoring, `{@link}`/import pairing, `doc.md` regeneration | AGENTS.md + docts.md |
| `write-a-test` | pick the layer from the FRONTEND.md taxonomy, create the file where the runner expects it | FRONTEND.md test layers |
| `e2e-environment` | `e2e:up`, seed variants, slice selection, `e2e:down`, log triage | FRONTEND.md E2E sections + web taskfile |
| `select-checks` | map a diff to the narrowest `task` targets before push; CI owns the full matrix | AGENTS.md test-target rule |

Candidates extracted on demand: WASM rebuild, assets/lumen pin reconcile,
desktop and docker release.

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
by an exec plan may keep the decision inside the plan's completed record;
link it instead of duplicating. Mechanical or local edits are exempt.

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
- Existing regression slices (`auth-totp`, `backup-recovery`, ...) show the
  pattern: an incident buys a permanent gate. The postmortem keeps the *why*
  attached to that gate.

## Generated-artifact freshness

Rule: every checked-in generated artifact either has a CI freshness gate or a
named exception with a reason. "Never hand-edit" stays a prose rule only
until a gate exists; gates are the durable form.

A root `verify:generated` task regenerates and asserts a clean tree
(`git diff --exit-code`), wired into CI:

| Artifact | Generator | Gate |
| --- | --- | --- |
| `web/src/lib/http-commons/schema.d.ts`, OpenAPI spec and docs | `task dto` | `verify:generated` |
| `doc.md` siblings of feature `doc.ts` | docts | `verify:generated` |
| Server config schema and examples | `task config:examples` | `verify:generated` |
| `assets.lock.json`, `lumen.lock.json` | reconcile/sync tools | already gated: `assets:check`, `lumen:check` |
| `web/src/wasm/*` bundles | `task wasm:blake3` | **exception**: Rust toolchain is not a CI baseline; PR review owns freshness |

Pre-commit may regenerate-and-stage instead of rejecting, so a forgotten
generator run is fixed at commit time rather than failing CI later.

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
- **Record/replay at the expensive nondeterministic boundary.** The Lumen
  inference boundary (`siglip`, `face`, `ocr`, `bioclip`) is the candidate:
  record real responses once into reviewed fixtures, replay them keyless and
  GPU-less in CI, and keep the full-stack E2E slices for a small set of
  smokes. Recording is explicit, never implicit in CI; every fixture diff is
  reviewed.

## The obligation

The single rule that closes the loop, carried in `AGENTS.md`:

> A non-trivial change updates one memory in the same PR — the owning
> decision record, exec plan, or postmortem — and passes the gates its diff
> touches. Mechanical or local edits are exempt.

## Bootstrap state

The directories, this reference, and the `AGENTS.md` links come first; the
initial skills are extracted from the docs named above; `verify:generated`
lands with the first gate it can enforce. Decision records and postmortems
accumulate as the work happens — an empty `postmortems/` directory is a good
sign, not a gap.
