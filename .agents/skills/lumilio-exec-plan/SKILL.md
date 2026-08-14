---
name: lumilio-exec-plan
description: Use when starting work large enough to need an execution plan,
  updating an active plan as PRs land, or completing/abandoning one — creation
  criteria, the plan skeleton, and the completion procedure that extracts
  decisions and deletes the plan.
---

# Execution Plans

A plan is a working document for large prospective work, not a decision
archive. Plans live in `site/docs/internal/exec-plans/active/` — one file per
plan, no completed tier. Durable memory lives elsewhere: decisions in
`.agents/decisions/`, escaped bugs in `.agents/postmortems/`, debt in
`exec-plans/tech-debt-tracker.md`.

## When work earns a plan

Write a plan when the work is multi-phase or multi-PR, spans modules, must
freeze contracts before implementation, or coordinates a migration/recovery
that later sessions must be able to resume. A single-PR non-trivial change
records a decision in `.agents/decisions/` instead; do not write a plan for
it.

## Skeleton

Follow the shape the live plans share (see any file in `active/`):

```markdown
# <Title>

Status: active. <Where things stand, with dates for frozen contracts.>

Goal: <observable end state, and what this plan explicitly is not.>

## Non-goals
## <Fixed contracts — the sections the work must not violate>
## Execution phases
### Phase 0 — Lock the failures
### Phase 1 — ...
## Validation boundaries
```

- `Status:` is the first thing a resuming session reads: keep it current as
  PRs land, and date contract freezes.
- When the plan fixes broken behavior, Phase 0 locks the current failures as
  deterministic regression tests before any fix.
- `Validation boundaries` states the observable evidence that means done —
  invariants and required scenarios, not a test-file inventory.
- The plan owns its interim decisions while active (a `## Decisions (frozen)`
  section is fine); they are extracted at completion.

## Maintain

Update `Status:` and the phase list in the same PR that changes the reality
they describe. A plan that no session updates is drift — either the work is
dead (abandon it) or the plan no longer matches the code (fix it before more
work lands).

## Complete

1. Verify the validation boundaries actually pass; run the relevant slices
   per [lumilio-select-checks](../lumilio-select-checks/SKILL.md).
2. Extract each durable decision — with its alternatives and why they lost —
   into `.agents/decisions/YYYY-MM-DD-topic.md`. A decision nobody will
   revisit needs no record.
3. Move surviving debt to `tech-debt-tracker.md`; update the owning reference
   docs (BACKEND.md, FRONTEND.md, feature `doc.ts`) in the same change.
4. Delete the plan file. Git history retains the full record; do not keep a
   trimmed copy anywhere.

## Abandon

If the direction itself was rejected and remains tempting, record a
`Status: rejected` decision with the reason, then delete the plan. If it was
merely overtaken by events, delete the plan outright.
