# Decision: Adopt a lean agent harness (.agents/ memories and skills)

Status: implemented

## Problem

Recurring procedures were scattered through reference docs and drifted there
(`web/ARCHITECTURE.md` still instructed `make web-test` after the Make-to-Task
migration). Decision rationale lived only in the owner's Obsidian vault, which
agents cannot read — FRONTEND.md cited out-of-repo "ADR-005/006" that no
session could resolve. The `exec-plans/completed/` tier had grown to 24 files
whose job (retain durable decisions) it performed poorly: full plan bodies
were kept as "historical records" nobody was required to read.

## Decision

Three memories plus skills, defined in
[agent-harness.md](../../site/docs/internal/agent-harness.md):

- `.agents/skills/lumilio-<name>/SKILL.md` owns recurring procedures;
  reference docs keep contracts and link to skills. Initial set:
  select-checks, api-contract-change, write-a-test, frontend-i18n,
  e2e-environment, add-task-target, feature-doc, exec-plan.
- `.agents/decisions/` owns project-coupled decision records with mandatory
  alternatives. Strategic/personal ADRs stay in the owner's Obsidian vault;
  any rationale a future agent needs must exist here.
- `.agents/postmortems/` owns escaped-bug records whose guardrails must link
  merged artifacts.
- Exec plans remain working documents in `exec-plans/active/` only.
  Completing a plan extracts durable decisions into `.agents/decisions/` and
  deletes the plan file; the `completed/` tier is removed and git history is
  the archive.

The obligation carried by root `AGENTS.md`: a non-trivial change updates one
memory in the same PR (owning decision record, exec plan, or postmortem);
mechanical or local edits are exempt.

## Alternatives considered

**The full deepseek-harness apparatus** — lifecycle folders
(proposed/implemented/rejected/archived), machine-gated note formats, word
budgets, ~20 documentation verifiers. Rejected as disproportionate: that
machinery serves a large plugin monorepo with many concurrent agents; here it
would cost more upkeep than the drift it prevents. Gates can be added later
per artifact when a class of drift recurs.

**Keep the `completed/` plan tier** — rejected: it duplicated the decisions
directory's job while burying decisions inside full plan bodies, and its
"historical record, not required reading" status meant it was write-only.
Git history preserves everything the tier did.

**Record all decisions in Obsidian only** — rejected: agents cannot read the
vault, so settled decisions get re-litigated and prose accumulates dead
references (the ADR-005/006 citations were the live example).
