# Decision: A Repository requires writable storage; read-only is a fault, not a mode

Status: implemented — owner decision 2026-10-28; this record makes current behavior explicit.

## Problem

Other photo servers support read-only originals. Examples are Immich's
external libraries and PhotoPrism's originals folder, and users often mount
an archive drive read-only on purpose. Lumilio had never written down whether
a Repository may be read-only. The question came up while weighing portable
per-asset sidecars: a sidecar that cannot be written needs either a fallback
location or a pending state. In Lumilio, a Repository becomes read-only after
creation in cases such as these:

- an NTFS drive mounted on macOS;
- ext4 remounted read-only after I/O errors (`errors=remount-ro`);
- a degraded NAS volume;
- a `:ro` bind mount;
- a changed share permission or container UID.

## Decision

A Repository is a directory that Lumilio manages, and it is always writable.
Initializing a Repository probes writability first
(`prepareRepositoryInitializationTarget`), and so does registering an
existing one (`validateRepository`). Each then writes the `.lumiliorepo`
marker. Lumilio
keeps its derived and private state inside the Repository, under `.lumilio/`:

- published artifacts such as thumbnails and transcodes;
- face crops;
- staging;
- Trash;
- logs;
- Studio sidecars.

The Repository's private state moves together with its originals.

A Repository that later turns out to be read-only is in a fault state. It is
not a supported mode, and Lumilio has no persistent read-only state or
read-only UI mode for it. Writability is a live fact, probed at the I/O
boundary:

- Upload and cloud materialization are refused with `ErrRepositoryReadOnly`
  (`UploadAdmission`).
- Writes to derived and private state fail as ordinary I/O errors in the
  work that owns them.
- Browsing and opening originals still work, because `OpenOriginalAdmission`
  checks only reachability.

Any future per-Repository write follows the same rule. A portable sidecar is
one example. A failed write is retried once the storage is writable again.
Lumilio does not redirect such a write to another location.

## Alternatives considered

- **Support read-only Repositories by moving `.lumilio/` into an app-owned
  mirror keyed by Repository ID.** This is the PhotoPrism model. Rejected:
  - The Repository's private state would no longer travel with its
    originals. Trash and Studio would no longer be reachable after a
    reconnect, unlike the class-specific preservation in the
    [storage command boundary](2026-10-26-storage-command-boundary.md).
  - Reconnect and rekey would have to move a second tree in step with the
    first.
  - Every thumbnail and transcode would land on app storage. On Desktop that
    is often the nearly full system disk.
- **A read-only "archive" mode that indexes originals but produces no derived
  files.** Rejected because it is a partial product: no thumbnails, no ML
  results, and no Trash. It also adds a third Repository behavior to every
  admission and UI path.
- **Fall back to a mirror only while a Repository is read-only.** This is
  similar to digiKam writing sidecars only for read-only files. Rejected:
  state written while the Repository was read-only would later need
  reconciliation, and the fault is expected to be temporary or a sign of a
  failing disk, not a place to keep data.
