# Decision: Integrate ordinary pull requests through dev

Status: implemented — repository instructions and the Codex Issue trigger
explicitly route ordinary work to `dev`.

## Problem

GitHub identifies `main` as the repository's default branch, while ongoing
integration work lives on `dev`. Agents that infer a PR target from GitHub's
default can bypass the integration branch and send feature or harness changes
directly to the stable branch.

## Decision

Ordinary feature, fix, refactor, documentation, and agent-harness work starts
from the latest `dev`, and every Draft or implementation PR targets `dev`.
`main` is the stable/release branch and normally receives changes through an
intentional `dev` → `main` promotion PR. An Issue or a human may explicitly
name another target for an exceptional change; absent that instruction, agents
must not infer a target from GitHub's default-branch setting.

The standing order lives in [AGENTS.md](../../AGENTS.md). The labeled-Issue
workflow repeats the routing constraint in its authenticated `@codex` prompt
so Cloud tasks receive it before planning or implementation.

## Alternatives considered

- Target `main` because GitHub marks it as the default. Rejected because
  repository-hosting metadata does not express the project's integration and
  release boundary.
- Infer the base from recent PR history or whichever long-lived branch is
  ahead. Rejected because both signals are transient and previously produced
  inconsistent targets.
- Require a human to state the base on every Issue. Rejected because `dev` is
  a stable repository-wide default; humans only need to state exceptions.
