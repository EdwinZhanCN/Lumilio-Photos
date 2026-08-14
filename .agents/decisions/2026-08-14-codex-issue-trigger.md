# Decision: Trigger Codex with one labeled Issue comment

Status: implemented — an owner-applied `codex` label delegates triage to
Codex Cloud through one authenticated `@codex` comment.

## Problem

The first Issue-agent design recreated orchestration inside the repository:
custom commands, Project-field state, schemas, publishing scripts, and a
manual Cloud handoff. The OpenAI GitHub integration already creates a Codex
Cloud task when the connected user mentions `@codex`, so the repository
machinery added state without owning the actual agent runtime.

## Decision

Issues stay short and use normal GitHub authoring. Applying the `codex` label
is the explicit owner-only opt-in. A single GitHub workflow posts one
idempotent, triage-only `@codex` comment with a user token, so the Connector
sees the same GitHub identity as a manual comment. Human review and follow-up
happen in the linked Codex Cloud task; implementation returns as a Draft PR,
and a human merge closes the Issue through the PR's `Closes #<number>` link.

`CODEX_TRIGGER_TOKEN` is a repository-scoped fine-grained token with only
Issues read/write access. The previous `AGENT_PROJECT_TOKEN` name is accepted
temporarily so the transition does not break before the narrower secret is
stored.

## Alternatives considered

- Keep the custom Issue/Project state machine. Rejected because it duplicated
  Codex Cloud lifecycle state, required bespoke slash commands, and could not
  invoke the subscription-backed Connector directly.
- Let `github-actions[bot]` mention `@codex` with `GITHUB_TOKEN`. Rejected
  because OpenAI does not document Connector activation from bot-authored
  comments; a user-authored comment is the behavior verified in this
  repository.
- Use a custom `Agent` Issue Type. Rejected because GitHub exposes custom
  Issue Types through organization ownership; this personal-account
  repository returns no available types and forbids creating one.
- Trigger every new Issue. Rejected because public reporters must not be able
  to spend the maintainer's Codex allowance or delegate unreviewed prompts.
- Require a special Issue template. Rejected because capture should remain a
  short normal Issue from Raycast; the `codex` label is the delegation control.
