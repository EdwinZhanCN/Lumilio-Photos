# Add Media to a Repository

<DocPath :items="['Sidebar', 'Manage']" />

The **Manage** page has two areas: an upload zone at the top and repository cards below. The upload zone ingests files you select in the browser into the current repository; the cards are used to scan files that are already on the server, create repositories, and run maintenance operations.

## Pick the right import method first

There is one deciding question: **can the machine running Lumilio read the files directly?**

| Where the files are now | Method | Where they end up | Best for |
| --- | --- | --- | --- |
| Current computer, phone, or tablet, selectable in the browser | **Upload** | `inbox/`, archived by the repository policy | Small batches, importing across devices |
| A location authorized in Desktop, or a repository directory mounted in Server | **Scan** | stays at its current free-zone path | Large batches, when you want to manage the folders yourself |
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
4. Keep the page open until the queue items become **Completed**, **Duplicate**, or **Failed**. Large files automatically use resumable chunked upload; no manual splitting is needed.

The browser filters by extension first; the server validates again. Unsupported files do not enter the upload queue — remove or convert them and retry.

### Reading upload results

- **Completed**: the file passed server validation and finished ingestion.
- **Processing**: the original file was received; metadata, thumbnails, video/audio web versions, and optional analysis are still being generated in the background.
- **Duplicate**: the content fingerprint matches existing media in the target repository; Lumilio skipped the second copy and does not use extra storage.
- **Failed**: ingestion did not finish. The failed file stays in the upload queue; fix the network, permission, or disk issue and click upload again.

After a successful upload, the original file first goes through the repository's staging pipeline, then into `inbox/` according to the policy chosen when the repository was created. Never edit `inbox/` manually in a file manager; the storage details page explains policies and the directory layout.

::: tip Validate with a small batch first
When importing from a new camera, a phone export folder, or an external drive, select a few files first. Confirm dates, orientation, duplicate handling, and thumbnails before submitting the full set.
:::

## Method 2: scan files already on the host

Scanning does not copy files and does not make the browser choose server paths; it registers files that already exist in the target repository.

### Preparation

- Put the files in the repository's **free zone**: any directory under the repository root except `inbox/` and `.lumilio/`.
- Desktop: authorize the Storage Location in the Desktop Control Panel first.
- Server: complete the bind mount per [repository mounting and creation](../introduction/repositories); the in-container path must be readable.

::: danger Do not put files into protected directories
`inbox/` is managed by the upload and cloud-import pipeline, and `.lumilio/` holds thumbnails, staging files, logs, and other system data. The scanner skips both; manual changes can make the database and the files disagree.
:::

### Running a scan

1. Put media into the free zone and wait until copying finishes.
2. Open the **⋯** menu on the repository card and choose **Rescan**.
3. To scan every repository, click **Scan all repositories** above the cards.
4. Wait for the operation to finish, then check the repository or the [Server Monitor](./monitor) page for new media and background tasks.

The scanner only registers supported media extensions. For already-registered files in the free zone, it tries to recognize “the same file moved to a new path” and updates the record instead of creating a second media record; deleting a free-zone file marks the record deleted.

## Method 3: cloud import

Cloud import needs a connected cloud credential. First complete the login and any required verification on the **Cloud import** tab of [Settings](./settings), then choose by source:

- **Import when creating a repository**: click **Create repository**, pick the cloud source and a connected credential. The new repository queues an import automatically.
- **Import again into an existing cloud repository**: choose **Import from cloud** in the card's **⋯** menu. This only appears for repositories bound to a cloud credential.

Cloud-imported files enter `inbox/` like uploads and follow the repository's storage policy. The card shows the last import status, imported count, and failed count; while the task is “queued” or “running”, the trigger button is disabled.

::: warning iCloud is still experimental
iCloud import depends on Apple's unofficial network service behavior, may require extra verification, and can stop working at any time. Use it with care.
:::

## Creating a repository (administrator)

The **Add repository** (create) button in the card area is for administrators. Creation decides the following once:

1. **Storage Location**: Desktop picks from locations registered in the Control Panel; Server uses the mounted Storage Location from Compose.
2. **Repository name**: for regular repositories the name is also the directory name. Only letters, digits, full-width text, half-width spaces, `-`, and `_` are allowed, 1–80 characters, no leading/trailing spaces; case is preserved.
3. **Source**: local files or a cloud credential.
4. **File layout**:
   - `date`: archive into `inbox/YYYY/MM/` by ingestion time;
   - `flat`: all ingested files in the `inbox/` root;
   - `cas`: content-hash-sharded storage.
5. **Filename conflict handling**: `rename` (default — safe rename on conflict) or `uuid` (append a short UUID). **No policy replaces an existing original.**

Creation validates the name, directory state, and write permissions and fails fast; an incomplete repository is never registered. If the target directory already contains a valid `.lumiliorepo`, Lumilio registers it as an existing repository; an ordinary non-empty directory cannot be initialized as a new one. After creation, the current version cannot change the name, layout, or conflict policy, nor delete the repository from Lumilio. For Server, follow the “mount first, then create” order and exact directory-name correspondence in [repository mounting and creation](../introduction/repositories).

## Maintenance actions on a repository card

Open the **⋯** menu on a card for the current repository's actions:

- **Rescan**: scan only this repository's free zone; use after copying a batch of files.
- **Detect stacks**: run automatic stack detection; review results in [Browse, filter & batch](./assets).
- **Scan duplicates**: run duplicate analysis; results are handled in [Duplicates, likes & trash](./utilities).
- **Rebuild location clusters**: recompute media location clusters; never moves originals.
- **Import from cloud**: only for repositories bound to a cloud credential.

The counts and “offline / needs attention” marks on the cards come from the server's current state. When a Storage Location is unavailable, write and maintenance buttons are disabled; restore the mount or permissions first, then retry.

## What to check after importing

Ingestion and derived processing are two stages: the original file may already be in the repository while thumbnails, web versions, people, or semantic results are still queued. When in doubt:

- Queue still running and the completed count is growing: wait; do not rescan or re-upload;
- Media shows as duplicate: confirm the target repository; usually no second import is needed;
- Failures keep growing or nothing progresses: confirm the disk is online, writable, and has space, then check task errors on the [Server Monitor](./monitor) page;
- Scan found 0: confirm files are not in `inbox/`/`.lumilio/`, the extension is supported, and the files really are under the current repository's mounted path.
