# Upload Workflow — Business Flow Diagrams

## System Overview

```mermaid
graph TB
    subgraph Client["Frontend (Browser)"]
        UP_COMP[Upload Component]
        HASH_W[Hash Worker<br/>BLAKE3 in Web Worker]
        CHUNK[Chunk Splitter]
    end

    subgraph Server["Backend (Go + Gin)"]
        HANDLER[AssetHandler]
        SM[SessionManager<br/>chunk tracking]
        CM[ChunkMerger]
        STAGING[StagingManager]
        INGEST_Q[River Queue<br/>ingest_asset]
    end

    subgraph Pipeline["Processing Pipeline (River Workers)"]
        INGEST_W[IngestAssetWorker]
        META_W[MetadataWorker<br/>EXIF / ffprobe]
        THUMB_W[ThumbnailWorker<br/>multi-size]
        TRANS_W[TranscodeWorker<br/>video/audio]
        CLIP_W[CLIP Worker]
        BIOCLIP_W[BioCLIP Worker]
        OCR_W[OCR Worker]
        CAPTION_W[Caption Worker]
        FACE_W[Face Worker]
    end

    subgraph Storage["Repository Storage"]
        STAGE_DIR[".lumilio/staging/incoming/"]
        INBOX["inbox/<br/>date / flat / CAS structure"]
        THUMBS[".lumilio/assets/thumbnails/"]
        VIDEOS[".lumilio/assets/videos/web/"]
        AUDIOS[".lumilio/assets/audios/web/"]
    end

    subgraph DB["PostgreSQL"]
        ASSETS[assets]
        THUMBNAILS[thumbnails]
        TAGS[tags / asset_tags]
        RIVER_JOBS[river_job]
    end

    Client -->|multipart/form-data| HANDLER
    HANDLER --> STAGING
    STAGING --> STAGE_DIR
    HANDLER --> INGEST_Q
    INGEST_Q --> INGEST_W
    INGEST_W --> INBOX
    INGEST_W -->|fan-out| META_W & THUMB_W & TRANS_W
    THUMB_W -->|Photo only| CLIP_W & BIOCLIP_W & OCR_W & CAPTION_W & FACE_W
    INGEST_W --> DB
    META_W --> DB
    THUMB_W --> THUMBS
    TRANS_W --> VIDEOS & AUDIOS
```

---

## 1. Single File Upload

```mermaid
sequenceDiagram
    participant U as Browser
    participant F as Frontend
    participant B as Backend (Handler)
    participant S as StagingManager
    participant Q as River Queue
    participant DB as PostgreSQL

    U->>F: Select file
    F->>F: Calculate BLAKE3 hash (Web Worker)
    F->>B: POST /api/v1/assets<br/>multipart/form-data + X-Content-Hash header
    
    Note over B: Upload Limiter (max 32 concurrent)

    B->>B: ParseMultipartForm (32MB limit)
    B->>B: ValidateFile (extension + MIME → asset type)
    B->>B: Resolve repository (explicit or default)
    B->>S: CreateStagingFile(repoPath, filename)
    S-->>B: StagingFile{Path: .lumilio/staging/incoming/uuid_filename}
    B->>B: io.Copy(file → staging path)

    alt Client provided X-Content-Hash
        B->>B: Trust client hash
    else No hash header
        B->>B: CalculateFileHash(BLAKE3, quickMode for large files)
    end

    B->>B: Build AssetPayload{hash, stagedPath, userID, contentType, fileName, repoID}
    B->>Q: Insert(IngestAssetArgs) → queue: "ingest_asset"
    Q-->>B: Job ID
    B-->>F: {task_id, status:"processing", fileName, size, contentHash}
```

---

## 2. Batch Upload with Chunking

