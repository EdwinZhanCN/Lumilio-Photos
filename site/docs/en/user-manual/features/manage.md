# Add Media to a Repository

<DocPath :items="['Sidebar', 'Manage']" />

The **Manage** page has two areas: an upload zone at the top and Repository cards below. The upload zone ingests files you select in the browser into the current Repository; the cards are used to observe files that are already on the server, create Repositories, and run maintenance operations.

## Pick the right import method first

There is one deciding question: **can the machine running Lumilio read the files directly?**

| Where the files are now | Method | Where they end up | Best for |
| --- | --- | --- | --- |
| Current computer, phone, or tablet, selectable in the browser | **Upload** | `inbox/`, archived by the repository policy | Small batches, importing across devices |
| A location authorized in Desktop, or a Repository directory mounted in Server | **Scan** | stays at its current path inside the Repository | Large batches, when you want to manage the folders yourself |
| A connected cloud-service credential | **Cloud import** | `inbox/`, archived by the repository policy | Importing from the cloud; iCloud is still experimental |

Selecting files in the browser only sends their contents to Lumilio; it does **not** authorize the server to read arbitrary directories on the browser device. For Storage Locations, repositories, and Desktop/Server differences, read [Storage Locations and Repositories](../introduction/repositories) first.

## Before you start: confirm the upload target

1. Open the **Manage** page.
2. Choose the repository in the **Upload target** dropdown above the upload zone.
3. Confirm the target again before clicking **Upload**; the batch goes to the repository selected here.

If the dropdown is empty, or a repository shows “offline” or “needs attention”, do not import yet. Desktop users check the Control Panel Storage Locations; Server users check Compose mounts, directory permissions, and disk state. An offline repository refuses writes; it does not quietly create a same-named directory elsewhere.

## Method 1: upload files from the browser

### Steps

1. Click **Supported formats** next to the page title to confirm the extensions are supported.
2. Drag files onto the upload zone or click **Add files** to select them. Images (including camera RAW), videos, and audio are supported.
3. Review the file list and the **upload target**, then click **Upload (count)**.
4. Keep the page open until the queue items become **Completed** or **Failed**. Large files automatically use resumable chunked upload; no manual splitting is needed.

The browser filters by extension first; the server validates again. Unsupported files do not enter the upload queue — remove or convert them and retry.

### Reading upload results

- **Completed**: the file passed server validation and finished ingestion.
- **Processing**: the original file was received; metadata, thumbnails, video/audio web versions, and optional analysis are still being generated in the background.
- **Failed**: ingestion did not finish. The failed file stays in the upload queue; fix the network, permission, or disk issue and click upload again.

After a successful upload, the original file first goes through the Repository's recoverable staging journal, then into `inbox/` according to the Repository policy. Repository creation uses the Server's safe defaults instead of asking for layout details in the primary task. After the upload is complete, `inbox/` is an ordinary user-visible landing area: you may move or rename its originals inside the same Repository, then rescan. Only `.lumilio/` is application-private. If several files owned by the same user have exactly the same bytes, Lumilio uses one Asset identity and keeps every physical file as a separate Location.

::: tip Validate with a small batch first
When importing from a new camera, a phone export folder, or an external drive, select a few files first. Confirm dates, orientation, exact-copy behavior, and thumbnails before submitting the full set.
:::

## Method 2: scan files already on the host

Scanning does not copy files and does not make the browser choose server paths; it starts a durable observation operation for files that already exist in the target Repository. The request settles after that operation is recorded, while discovery, hashing, and derived processing continue in the background.

### Preparation

- Put the files anywhere in the Repository's user-visible media tree, including `inbox/`, but never under `.lumilio/`.
- Desktop: authorize the Storage Location in the Desktop Control Panel first.
- Server: complete the bind mount per [repository mounting and creation](../introduction/repositories); the in-container path must be readable.

::: danger Do not put files into the private directory
`.lumilio/` holds staging files, derived resources, and other application-private state. Repository observation excludes it, and manual changes can make the catalog and files disagree. `inbox/` is not private and is included in scans.
:::

### Running a scan

1. Put media into the repository and wait until copying finishes.
2. Open the **⋯** menu on the repository card and choose **Rescan**.
3. To scan every repository, click **Scan all repositories** above the cards.
4. Follow the queued, crawling, catching-up, and finalizing phases on the Repository card. Check the [Server Monitor](./monitor) page for partial coverage, errors, and remaining background tasks.

