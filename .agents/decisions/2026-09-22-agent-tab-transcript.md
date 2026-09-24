# Decision: Keep the Agent transcript per tab for the server memory window

Status: implemented, 2026-09-22. Owner:
[Lumilio doc.ts](../../web/src/features/lumilio/doc.ts).

## Problem

The Agent chat store lived only in memory. A reload discarded the transcript
while the server still held the thread's model memory for its two-hour idle
TTL, so the user lost context the model still had. Worse, when a confirmation
effect committed but its receipt SSE was lost, a reload also discarded the
pending confirmation identity, leaving the outcome ambiguous until the user
inspected the affected resource. A confirmation without an effect identity
could also leave the run reading as "working" indefinitely.

## Decision

`features/lumilio/state/chatSessionPersistence.ts` stores the settled
transcript, thread identity, and pending confirmation identity in
`sessionStorage` under `lumilio.agent.session` (registered in
`lib/settings/registry.ts`). Streaming chunks are not written; a turn is
persisted when it settles, and a pending confirmation is written as soon as it
exists. A saved transcript older than the server's two-hour conversation TTL is
discarded. The session boundary (`resetLumilioSession`) and "New conversation"
clear it.

On load, unfinished tools read as cancelled and an empty assistant placeholder
is dropped. An undecided confirmation stays actionable because its interrupt
checkpoint is durable on the server. A submitted confirmation is reconciled
through the scoped effect-status endpoint, never by replaying the mutation;
without an effect identity it falls through to the retryable failure state.

A turn that fails before producing any output can be retried with the same
in-memory request scope. Selected asset ids are never persisted.

## Alternatives considered

**Durable server-side chat history** — rejected for this release. The product
position is "conversation memory is temporary; pins and completed actions are
durable", and a history product needs retention, search, and privacy controls.

**`localStorage`** — rejected. It would share one transcript across tabs and
survive browser restarts beyond the server memory window, and two tabs would
race on one thread.

**Persist every streamed chunk** — rejected. It rewrites a growing JSON value
per token for no recovery benefit; an interrupted stream cannot be resumed.

**Replay the resume request after reload** — rejected. The effect receipt is
the authority; replaying could commit twice.
