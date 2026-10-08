# Decision: Storage Location does not gate Repository I/O

Status: implemented

## Problem

Storage Location `status` and marker health were treated as a parent gate for
child Repository I/O, enqueue, rename, detach, and reconnect. A missing,
invalid, or replaced Location marker — or an offline Location projection —
could deny a healthy Repository whose own path and `.lumiliorepo` marker were
valid. Capacity was also attributed to the Location registration, so siblings
on independent mounts failed together and bind mounts could be double-counted.

## Decision

A Storage Location is authorization and registration only: a host-authorized
parent identity marked by `.lumilioroot`. It does not own health, capacity, or
I/O admission. A Repository is admitted from its own rooted marker, path, and
operation policy.

Catalog browse uses owner authorization only. Opening an original requires the
Repository marker, a readable path, and a lifecycle lease; Location status,
capacity, and manual write pause are ignored. Upload and cloud materialization
recheck write ownership, pause, and space at I/O. Verification needs a readable
identity and lease; low space or read-only does not forbid observing originals.
Rename takes only the Repository lease. Reconnect and copy require explicit
intent at an authorized destination; the old path need not be readable and
originals are never moved. Detach is a catalog-only unregistration with a fresh
impact preview; it does not require an online marker or writable disk, and it
never deletes user media.

Capacity is grouped by a proven backing-storage key sampled at the Repository
path, not by Location ID, mount spelling, or matching byte counts. Unknown
grouping stays ungrouped. Linux bind-mount grouping is locked by
`TestInspectStoragePathProvesSharedCapacityGroupAcrossBindMount`, which
self-skips without `CAP_SYS_ADMIN`. Docker Desktop virtiofs/osxfs topologies
remain unexecuted native debt.

Desktop tray shortcuts open a Location directory when the registered path is
readable; catalog Location status does not disable Finder.

The observation engine in
[Observe repositories through resumable facts and content Locations](2026-08-22-repository-observation-engine.md)
remains the verification mechanism. Presentation of reserved product names in
[Present storage entities from semantic identity](2026-08-17-storage-entity-presentation.md)
remains; overlaying Location health onto a Repository row does not.

## Alternatives considered

**Keep Location health as a parent I/O gate, with a compatibility overlay in
the Web.** Rejected because a healthy child on a valid path must keep serving
media when the parent marker is wrong, and a display overlay recreates the
same false unavailability.

**Attribute capacity to the Storage Location and subtract Repository media
size from free space.** Rejected because siblings on independent mounts are
not one pool, bind mounts would double-count, and media size is already
consumed on the backing filesystem.

**Move files automatically when reconnecting a copied marker.** Rejected
because originals must stay put; copy is explicit `add_separate`, and a copied
marker is not proof of physical continuity.
