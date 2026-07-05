# Upload

The Upload feature owns the client-side queue, drag-and-drop intake, hashing
pipeline, batch/chunk transport calls, and global upload status UI. It is
surfaced primarily on `/manage`, but the feature boundary is separate:
Manage decides where upload appears, while Upload decides how files move from
browser selection into the repository.

## State

[UploadProvider](./UploadProvider.tsx) wraps the app with [UploadContext](./upload.type.ts). The reducer
stores selected `UploadState.files`, placeholder preview slots, and
drag-over state. Consumers use [useUploadContext](./hooks/useUpload.tsx); calling that hook
outside the provider is an error.

Upload processing state comes from [useUploadProcess](./hooks/useUploadProcess.tsx). It owns the
aggregate progress number, per-file [FileUploadProgress](./hooks/useUploadProcess.tsx), hashing
progress, and the two active flags used by the provider:
`isGeneratingHashCodes` and `isUploading`.

[UnifiedUploadSection](./components/UnifiedUploadSection.tsx) is the primary queue editor. It validates
selected files, adds them to the provider queue, lets the user clear the
queue while idle, starts upload, and exposes the working repository picker.
[NavbarUploadQueue](./components/NavbarUploadQueue.tsx) is only a compact global status surface; it reuses
provider state and links back to `/manage` for detailed control.

## Data

Upload target selection is the settings feature's working repository, read
through `useWorkingRepository`. The upload path requires one concrete
repository id; unlike browse scope, "all repositories" is not a valid target.
If the user has not selected a working repository, the settings hook resolves
primary/first repository fallback before upload transport receives the id.

[useUploadConfig](./hooks/useUploadQueries.ts) reads `/api/v1/assets/batch/config`. The server is
authoritative for chunk size and concurrency; [useUploadProcess](./hooks/useUploadProcess.tsx) uses
fixed fallbacks only while that config is unavailable. Small files are sent
through [useBatchUploadMutation](./hooks/useUploadMutations.ts); large files are sent through
[useChunkedUploadMutation](./hooks/useUploadMutations.ts). Both pass the resolved repository id to the
upload transport layer.

Files are hashed before transport through `useGenerateHashcode`. Hashing is
pipelined with upload: each hashed large file can start chunked upload, and
small files are buffered into smart batches. After transport completes,
asset list/search queries are invalidated so repository-aware galleries can
show the newly indexed assets.

## Composition

```mermaid
flowchart TD
    PROVIDER["UploadProvider"] --> REDUCER["uploadReducer"]
    PROVIDER --> PROCESS["useUploadProcess"]
    UI["UnifiedUploadSection"] --> CTX["useUploadContext"]
    NAV["NavbarUploadQueue"] --> CTX
    UI --> PICKER["useWorkingRepository"]
    UI --> CONFIG["useUploadConfig"]
    PROCESS --> HASH["useGenerateHashcode"]
    PROCESS --> BATCH["useBatchUploadMutation"]
    PROCESS --> CHUNK["useChunkedUploadMutation"]
    BATCH --> TRANSPORT["uploadTransport"]
    CHUNK --> TRANSPORT
```

[FileDropZone](./components/FileDropZone.tsx) contributes drag/drop interaction, but validation and
queue mutation stay in [UnifiedUploadSection](./components/UnifiedUploadSection.tsx). [ProgressIndicator](./components/ProgressIndicator.tsx)
can render aggregate progress, while [NavbarUploadQueue](./components/NavbarUploadQueue.tsx) renders the
durable per-file queue that remains visible across routes.

## Decisions

The upload queue is global because users can leave `/manage` while an upload
is still running. The navigation queue keeps progress inspectable without
duplicating transport logic.

Transport parameters come from the server because the server owns memory,
chunk, and concurrency limits. Client fallbacks are resilience defaults, not
product configuration.

The working repository boundary must stay separate from browse scope. Upload
creates new assets and therefore needs a concrete destination; browse pages
can intentionally aggregate multiple repositories.
