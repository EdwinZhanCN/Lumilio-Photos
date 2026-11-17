# Music Agent uses existing refs, player and confirmed effects

Status: accepted, 2026-09-09.

## Decision

Music tools extend the existing Agent runtime. AuthorizedLibrary performs owner-scoped metadata search and validates every ref consumer. Candidate refs are ordered unique asset sets, bounded to 100 tracks for audition and persistence. Hydration additionally requires the owning thread. Duration budgets exclude unknown/nonpositive durations; artist diversity uses stable round-robin ordering. Search reports bounded results and does not infer acoustic mood.

The Music result card owns temporary selection edits and attaches their exact order through the existing context injection boundary. Playback starts only on a user gesture through the shell's MusicPlayerProvider. The preview queue is local playback state; it is never a saved playlist or a tool-driven autoplay action.

Playlist saves reuse the confirmed effect journal. MusicService implements a narrow caller-transaction writer so entry changes and the committed receipt are atomic. The commit rechecks live source membership and target ownership/revision. Append preserves existing entry identities and duplicate occurrences; skip-existing is explicit. Repeated confirmation returns the original receipt. The confirmation card hydrates the actual effect ref so the user can inspect membership before applying it.

## Alternatives

A separate music agent/runtime would duplicate checkpoints, authorization and confirmation flows. Writing a playlist through ordinary HTTP mutations after tool confirmation would break atomicity with its durable receipt. Reusing photo cover widgets would hide music ordering and audition controls. Storing candidate order as playlist entries would conflate a temporary selection with occurrence identity. These alternatives were rejected.

## Evidence

Server gate and frontend gate passed (429 frontend tests, 6 existing skips), along with architecture checks. Focused regressions cover ordering, metadata filters, scope isolation, duration/artist selection, stale revisions, duplicate append semantics, rejected/deleted selections, runtime reconstruction and replay, and rollback on receipt persistence failure. Real keyless Agent E2E passes search, audition, reorder/exclude, scoped refinement, confirmation, playlist hydration and uninterrupted SPA playback. Original Music playback E2E also passes. OpenAPI, SQLC and feature docs reproduce with unchanged hashes after regeneration.
