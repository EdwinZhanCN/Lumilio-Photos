# Decision: Each Repository is keyed under the name rule of the volume that holds it, measured on the tree

Status: implemented, 2026-10-26 (storage plan P6b, PR #261). Companion of
[the storage command boundary](2026-10-26-storage-command-boundary.md).

## Problem

The scan index keyed paths under one host-wide rule (the Linux default:
case-sensitive, no normalization). A Repository on an exFAT or FAT32 drive,
or on a case-insensitive APFS volume, therefore reported files as missing or
duplicated whenever the user changed the case of a name, and a reconnect
onto a disk with different rules could silently merge two distinct files
into one key.

## Decision

- `repositories.path_case` and `repositories.path_normalization` hold the
  rule measured on the tree when the Repository is created, opened, copied,
  adopted or reconnected; there is no default. `pathsemantics.Probe` measures
  behaviourally (a hidden temporary file looked up under a case-changed and
  a normalization-changed spelling, `os.SameFile` deciding) and
  `pathsemantics.Qualify` requires the root and `.lumilio/` to agree; a tree
  whose rule cannot be qualified, or is mixed, is neither registered nor
  reconnected (`storage/volume-unqualified`).
- The scanner, the known-content binder, Trash commits and the recovery
  service all key under the row's rule (`scan.SemanticsOf`,
  `storage.RepositoryVolumeRule`); nothing asks the host.
- A reconnect whose destination follows another rule first proves every
  live key distinct in bounded pages (`PathKeyCollisionError`,
  `storage/name-collision`, with a bounded list of pairs), then cuts the
  path and the rule over together into maintenance, rebuilds the keys in
  revision-fenced batches, and releases maintenance; journal recovery
  finishes an interrupted rebuild. Entry ids and Asset bindings survive.
- The supported rules are the ones CI qualifies on real images: Linux loop
  mounts of FAT32, exFAT, ext4 and case-folding ext4; macOS `hdiutil` APFS,
  case-sensitive APFS, HFS+ and ExFAT (`task server:test:volumes`).

## Alternatives considered

- **Keep one host default and document the limitation.** Rejected: the
  default was wrong for the most common external media on a Mac and
  produced false "missing" rows, which the asset lifecycle treats as data
  loss signals.
- **Decide the rule from the filesystem name.** Rejected: ext4 can fold case
  per directory, APFS comes in both flavours, and a container sees the
  host's rule through a bind mount; measuring the tree is the only answer
  that is true for that tree.
- **Refuse every cross-rule reconnect.** Rejected: moving a Repository from
  an internal disk to an exFAT drive is a supported journey; the collision
  preflight makes it safe instead of impossible.
- **Rekey in one transaction.** Rejected: a large index would hold the
  writer for minutes; bounded revision-fenced batches under maintenance keep
  the catalog responsive and recoverable.

## Where it lives

`server/internal/storage/pathsemantics`, `server/internal/storage`
(`repo_reconnect.go`, `RepositoryVolumeRule`), `server/internal/storage/scan`,
the `server volume semantics` CI jobs in `.github/workflows/ci.yml`.