```mermaid
sequenceDiagram
    participant F as Frontend
    participant B as AssetHandler
    participant SM as SessionManager
    participant CM as ChunkMerger
    participant S as StagingManager
    participant Q as River Queue

    Note over F,Q: Field naming convention:<br/>single_{sessionId} — complete file<br/>chunk_{sessionId}_{index}_{total} — chunk

    F->>B: GET /api/v1/assets/batch/config
    B-->>F: {chunkSize, maxConcurrent, memoryBuffer, ...}

    F->>F: Split large file into chunks
    F->>B: POST /api/v1/assets/batch<br/>multipart stream (multiple parts)

    loop For each part in multipart stream
        B->>B: ParseFileField(fieldName) → {type, sessionID, chunkIndex, totalChunks}
        B->>SM: CreateSession / GetSession
        B->>S: CreateStagingFile(repoPath, chunkName)
        B->>B: io.CopyBuffer(part → stagingFile) using 1MB shared buffer
        B->>SM: UpdateSessionChunk(sessionID, index, bytesWritten)
    end

    loop For each completed session
        alt Single file (type="single")
            B->>B: processCompletedUpload(file, session, repo, path)
            B->>Q: Insert(IngestAssetArgs)
        else Chunked file — all chunks received
            B->>SM: IsSessionComplete(sessionID) → true
            B->>SM: UpdateStatus("merging")
            B->>CM: MergeChunks(sessionID, totalChunks, repoPath)
            CM-->>B: {MergedFilePath, TotalSize}
            B->>B: processCompletedUpload(merged, session, repo, mergedPath)
            B->>Q: Insert(IngestAssetArgs)
            B->>CM: CleanupChunks(sessionID)
        else Chunked file — not complete yet
            B-->>F: {status:"uploading", progress: "65.3% complete"}
        end
    end

    B-->>F: BatchUploadResponseDTO{results: [...]}
```

---

## 3. Upload Session Lifecycle

```mermaid
stateDiagram-v2
    [*] --> pending: CreateSession
    pending --> uploading: First chunk received
    uploading --> uploading: More chunks arrive
    uploading --> merging: All chunks received
    merging --> completed: Merge + enqueue success
    merging --> failed: Merge error

    uploading --> failed: Upload error
    completed --> [*]: Cleanup (30min TTL)
    failed --> [*]: Cleanup

    note right of uploading
        SessionManager tracks:
        - received_chunks[]
        - bytes_received
        - last_activity
    end note
```

```mermaid
graph LR
    subgraph "UploadSession State"
        ID["session_id: UUID"]
        FN["filename"]
        TC["total_chunks"]
        RC["received_chunks: int array"]
        BR["bytes_received: int64"]
        CH["content_hash: string"]
        ST["status: pending/uploading/merging/completed/failed"]
        LA["last_activity: time"]
    end
```

---

## 4. Ingest Worker — Asset Creation & Fan-out

```mermaid
flowchart TD
    START[IngestAssetWorker receives job] --> REPO[Resolve repository by ID or fallback]
    REPO --> VALIDATE[ValidateFile: extension + MIME check]
    VALIDATE --> STAT[os.Stat staged file → get size]
    STAT --> HASH{Hash provided?}
    HASH -->|Yes| NORM[Normalize hash lowercase]
    HASH -->|No| CALC[CalculateFileHash BLAKE3]
    CALC --> NORM

    NORM --> OWNER[Parse owner: userID → int32 or lookup by username]
    OWNER --> STATUS[Build initial TrackedProcessingStatus<br/>tasks: metadata + thumbnail/transcode]
    STATUS --> CREATE[CreateAssetRecord in DB<br/>status=processing, storage_path=nil]

    CREATE --> COMMIT[CommitStagingFileToInbox<br/>staging → inbox/YYYY/MM/hash_filename]
    COMMIT -->|Success| UPDATE[Update asset: set storage_path]
    COMMIT -->|Failure| MOVE_FAIL[MoveStagingToFailed<br/>Mark asset status=failed]

    UPDATE --> ENQUEUE_META[Enqueue: metadata_asset]

    ENQUEUE_META --> TYPE{Asset Type?}

    TYPE -->|PHOTO| THUMB[Enqueue: thumbnail_asset]
    THUMB --> ML[Enqueue ML jobs<br/>based on settings]

    TYPE -->|VIDEO| THUMB_V[Enqueue: thumbnail_asset]
    THUMB_V --> TRANSCODE[Enqueue: transcode_asset]

    TYPE -->|AUDIO| TRANSCODE_A[Enqueue: transcode_asset]

    ML --> DONE[Done — asset processing in parallel]
    TRANSCODE --> DONE
    TRANSCODE_A --> DONE
```

---

## 5. Processing Pipeline (Parallel Workers)