The Repository Observation Engine reads bounded directory pages and registers only supported media extensions. A healthy Windows USN/directory-change, macOS FSEvents, or Linux inotify cursor makes unchanged incremental work independent of the Repository's total size; periodic full verification still repairs missed hints.

A proven directory move changes one directory relationship without rewriting or rehashing every descendant. A change notification never proves deletion by itself. Lumilio closes a missing Location only after an error-free parent enumeration has caught up through a fixed change boundary. Offline storage, access errors, cancellation, cursor gaps, notification overflow, and partial coverage preserve every previously valid Location. This reconciliation never deletes an original from disk.

## Method 3: cloud import

Cloud import needs a connected cloud credential. First complete the login and any required verification on the **Cloud import** tab of [Settings](./settings). Repository creation and cloud authorization are separate tasks: create or open the destination repository first, then choose **Import from cloud** in its card menu and select the connected credential.

Cloud-imported files enter `inbox/` like uploads and follow the repository's storage policy. The card shows the last import status, imported count, and failed count; while the task is “queued” or “running”, the trigger button is disabled.

::: warning iCloud is still experimental
iCloud import depends on Apple's unofficial network service behavior, may require extra verification, and can stop working at any time. Use it with care.
:::

## Creating a repository (administrator)

The **Create repository** button in the card area is for administrators. Its primary path asks for:

1. **Storage Location**: Desktop picks from locations registered in the Control Panel; Server uses the mounted Storage Location from Compose.
2. **Repository name**: the user-facing display name. It is independent of the folder name.
3. **Storage folder**: one portable direct-child directory name. Only letters, digits, full-width text, half-width spaces, `-`, and `_` are allowed, 1–80 characters, no leading/trailing spaces; case is preserved.

Creation validates the name, direct-child topology, directory state, and write permissions and fails fast; an incomplete repository is never registered. It applies the Server's safe layout and filename-conflict defaults. If the target already contains a valid `.lumiliorepo`, creation stops and directs you to the explicit **Open Existing Repository** task; it never attaches that identity implicitly. An ordinary non-empty unmarked directory cannot be initialized as a new repository.

Desktop can also start **Add Storage Location** or **Open Existing Repository** from Web. The request appears under **Desktop Settings → Storage → Requests from Web**; a person at that computer must review it and choose the directory locally. Web never submits an arbitrary Desktop path. Standalone and Docker Server instead list classified direct-child candidates from the configured default Storage Location.

For Docker, create a new folder by entering a directory name, or mount an existing empty host directory at that exact direct-child path before creation. On Linux, an existing empty target must be verified as an actual mount point. Follow the long-syntax Compose example in [Storage Locations and Repositories](../introduction/repositories).

## Maintenance actions on a repository card

Open the **⋯** menu on a card for the current repository's actions:

- **Rescan**: request observation of this Repository's user-visible media tree, including `inbox/`; use after copying, moving, or renaming a batch of files.
- **Detect stacks**: run automatic stack detection; review results in [Browse, filter & batch](./assets).
- **Scan duplicates**: run duplicate analysis; results are handled in [Duplicates, likes & trash](./utilities).
- **Rebuild location clusters**: recompute media location clusters; never moves originals.
- **Import from cloud**: only for repositories bound to a cloud credential.
- **Remove from Lumilio**: unregisters a non-primary, idle repository after showing its impact. `.lumiliorepo`, `.lumilio/`, and every media file remain on disk; this is not physical deletion.

The counts and “offline / needs attention” marks on the cards come from the server's current state. When a Storage Location is unavailable, write and maintenance buttons are disabled; restore the mount or permissions first, then retry.

## What to check after importing

Ingestion and derived processing are two stages: the original file may already be in the repository while thumbnails, web versions, people, or semantic results are still queued. When in doubt:

- Queue still running and the completed count is growing: wait; do not rescan or re-upload;
- Exact copies resolve to one owner/content Asset with separate physical Locations; confirm each intended file remains on disk;
- Failures keep growing or nothing progresses: confirm the disk is online, writable, and has space, then check task errors on the [Server Monitor](./monitor) page;
- Scan found 0: confirm files are not under `.lumilio/`, the extension is supported, and the files really are under the current repository's mounted path.
