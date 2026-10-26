# Decision: Storage commands go through one read-only assessment, a role-aware policy, journaled receipts and guarded marker writes; the storage refactor's settled defaults

Status: implemented, 2026-10-26. Delivered by the storage repository lifecycle
plan (P1–P8, PRs #250, #252, #256, #257, #259, #260, #261, #262 and the S8
PR), which this record and its two siblings
([volume semantics](2026-10-26-repository-volume-semantics.md),
[executable recovery registry](2026-10-26-executable-recovery-registry.md))
replace. The product decisions it implements are the owner's
[2026-10-07 record](2026-10-07-storage-refactor-decisions.md).

## Problem

Before the refactor, every storage journey classified its target for itself
(the create wizard, the Docker candidates, the Desktop host action, setup), a
Location-level "risk confirmed" flag outlived the facts it confirmed, handlers
decided which actions a row offered, rename rewrote a marker it had not
re-read under the lease, and an unreadable original was treated like an
absent one. The three rc.1 blockers (#242, the Desktop relocate no-op, the
leftover-primary dead end) were symptoms of those four structural faults.

## Decision

1. **One read-only assessment feeds a pure policy.** `ObserveStorageTarget`
   collects facts (identity, registration, marker compatibility, layout,
   availability, ownership, filesystem support, backing storage, volume
   semantics) through a `StorageObserver` that can only read and stat;
   `ClassifyStorage` and `AssessStorageTarget` turn them into a support
   verdict, a closed risk vocabulary and a consent token. Setup, status,
   candidates and assessments perform no probes, lock files, reconciliation
   or expiry writes; deep materialization checks are bounded command
   preflight. Unknown never means safe: a denied or failed observation is
   `permission-denied` or `observation-unknown`, never absent or offline.
2. **Consent is per operation and bound to the assessment.** The token is an
   HMAC over the target identity, intent, risk set and the marker and mount
   facts; capacity alone does not invalidate it. Every create, open,
   candidate decision, native task and setup adoption carries
   `assessment_token` plus `acknowledged_risks`; a changed target answers
   `storage/assessment-stale`, missing acknowledgements answer
   `storage/confirmation-required` with the current risks, and an
   unsupported target is refused before any consent is possible.
3. **Commands are role-aware, journaled and replayable.** `DeriveRepositoryActions`
   and the Storage Location policy decide, per role and current facts, which
   of verify, rename, detach and reconnect a row offers; the command re-derives
   the same policy on the row it reads under its mutation lease, so menu and
   command cannot disagree. A withheld action is a typed
   `RepositoryActionUnavailableError` with stable blocker codes and legal
   alternatives. Rename, detach, open, copy, adopt and reconnect are
   journaled lifecycle operations keyed by the request's idempotency key: a
   retry replays the same `LifecycleReceipt`, an interrupted one is settled by
   startup recovery, and a conflicting payload under a completed key is
   refused. The Primary Repository and the Default Storage Location never
   offer ordinary rename, copy, detach or reconnect; their recovery is the
   default-configuration handoff.
4. **Markers are refreshed under the lease and written guarded.** A command
   loads the complete marker after it holds the row, changes only what it
   owns (rename changes the raw name), replaces the file atomically, and
   rolls back without ever overwriting an identity that was replaced
   externally in between (R5).
5. **Reconnect requires a positively unavailable original.** A registered
   path that is still readable, or whose state cannot be determined, never
   authorizes a reconnect; an online copy is refused with
   `storage/original-online`, a tree with another identity with
   `storage/identity-mismatch`. Ownership follows the moved tree (the runtime
   lock travels with it), and a Storage Location reconnect revalidates every
   child marker before any row changes.
6. **Private state is preserved by class, with a receipt.** Open, copy and
   adopt record what `.lumilio/` already holds in a format-1 recovery receipt:
   Trash entries are rebuilt and held out of automatic expiry until the user
   decides, Studio records are reattached only on a proven content match,
   staging is moved under the receipt and never resumed, logs and derived
   files stay in place, unknown content is preserved and disclosed.

### Settled defaults

These were interim defaults of the plan; each was exercised by its phase and
is now final.

| Question | Settled answer |
| --- | --- |
| Pre-release compatibility | No catalog or configuration migration; portable marker `"1.0"` is accepted whichever prerelease wrote it |
| Lost catalog metadata | Albums, People, ratings, Event names and share links return only from a compatible backup; adoption and open disclose this and promise nothing portable |
| Primary/default movement | The primary is the fixed `<default>/primary` child; the whole default moves only through configuration Apply; no individual primary reconnect |
| Fresh default during setup | Allowed only before the primary exists, with no child registrations and no pending mutation; the old tree is left untouched |
| Path-selection ownership | Web owns the journey, Desktop supplies selection, approval and Apply, Server owns policy and commands; host paths and nonces never enter HTTP DTOs |
| Corrupt or newer marker | No automated reset or overwrite: diagnose, known-backup instructions, or a fresh destination |
| Primary rename | Prohibited by policy and command; the name is reserved |
| Permanently lost established primary | Identity protected; recovery is a compatible backup or a separate fresh app state |
| Existing empty directory (Docker) | Must be a mounted volume; an ordinary empty folder lives in the ephemeral layer and is refused with `storage/not-a-mounted-volume` |
| Ambiguous Studio match | Proven content hash, size and source or nothing; ambiguity is reviewed or exported, never guessed from filenames |
| Inherited Trash retention | Held until restore, export, delete or explicit acceptance of normal retention; new deletes follow the configured retention |
| Unsupported volume semantics | The affected transition is refused; support grows only with CI filesystem-image evidence |

## Alternatives considered

- **Per-journey classifiers with a shared helper.** Rejected: #242 was the
  direct result of the review and the command assessing different targets;
  only one classifier, enforced by `architecturecheck`, removes the class of
  bug.
- **A reusable confirmation flag on the Storage Location or the session.**
  Rejected by the owner record; it confirms facts that no longer hold.
- **Handler-side action rules.** Rejected: a handler cannot see the lease,
  the journal or the role policy, so its menu drifts from what the command
  accepts; storage now owns every action vocabulary and handlers only
  project.
- **Inferring "offline" from any stat error.** Rejected: a permission
  problem then authorizes a reconnect onto a copy; denial and unknown are
  their own states and never positive absence.
- **Best-effort marker writes without a lease.** Rejected: the rename race
  (R5) overwrote an identity another process had replaced; refresh under the
  lease plus guarded replace closes it.
- **Blanket isolation of `.lumilio/` into a recovery folder.** Rejected by
  the owner record; class-specific preservation keeps Trash and Studio
  reachable.

## Where it lives

`server/internal/storage` (`doc.go` is the map), `server/internal/storage/recovery`,
`server/internal/storage/trash`, `server/app` (setup and reconnect acceptance
tests, `ReviewDefaultStorageMove`), the Web `repositories` feature and the
`@storage-recovery` E2E slice.
