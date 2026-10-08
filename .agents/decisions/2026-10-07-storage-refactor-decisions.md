# Decision: Storage refactor product decisions — per-operation consent, explicit primary adoption, preserved private state, no network storage in rc.1, Reconnect not Relocate

Status: accepted, 2026-10-07 (owner). Implementation is in progress under
[the storage repository lifecycle plan](../../docs/exec-plans/active/storage-repository-lifecycle.md);
this record states the target behavior that plan delivers. The plan's
remaining smaller decisions stay inside the plan until it completes.

## Problem

Three rc.1 blockers live in one subsystem: the Desktop risk confirmation on
a removable non-default Location never renders
([#242](https://github.com/EdwinZhanCN/Lumilio-Photos/issues/242)), Desktop
"relocate" cannot actually re-point a Repository, and a leftover primary
Repository found on reinstall reports "existing Repository" with no recovery
action. The audit traced all three to competing classifiers, consent stored
as a reusable flag, recovery actions that are unreachable in the current
phase, and an operation name ("relocate") that promised behavior the code
never had. Fixing them structurally requires five product decisions before
implementation.

## Decision

1. **Risk confirmation is per operation and bound to the assessment.** A
   risky target (removable, external, cloud-backed, and similar) is
   acknowledged for one command, against the assessment fingerprint the
   Server returned (target identity, intent, risk vocabulary, relevant
   mount and marker facts). Changed facts invalidate the acknowledgement;
   capacity fluctuation alone does not. There is no persistent
   per-Location "confirmed" flag. Storage Location authorization grants a
   path scope only and is never reusable risk consent.
2. **A leftover primary Repository gets an explicit choice.** When setup
   finds a compatible primary at the fixed `<default>/primary` target, the
   authenticated setup journey offers **Adopt Primary** (register the
   existing UUID as primary, keep originals, assign the current Host Owner)
   or **start fresh elsewhere** (choose a different default Location; the
   old tree is preserved untouched). Adoption is never implicit, and the
   journey discloses that catalog-only metadata (albums, ratings, People,
   shares) needs a compatible backup.
3. **`.lumilio/` private state is preserved and recovered by class.**
   Adopt, open, and copy never isolate or discard user-bearing private
   data. Trash entries are rebuilt through the existing Trash machinery and
   Studio edit records are reassociated only on a proven content match.
   Inherited Trash is held: it never enters automatic expiry or purge until
   the user restores it, exports it, deletes it, or accepts normal
   retention. Unknown or newer private content is preserved unchanged and
   disclosed.
4. **Network storage is explicitly unsupported in rc.1.** NFS, SMB/CIFS,
   AFP, sshfs, and WebDAV targets (including Windows remote drives) are
   refused by one shared support classifier on every platform. The UI says
   so, with the reason, before the user starts a create, open, or reconnect;
   a confirmation can never enable it. The repository lock, move/identity
   detection, and change watching sit behind swappable interfaces so
   network support can be added later as its own feature
   ([#245](https://github.com/EdwinZhanCN/Lumilio-Photos/issues/245)).
5. **Relocate means reconnect only.** The action re-points a registered
   identity after its tree was moved or remounted externally; it never
   moves data. The UI labels it **Reconnect** and explains its
   prerequisites. Managed movement between volumes is a separate future
   feature.

## Alternatives considered

- **Persist risk consent per Location (today's `storage_risk_confirmed`).**
  Rejected: the flag outlives the facts it confirmed, and #242 showed that
  deriving the confirmation UI from a different target than the one the
  command assesses produces an impossible checkbox.
- **Session-scoped or remembered consent.** Rejected: it reintroduces stale
  consent with an ambiguous lifetime and adds persistence for no user value.
- **Adopt a leftover primary automatically on reinstall.** Rejected: marker
  UUIDs cannot prove which installation wrote them, and silent adoption can
  expose inherited Trash to expiry.
- **Allow only "start fresh" (or only adoption).** Rejected: either choice
  alone strands users who want the other, which is today's dead end.
- **Keep blanket isolation of `.lumilio/` into a recovery directory.**
  Rejected: bytes survive but Trash and Studio edits become unreachable, and
  rebuilt Trash could expire inherited items immediately.
- **Support network storage in rc.1.** Rejected for now: OS file locks are
  unreliable across machines, inode/device identity is unstable, fsnotify
  misses remote changes, and disconnects need the offline path. The common
  deployment (Lumilio on the NAS, photos on local disks) is unaffected.
- **Let users confirm past the network-storage check.** Rejected: a
  confirmation cannot make an unreliable lock reliable.
- **Implement managed Move under the Relocate label.** Rejected: copying,
  verification, cancellation, space planning, and crash recovery need their
  own design and must not be smuggled into the assessment refactor.

## Facts/clarifications 2026-10-07

macOS and Windows ship the Desktop App plus Web App. Linux ships only the
Docker app, using an Ubuntu-based image; there is no native Linux desktop.
The smaller **Existing empty directory / Linux empty mount point versus empty
folder** question is resolved: retain the mounted-volume constraint
permanently. An existing empty mount point (including a bind-mounted host
volume) is allowed; an ordinary existing empty directory is rejected because
it lives in the container's ephemeral writable layer and its data is lost on
container recreation. The reason is `storage/not-a-mounted-volume`, with
instructions to mount a host directory in compose. This clarifies the support
matrix without changing the five approved decisions or rejection outcome.