```mermaid
graph TB
    subgraph "Ingest (entry)"
        INGEST[ingest_asset<br/>MaxWorkers: 50]
    end

    subgraph "Core Pipeline (always)"
        META[metadata_asset<br/>MaxWorkers: 20<br/>EXIF / ffprobe extraction]
    end

    subgraph "Media Generation"
        THUMB[thumbnail_asset<br/>MaxWorkers: CPU/2<br/>small/medium/large]
        TRANS[transcode_asset<br/>MaxWorkers: 1<br/>H.264 ≤1080p / MP3]
    end

    subgraph "ML Pipeline (Photo only, config-gated)"
        CLIP[process_clip<br/>MaxWorkers: 2<br/>CLIP embeddings + classification]
        BIOCLIP[process_bioclip<br/>MaxWorkers: 2<br/>Bio species classification]
        OCR[process_ocr<br/>MaxWorkers: 3<br/>Text extraction]
        CAPTION[process_caption<br/>MaxWorkers: 1<br/>AI image captioning]
        FACE[process_face<br/>MaxWorkers: 2<br/>Face detection + recognition]
    end

    INGEST -->|always| META
    INGEST -->|PHOTO| THUMB
    INGEST -->|PHOTO| CLIP & BIOCLIP & OCR & CAPTION & FACE
    INGEST -->|VIDEO| THUMB
    INGEST -->|VIDEO| TRANS
    INGEST -->|AUDIO| TRANS

    style INGEST fill:#f96
    style META fill:#69f
    style THUMB fill:#6c6
    style TRANS fill:#6c6
    style CLIP fill:#c6f
    style BIOCLIP fill:#c6f
    style OCR fill:#c6f
    style CAPTION fill:#c6f
    style FACE fill:#c6f
```

---

## 6. Pipeline Task Status Tracking

```mermaid
stateDiagram-v2
    [*] --> pending: buildTrackedProcessingStatus
    pending --> processing: Worker picks up job
    processing --> complete: Task succeeds
    processing --> failed: Task errors

    state "Per-Asset Status" as PAS {
        [*] --> processing_overall
        processing_overall --> complete_overall: All tasks complete
        processing_overall --> failed_overall: Any task failed
        processing_overall --> warning: Some tasks failed, others complete
    }

    note right of PAS
        Asset.status JSON tracks each task:
        {
          "state": "processing",
          "tasks": {
            "metadata_asset": {"state":"complete"},
            "thumbnail_asset": {"state":"processing"},
            "process_clip": {"state":"pending"}
          }
        }
    end note
```

---

## 7. Repository Scan (Filesystem Discovery)

```mermaid
sequenceDiagram
    participant Admin as Admin User
    participant B as Backend
    participant Q as River Queue
    participant SCAN_W as ScanRepositoryWorker
    participant FS as Filesystem
    participant DISC_W as DiscoverAssetWorker
    participant DB as PostgreSQL

    Admin->>B: POST /api/v1/repositories/:id/scan
    B->>Q: Insert(ScanRepositoryArgs)<br/>queue: "scan_repository"
    B-->>Admin: Scan queued

    Q->>SCAN_W: Execute scan
    SCAN_W->>FS: Walk repository user-space<br/>(exclude .lumilio/ and inbox/)
    
    loop For each discovered file
        SCAN_W->>Q: Insert(DiscoverAssetArgs)<br/>{repoID, relativePath, fileName, operation:"upsert"}
    end

    loop For each deleted file
        SCAN_W->>Q: Insert(DiscoverAssetArgs)<br/>{operation:"delete"}
    end

    Q->>DISC_W: Process discovered asset

    alt operation = "delete"
        DISC_W->>DB: SoftDeleteAssetByRepositoryAndStoragePath
    else operation = "upsert"
        DISC_W->>FS: os.Stat(fullPath)
        DISC_W->>DISC_W: ValidateFile + CalculateHash(BLAKE3)
        DISC_W->>DB: Check existing asset by repo+path

        alt Existing asset (unchanged hash + size)
            DISC_W->>DISC_W: Skip (no-op)
        else New or changed file
            DISC_W->>DB: CREATE or UPDATE asset record
            DISC_W->>Q: Enqueue downstream pipeline<br/>(metadata, thumbnail, transcode, ML)
        end
    end
```

---

## 8. Storage Architecture

