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
| **Open Existing Repository** | Web; Desktop chooser or bounded Server candidates | An existing direct-child Repository directory containing `.lumiliorepo` | No; it registers the existing Repository and queues fresh observation |

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

**No import policy ever replaces an existing original.** If a new file needs to be written to `inbox/` where a same-named file already exists, the conflict is resolved by renaming the new file — `rename` appends `(1)`, `(2)`, … and `uuid` appends a short UUID — and the existing file stays untouched. Exact byte identity is established only with a stable full BLAKE3 hash and size. Files with the same bytes and owner share one Asset while each physical copy remains an independent Location; a quick fingerprint is only an upload hint and never authorizes removal or deduplication.

## Inbox is a landing area, not a lock

Uploads and cloud imports finish in `inbox/`, but a completed original remains an ordinary user-controlled file. After an upload reports completion, you may move or rename it anywhere else inside the same Repository with Finder, Explorer, or another filesystem tool. Repository observation includes `inbox/`; stable node/content identity preserves the Asset and its product metadata while its Location changes.

Lumilio never reorganizes originals during a scan. Every exact copy may remain as a Location of the same owner/content Asset. Files that are still being written, inaccessible directories, offline volumes, cursor gaps, notification overflow, cancellation, and incomplete coverage make an operation partial; none is evidence that an original was deleted. Only an error-free, caught-up enumeration of the parent directory can close a missing Location, and that closure never removes the file from disk.

`.lumilio/` is the only application-private subtree inside a repository. Do not move files into it or edit its staging and derived files. `.lumiliorepo` is the repository identity marker; every other ordinary directory, including `inbox/`, is part of the user-visible media tree.

The primary repository is the exception: first-run setup creates it in the non-removable default Storage Location. Regular repositories can use any registered, active Storage Location.

## Opening is only for an existing repository

**Open Existing Repository** registers a Lumilio Repository that already exists on disk. The selected directory must contain a valid `.lumiliorepo`; an empty directory or an ordinary folder of photos is not initialized automatically. Before registration, Lumilio records what the Repository's `.lumilio/` private workspace already holds in a recovery receipt and preserves it by class: the Trash stays and is listed as held (it never expires on its own until you decide), Studio edits are attached to their photos again once a scan has proven the content, unfinished uploads and unknown files are kept for export or explicit deletion in the Repository's **Recovery inventory** on the Storage page. It then queues an authoritative initial observation. Original media and `.lumiliorepo` stay in place.

If that repository identity is already registered at another path, Desktop asks you to decide explicitly:

- **Use as moved original** when the repository moved to another disk, mount point, or Windows drive letter. This is a Reconnect: it re-points the registration and never moves files. It is refused while the registered original is still online, and a registered folder that Lumilio cannot read is never treated as gone.
- **Add as separate repository** when this directory is an independent copy that needs a new identity. Lumilio preserves the copied private state the same way, mints a fresh repository UUID, and requires explicit confirmation.

Lumilio does not guess whether a directory moved or was copied based on whether its old path is currently online.

## Offline and moved locations

When an external drive or network volume disconnects, its Storage Location and Repositories become offline. Lumilio preserves their identity, browsing records, and valid Locations, but refuses writes and absence finalization until they reconnect. It does not silently create a replacement directory on another disk.

If the same `.lumilioroot` appears at a new mount path or drive letter, choose **Reconnect Storage Location** from that Storage Location's menu on the Web Storage page (offered on a Desktop host; while the registered folder is still there the entry is disabled with the reason) and approve the local Desktop request. Every child Repository must be where the Location layout says it is, carrying its own identity; then the Location and all its children are re-pointed together, and nothing is moved. A single moved Repository uses **Reconnect** in its own row menu the same way.

Removing an unused external Storage Location only removes its registration. It does not delete its directory, marker, or media. A location still referenced by registered repositories cannot be removed.

Removing a regular repository from Lumilio is also registration-only. The confirmation dialog shows catalog impact and requires the exact repository name; active tasks and the primary repository block removal. The `.lumiliorepo` marker, `.lumilio/` recovery data, and every media file remain on disk so the repository can be opened again later.

## Docker and standalone Server candidates

Docker and standalone Server have one configured default Storage Location. Web can inspect only its direct child directories and classifies each as registered, ready to open, empty and writable, non-empty without a marker, invalid marker, or identity conflict. An identity conflict offers the same explicit **Use as moved original** and **Add as separate repository** decisions as Desktop; Reconnect remains unavailable while the registered original is online. The API accepts a portable directory name, never an arbitrary server path.

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
