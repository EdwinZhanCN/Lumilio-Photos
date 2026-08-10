# Storage Locations and Repositories

::: warning Beta software
Start with test media or a Repository that already has a reliable backup. Do not use Lumilio Photos as the only copy of important media.
:::

Lumilio separates a directory that is **authorized to contain repositories** from a **repository that stores media**:

- A **Storage Location** is an authorized parent directory. Its portable identity is stored in `.lumilioroot`.
- A **repository** is an independent directory containing media and repository configuration. Its identity is stored in `.lumiliorepo`.

One Storage Location can contain multiple repositories. A directory cannot be both a Storage Location and a repository.

```text
Storage Location/
├─ .lumilioroot
├─ primary/
│  ├─ .lumiliorepo
│  └─ .lumilio/
└─ Family Photos/
   ├─ .lumiliorepo
   └─ .lumilio/
```

## Three operations that look similar

| Operation | Where | Directory to select | Initializes a repository? |
| --- | --- | --- | --- |
| **Add Storage Location** | Web request, approved in Desktop Settings → Storage | An existing parent directory, such as `Lumilio/` on an external drive | No; it only registers or creates `.lumilioroot` |
| **Create repository** | Repository creation in the browser | Select a registered, active Storage Location | Yes; it creates a child directory and `.lumiliorepo` |
| **Open Existing Repository** | Web; Desktop chooser or bounded Server candidates | An existing direct-child repository directory containing `.lumiliorepo` | No; it registers the existing repository and queues a fresh scan |

::: tip Create a new repository on an external drive
Start **Add Storage Location** in Web, approve the request under **Desktop Settings → Storage → Requests from Web**, and select the parent directory on the drive locally. Then return to repository creation and select that Storage Location. Do not give an empty directory to **Open Existing Repository**; opening is only for a directory with a valid `.lumiliorepo`.
:::

## Adding a Storage Location does not create a repository

**Add Storage Location** authorizes a directory as a repository container. Desktop creates `.lumilioroot` when the directory does not already have one. An existing marker identifies the same location after a mount path or drive letter changes. Web owns the visible request, but only the local Desktop approval can open the native directory chooser; the path and one-time approval nonce never travel through the shared HTTP API.

After registration, the location appears in the browser repository form. Creation lets you select:

- the Storage Location;
- a mutable repository display name;
- a stable, portable direct-child storage folder.

The destination summary reports whether the location is writable, its available/total capacity, and its registered repository count. Capacity is read from the selected path itself, so a Docker child mount is not mistaken for its parent path. Filesystem type remains available to diagnostics instead of being a primary placement signal.

The primary creation flow applies the Server's deterministic layout and filename-conflict defaults. Cloud authorization and import are separate tasks performed after a destination repository exists.

**No import policy ever replaces an existing original.** If a new file needs to be written to `inbox/` where a same-named file already exists, the conflict is resolved by renaming the new file — `rename` appends `(1)`, `(2)`, … and `uuid` appends a short UUID — and the existing file stays untouched. A same-name, same-content file is recognized as a duplicate by content fingerprint before any naming applies, so it is not written a second time; a different-name, same-content file is still a duplicate.

## Inbox is a landing area, not a lock

Uploads and cloud imports finish in `inbox/`, but a completed original remains an ordinary user-controlled file. After an upload reports completion, you may move or rename it anywhere else inside the same repository with Finder, Explorer, or another filesystem tool. A later repository scan includes `inbox/` and can preserve the existing asset identity and catalog relationships when the move has one unambiguous full-content match.

Lumilio never reorganizes originals during a scan. If several identical new paths could match a missing original, the scan reports an ambiguous result and makes no identity guess. Remove the extra copies or restore a one-to-one layout, then scan again. Files that are still being written and incomplete directory reads make a scan partial; they are not evidence that an original was deleted.

`.lumilio/` is the only application-private subtree inside a repository. Do not move files into it or edit its staging and derived files. `.lumiliorepo` is the repository identity marker; every other ordinary directory, including `inbox/`, is part of the user-visible media tree.

The primary repository is the exception: first-run setup creates it in the non-removable default Storage Location. Regular repositories can use any registered, active Storage Location.

## Opening is only for an existing repository

**Open Existing Repository** registers a Lumilio repository that already exists on disk. The selected directory must contain a valid `.lumiliorepo`; an empty directory or an ordinary folder of photos is not initialized automatically. Before registration, Lumilio moves old `.lumilio/` private state under `.lumilio/recovery/reopened-…`, creates a fresh private workspace, and queues an authoritative initial scan. Original media and `.lumiliorepo` stay in place.

If that repository identity is already registered at another path, Desktop asks you to decide explicitly:

- **Use as moved original** when the repository moved to another disk, mount point, or Windows drive letter. Relocation is refused while the registered original is still online.
- **Add as separate repository** when this directory is an independent copy that needs a new identity. Lumilio isolates copied private state, mints a fresh repository UUID, and requires explicit confirmation.

Lumilio does not guess whether a directory moved or was copied based on whether its old path is currently online.

## Offline and moved locations

When an external drive or network volume disconnects, its Storage Location and repositories become offline. Lumilio preserves their identity and browsing records, but refuses writes until they reconnect. It does not silently create a replacement directory on another disk.

If the same `.lumilioroot` appears at a new mount path or drive letter, choose **Reconnect Storage Location** in Web and approve the local Desktop request. Child repository paths are updated while preserving their relative layout.

Removing an unused external Storage Location only removes its registration. It does not delete its directory, marker, or media. A location still referenced by registered repositories cannot be removed.

Removing a regular repository from Lumilio is also registration-only. The confirmation dialog shows catalog impact and requires the exact repository name; active tasks and the primary repository block removal. The `.lumiliorepo` marker, `.lumilio/` recovery data, and every media file remain on disk so the repository can be opened again later.

## Docker and standalone Server candidates

Docker and standalone Server have one configured default Storage Location. Web can inspect only its direct child directories and classifies each as registered, ready to open, empty and writable, non-empty without a marker, invalid marker, or identity conflict. An identity conflict offers the same explicit **Use as moved original** and **Add as separate repository** decisions as Desktop; relocation remains unavailable while the registered original is online. The API accepts a portable directory name, never an arbitrary server path.

Use Compose long syntax for host directories and disable automatic source creation:

```yaml
services:
  lumilio:
    volumes:
      - type: bind
        source: ./lumilio/media
        target: /data/storage
        bind:
          create_host_path: false
      - type: bind
        source: /mnt/archive
        target: /data/storage/archive
        bind:
          create_host_path: false
```

Create `./lumilio/media`, the app-state directory, and `/mnt/archive` before `docker compose up`. A misspelled host path then fails instead of silently creating an empty directory. When an already-existing empty direct child is used as a Linux repository target, Lumilio verifies it from `/proc/self/mountinfo`; ordinary new direct-child folders created by Lumilio do not need to be mount points.

## Data that does not travel with a Storage Location

A Storage Location is not a complete workspace. These remain private to the machine running Lumilio:

- the SQLite catalog;
- login keys and application credentials;
- cloud sessions and credential state;
- service logs, Lumen models, and database backups.

Repository-owned recoverable work remains under `.lumilio/`, including import staging and non-destructive edit state. Do not edit `.lumilioroot`, `.lumiliorepo`, or `.lumilio/` manually.

For a first test, use the primary repository in the default Storage Location. Once that works, test external storage with a small set of backed-up media using either **Add Storage Location → Create repository** or **Open Existing Repository**.