```mermaid
graph TD
    subgraph "Repository Root"
        subgraph ".lumilio/ (system-managed, protected)"
            STAGING["staging/<br/>├── incoming/ (active uploads)<br/>└── failed/ (failed uploads)"]
            ASSETS_DIR["assets/<br/>├── thumbnails/<br/>│   ├── small/ (150px)<br/>│   ├── medium/ (300px)<br/>│   └── large/ (preview)<br/>├── videos/web/ (H.264 ≤1080p)<br/>└── audios/web/ (MP3)"]
            TEMP["temp/ (processing workspace)"]
            TRASH["trash/ (soft-delete, 30-day TTL)"]
            LOGS["logs/"]
        end

        subgraph "inbox/ (app-managed, protected)"
            DATE["Date strategy (default):<br/>inbox/2026/05/hash_photo.jpg"]
            FLAT["Flat strategy:<br/>inbox/photo.jpg"]
            CAS["CAS strategy:<br/>inbox/aa/bb/hash.ext"]
        end

        subgraph "User Space (unprotected)"
            USER["Family Photos/<br/>Vacations/<br/>..."]
        end

        CONFIG[".lumiliorepo (YAML config)"]
    end

    style STAGING fill:#ffa
    style ASSETS_DIR fill:#afa
    style TEMP fill:#ffa
```

---

## 9. Storage Strategies (Inbox Commit)

```mermaid
flowchart TD
    COMMIT[CommitStagingFileToInbox] --> LOAD[Load .lumiliorepo config]
    LOAD --> STRATEGY{storage_strategy?}

    STRATEGY -->|"date" (default)| DATE_PATH["inbox/YYYY/MM/filename<br/>e.g. inbox/2026/05/IMG_001.jpg"]
    STRATEGY -->|"flat"| FLAT_PATH["inbox/filename<br/>e.g. inbox/IMG_001.jpg"]
    STRATEGY -->|"cas"| CAS_PATH["inbox/aa/bb/cc/hash.ext<br/>e.g. inbox/a1/b2/c3d4e5...jpg"]

    DATE_PATH --> DUP{Duplicate filename?}
    FLAT_PATH --> DUP
    CAS_PATH --> DONE[Move staging → final path]

    DUP -->|handle_duplicate: "rename"| RENAME["filename (1).jpg"]
    DUP -->|handle_duplicate: "uuid"| UUID_SUFFIX["filename_uuid.jpg"]
    DUP -->|handle_duplicate: "overwrite"| OVERWRITE[Replace existing]
    DUP -->|No duplicate| DONE

    RENAME --> DONE
    UUID_SUFFIX --> DONE
    OVERWRITE --> DONE
```

---

## 10. Upload Configuration (Memory-Adaptive)

```mermaid
flowchart LR
    subgraph "GET /assets/batch/config"
        MM[MemoryMonitor] --> |GetOptimalChunkConfig| CONFIG
    end

    subgraph CONFIG["Response"]
        CS[chunkSize: 5MB default]
        MC[maxConcurrent: 3]
        MB[memoryBuffer: 100MB]
        UI[updateInterval: 30s]
        MGC[mergeConcurrency: 2]
        MIR[maxInFlightRequests: 3]
    end

    subgraph "Server Limits"
        UL[uploadLimiter: chan(32)<br/>HTTP/2 multiplexing]
        ST[Session timeout: 30min]
        BG[Background cleanup: expired sessions]
    end
```

---

## 11. File Validation

```mermaid
flowchart TD
    INPUT[filename + Content-Type] --> EXT[Extract file extension]
    EXT --> LOOKUP[Lookup in supported types registry]
    
    LOOKUP --> VALID{Supported?}
    VALID -->|Yes| RESULT["ValidationResult{<br/>Valid: true,<br/>AssetType: PHOTO/VIDEO/AUDIO,<br/>MimeType: canonical MIME,<br/>IsRAW: bool<br/>}"]
    VALID -->|No| REJECT["ValidationResult{<br/>Valid: false,<br/>ErrorReason: 'unsupported...'<br/>}"]

    subgraph "Supported Types"
        PHOTO_T["PHOTO: jpg, png, gif, webp, heic, heif,<br/>tiff, bmp, avif, svg, RAW formats"]
        VIDEO_T["VIDEO: mp4, mov, avi, mkv, webm, flv, wmv"]
        AUDIO_T["AUDIO: mp3, wav, flac, aac, ogg, m4a, wma"]
    end
```

---

