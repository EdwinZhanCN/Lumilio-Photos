# Decision: Workflows call module Task targets; ci:* is orchestration only

Status: implemented

## Problem

The root Taskfile grew a 1:1 `ci:*` wrapper for almost every module target
(`ci:web:test` → `web:test`, `ci:web:e2e:browser` → `web:test:browser`,
`ci:server` → `server:test:ci`, …). Agents and humans had two names for the
same command, CONTRIBUTING listed both, and adding a slice meant a Taskfile
wrapper plus a workflow line. The wrappers did not encode extra sequencing;
they were a naming indirection.

## Decision

Workflows invoke module targets directly (`task web:test`,
`task server:test:ci`, `task web:e2e:up`, `task web:playwright:install`).
A root `ci:*` name exists only when it orchestrates more than one module
target. The remaining set is `ci:architecture`, `ci:site`,
`ci:desktop:panel`, and `ci:desktop:native`.

`task verify:generated` is a root target (cross-module generators) and is
wired as its own always-on CI job, not hidden behind a 1:1 wrapper.

Placement and naming stay in
[lumilio-add-task-target](../skills/lumilio-add-task-target/SKILL.md).

## Alternatives considered

**Keep `ci:*` as a stable CI façade** — rejected: the façade duplicated every
module name, so a rename still touched two files, and local reproduction
instructions had to teach both vocabularies. Stability is the module target
itself.

**A gate runner with modes (deepseek-harness `run-gates.ts`)** — rejected:
Task already sequences, and the repo's path-filtered workflows already select
which gates run. A second scheduler would be another source of truth.
