# Agent Runtime Security And Cancellation

Status: **completed and verified** (2026-07-24).

Goal: close the Agent data-plane authorization gap, make thread recovery and
effects durable, provide real server-side cancellation, and version every
replayable plan before adding new tools or memory.

## Decisions

- Bind every Agent data access path to an authenticated user through
  `AuthorizedLibrary`; a ref is never proof of current membership.
- Scope threads, runs, checkpoints, refs, pins, and pending effects by user and
  return `404` for cross-user lookups.
- Persist the original mode, typed context bindings, policy version, pending
  effect, and required refs. Resume restores this state and accepts no client
  override.
- Treat Stop as terminal cancellation and confirmation as a resumable
  interrupt. Cancellation is identified by the exact
  `(user_id, thread_id, run_id)` tuple, so an old run cannot cancel a new one.
- Require confirmation for every mutation. Reauthorize immediately before a
  single transactional commit, use an idempotency key, and treat the commit
  boundary as authoritative in cancellation races.
- Keep system instructions static. Context, mentions, and ref ledgers are
  schema-bound untrusted data, and provider reasoning is never emitted over the
  product SSE contract.
- Replay only the current typed Plan schema. Legacy live pins become explicit
  frozen snapshots; semantic cutoffs are selected from a versioned embedding
  profile.
- Keep refs durable while active and memory-bounded with per-user/global hot
  budgets and LRU spill to PostgreSQL.

## Implementation

1. Add forward-only runtime tables for scoped threads, runs, refs, and pending
   effects, plus explicit checkpoint deletion.
2. Route producers, people/OCR/semantic lookup, injection, hydration, live pin
   replay, and mutations through the user-bound library and ref assertions.
3. Add exact run registration, recursive Eino cancellation, disconnect
   cleanup, run-status SSE events, and the authenticated cancel endpoint.
4. Centralize mutation policy, confirmation persistence, reauthorization,
   transactionality, idempotency, and receipts in the effect runtime.
5. Version Plan payloads and embedding profiles; add bounded persistent ref
   storage and explicit live-pin fallback metadata.
6. Add the frontend run state and Stop workflow, preserving partial output and
   marking unfinished tools as cancelled.

## Completion

- Chat rejects missing or unknown modes and Resume restores the owned thread's
  original runtime state.
- Cross-user assets cannot enter refs, searches, live replay, or effects.
- Stop cancels the exact run, disconnects do not leave orphan execution, and
  an already committed effect reports its authoritative receipt.
- Incomplete cancelled turns are not replayed; ordinary SSE never exposes
  model reasoning.
- Legacy plans fail closed to frozen snapshots and hot ref memory is bounded.

## Verification

- `make server-test`
- `make web-test`
- `make dto`
- `cd web && vp build`
- `git diff --check`