## 12. Complete Upload-to-Searchable Flow

```mermaid
flowchart TD
    subgraph "Upload Phase"
        A[User selects file] --> B[Frontend: BLAKE3 hash in Web Worker]
        B --> C[POST /assets or /assets/batch]
        C --> D[Handler: validate + stage + hash]
        D --> E[Enqueue ingest_asset job]
    end

    subgraph "Ingest Phase"
        E --> F[IngestWorker: create DB record]
        F --> G[Commit staging → inbox]
        G --> H[Fan-out downstream jobs]
    end

    subgraph "Processing Phase (parallel)"
        H --> I[metadata_asset:<br/>EXIF/ffprobe → DB<br/>GPS, camera, lens, dates]
        H --> J[thumbnail_asset:<br/>Generate small/medium/large<br/>→ .lumilio/assets/thumbnails/]
        H --> K[transcode_asset:<br/>Video → H.264 ≤1080p<br/>Audio → MP3]
    end

    subgraph "ML Phase (Photo, config-gated, parallel)"
        H --> L[process_clip:<br/>CLIP embedding → pgvector]
        H --> M[process_bioclip:<br/>Species classification → tags]
        H --> N[process_ocr:<br/>Text extraction → full-text search]
        H --> O[process_caption:<br/>AI description → captions table]
        H --> P[process_face:<br/>Detection + recognition → people]
    end

    subgraph "Ready State"
        I & J & K & L & M & N & O & P --> Q[Asset status: complete<br/>Browsable, searchable, streamable]
    end

    style A fill:#f96
    style Q fill:#6f6
```

---

## 13. Error Handling & Retry

```mermaid
flowchart TD
    JOB[River Job Execution] --> SUCCESS{Success?}
    SUCCESS -->|Yes| MARK_DONE[MarkTaskComplete<br/>Update asset status JSON]
    SUCCESS -->|No| MARK_FAIL[MarkTaskFailed<br/>Update asset status JSON with error detail]
    MARK_FAIL --> RIVER_RETRY{River auto-retry?}
    RIVER_RETRY -->|Within retry limit| RETRY[Re-enqueue job]
    RIVER_RETRY -->|Exhausted| FINAL_FAIL[Asset status → failed/warning]

    FINAL_FAIL --> MANUAL[Admin can trigger:<br/>POST /assets/:id/reprocess]
    MANUAL --> RETRY_W[AssetRetryWorker<br/>queue: "retry_asset"]
    RETRY_W --> SELECTIVE[Retry only failed tasks<br/>or force full retry]
    SELECTIVE --> JOB
```

---

## API Route Map

| Method | Path | Auth | Description |
|--------|------|------|-------------|
| POST | `/assets` | Optional | Upload single file |
| POST | `/assets/batch` | Optional | Batch upload with chunk support |
| GET | `/assets/batch/config` | — | Get memory-adaptive upload config |
| GET | `/assets/batch/progress` | — | Get upload progress for sessions |
| POST | `/assets/:id/reprocess` | Optional | Retry failed processing tasks |
| POST | `/repositories` | Admin | Create new repository |
| POST | `/repositories/:id/scan` | Admin | Queue filesystem scan |
| GET | `/repositories/:id/scans/latest` | Admin | Get latest scan result |
| GET | `/repositories/:id/scans` | Admin | List scan history |

---

## Queue Configuration

| Queue | MaxWorkers | Purpose |
|-------|-----------|---------|
| `ingest_asset` | 50 | Initial staging → inbox commit |
| `discover_asset` | 20 | Filesystem discovery ingestion |
| `metadata_asset` | 20 | EXIF / ffprobe extraction |
| `thumbnail_asset` | CPU/2 | Multi-size thumbnail generation |
| `transcode_asset` | 1 | Video/audio transcoding (resource-heavy) |
| `process_clip` | 2 | CLIP embedding + classification |
| `process_bioclip` | 2 | BioCLIP species classification |
| `process_ocr` | 3 | Text extraction |
| `process_caption` | 1 | AI image captioning |
| `process_face` | 2 | Face detection + recognition |
| `retry_asset` | 2 | Selective task retry |
| `reindex_assets` | 1 | Batch reindex backfill |
| `scan_repository` | 1 | Repository tree scan |
| `rebuild_location_clusters` | 1 | Geo clustering rebuild |
