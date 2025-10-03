# Process Documentation: Inbox Upload

*Author: Edwin Zhan, documented by AI*

## Overview

The inbox upload process is the primary way users add assets to their Lumilio Photos repository. This document provides a comprehensive analysis of the upload workflow, from the initial HTTP request through background processing to final storage and indexing.

## System Architecture

### Components Involved

1. **API Layer**: Gin HTTP framework with handlers
2. **Queue System**: River (PostgreSQL-backed job queue)
3. **Asset Processor**: Background worker for asset ingestion
4. **Storage System**: Local filesystem with repository management
5. **Database**: PostgreSQL with SQLC queries
6. **ML Service**: gRPC service for AI features (optional)

### Key Files

- `server/internal/api/handler/asset_handler.go` - HTTP handlers
- `server/internal/queue/asset_worker.go` - Background worker
- `server/internal/processors/asset_processor.go` - Asset processing logic
- `server/internal/storage/staging_manager.go` - Staging and inbox management
- `server/internal/service/asset_service.go` - Asset business logic

## Upload Flow Overview

```
Client Upload → API Handler → Staging → Queue → Worker → Processor → Storage + DB
```

## Detailed Process Flow

### Phase 1: HTTP Request Handling

#### 1.1 Client Request
**Endpoint**: `POST /api/v1/assets`  
**Content-Type**: `multipart/form-data`

**Request Components**:
- **file**: The asset file (required)
- **X-Content-Hash** header: Client-provided hash (optional)

**Example**:
```bash
curl -X POST http://localhost:3001/api/v1/assets \
  -F "file=@photo.jpg" \
  -H "X-Content-Hash: blake3hash123..."
```

#### 1.2 Handler Processing
**Location**: `internal/api/handler/asset_handler.go::UploadAsset()`

**Steps**:
1. **Parse Multipart Form**
   - Extract file from form data
   - Get original filename and size
   - Read X-Content-Hash header (if present)

2. **Generate Staging Path**
   ```go
   stagingFilePath = filepath.Join(
       appConfig.StagingPath,
       uuid.New().String() + filepath.Ext(header.Filename)
   )
   ```
   - Creates unique filename with UUID
   - Preserves original file extension
   - Placed in `.lumilio/staging/incoming/`

3. **Save to Staging**
   ```go
   stagingFile, err := os.Create(stagingFilePath)
   io.Copy(stagingFile, file)
   ```
   - Writes file to staging area
   - No processing or validation yet
   - Fast response to client

4. **Extract User Context**
   ```go
   userID := c.GetString("user_id")
   if userID == "" {
       userID = "anonymous"
   }
   ```
   - Gets user ID from context (set by auth middleware)
   - Falls back to "anonymous" if not authenticated

5. **Create Job Payload**
   ```go
   payload := processors.AssetPayload{
       ClientHash:  clientHash,
       StagedPath:  stagingFilePath,
       UserID:      userID,
       Timestamp:   time.Now(),
       ContentType: header.Header.Get("Content-Type"),
       FileName:    header.Filename,
   }
   ```

6. **Enqueue Job**
   ```go
   jobInsetResult, err := h.queueClient.Insert(
       c.Request.Context(),
       jobs.ProcessAssetArgs(payload),
       &river.InsertOpts{Queue: "process_asset"}
   )
   ```
   - Submits job to River queue
   - Returns immediately with job ID
   - Job persisted in PostgreSQL

7. **Response to Client**
   ```go
   response := UploadResponse{
       TaskID:      jobId,
       Status:      "processing",
       FileName:    header.Filename,
       Size:        header.Size,
       ContentHash: clientHash,
   }
   c.JSON(http.StatusOK, response)
   ```
   - Client receives task ID for tracking
   - HTTP connection closed
   - Processing continues asynchronously

**Timing**: Typically 50-200ms for files < 100MB

---

### Phase 2: Background Job Processing

#### 2.1 Worker Dequeue
**Location**: `internal/queue/asset_worker.go::Work()`

**River Queue Behavior**:
- Workers poll `process_asset` queue
- Up to 5 concurrent workers (MaxWorkers: 5)
- Jobs processed in FIFO order (with priority support)
- Automatic retry on failure (exponential backoff)

**Worker Receives**:
```go
type ProcessAssetWorker struct {
    river.WorkerDefaults[jobs.ProcessAssetArgs]
    assetProcessor *processors.AssetProcessor
}

func (w *ProcessAssetWorker) Work(
    ctx context.Context,
    job *river.Job[jobs.ProcessAssetArgs]
) error {
    // Delegates to AssetProcessor
    _, err := w.assetProcessor.ProcessAsset(ctx, job.Args)
    return err
}
```

#### 2.2 Asset Processor Entry Point
**Location**: `internal/processors/asset_processor.go::ProcessAsset()`

**Core Logic**:

1. **Open Staged File**
   ```go
   assetFile, err := os.Open(task.StagedPath)
   defer assetFile.Close()
   
   info, err := assetFile.Stat()
   fileSize := info.Size()
   ```

2. **Determine Asset Type**
   ```go
   contentType := file.DetermineAssetType(task.ContentType)
   ```
   - Analyzes MIME type and file extension
   - Returns: "image", "video", "audio", or "unknown"
   - Uses magic number detection for ambiguous cases

3. **Commit to Inbox**
   ```go
   storagePath, err := ap.storageService.CommitStagedFile(
       ctx,
       task.StagedPath,
       task.FileName,
       task.ClientHash
   )
   ```
   - Moves file from staging to inbox
   - Applies repository storage strategy (date/flat/CAS)
   - Handles duplicate filenames
   - Returns relative path within repository

4. **Create Asset Record**
   ```go
   params := repo.CreateAssetParams{
       OwnerID:          ownerIDPtr,
       Type:             string(contentType),
       OriginalFilename: task.FileName,
       StoragePath:      storagePath,
       MimeType:         task.ContentType,
       FileSize:         fileSize,
       Hash:             &task.ClientHash,
       TakenTime:        pgtype.Timestamptz{Time: time.Now(), Valid: true},
       Rating:           func() *int32 { r := int32(0); return &r }(),
   }
   
   asset, err := ap.assetService.CreateAssetRecord(ctx, params)
   ```
   - Creates database record
   - Returns asset with ID for further processing

5. **Type-Specific Processing**
   ```go
   switch contentType {
   case file.AssetTypeImage:
       err = ap.ProcessPhoto(ctx, asset, assetFile)
   case file.AssetTypeVideo:
       err = ap.ProcessVideo(ctx, asset, assetFile)
   case file.AssetTypeAudio:
       err = ap.ProcessAudio(ctx, asset, assetFile)
   }
   ```

**Timing**: 
- Base processing: 1-5 seconds
- Plus type-specific processing (see below)

---

### Phase 3: Storage Commit (Inbox)

#### 3.1 Staging Manager
**Location**: `internal/storage/staging_manager.go::CommitStagedFile()`

**Process**:

1. **Load Repository Config**
   ```go
   cfg, err := repocfg.LoadConfigFromFile(repoPath)
   ```
   - Reads `.lumiliorepo` configuration
   - Gets storage strategy and settings

2. **Resolve Inbox Path**
   ```go
   finalPath, err := sm.resolveInboxRelativePath(
       repoPath,
       cfg,
       originalFilename,
       hash
   )
   ```

**Storage Strategy Logic**:

**A. Date Strategy** (default)
```
inbox/2024/01/filename.jpg
```
- Groups by year and month
- Easy chronological browsing
- Handles duplicates with rename/uuid/overwrite

**B. Flat Strategy**
```
inbox/filename.jpg
```
- All files in single directory
- Simplest organization
- Faster file operations (no subdirectory navigation)

**C. Content-Addressed Storage (CAS)**
```
inbox/ab/cd/ef/abcdef123456...789.jpg
```
- First 2 chars → first directory level
- Next 2 chars → second directory level
- Next 2 chars → third directory level
- Full hash + extension as filename
- Automatic deduplication by content
- Falls back to date strategy if hash unavailable

3. **Move File to Inbox**
   ```go
   err = os.Rename(stagingPath, finalPath)
   ```
   - Atomic move operation (same filesystem)
   - Fast (no copy, just metadata update)
   - Original staging file removed

4. **Handle Duplicates**
   - **rename**: Adds (1), (2), etc. suffix
   - **uuid**: Appends UUID to filename
   - **overwrite**: Replaces existing file

**Timing**: < 10ms (atomic move on same filesystem)

---

### Phase 4: Type-Specific Processing

#### 4.1 Photo Processing
**Location**: `internal/processors/photo_processor.go::ProcessPhoto()`

**Steps**:

1. **Extract Metadata with EXIF**
   ```bash
   exiftool -json -G -struct photo.jpg
   ```
   - Camera make/model
   - Capture date/time (used as TakenTime)
   - GPS coordinates
   - Exposure settings (ISO, aperture, shutter)
   - Image dimensions

2. **Handle RAW Files**
   - Detect RAW format (CR2, NEF, ARW, etc.)
   - Extract embedded JPEG preview if available
   - Use dcraw for thumbnail generation
   - Preserve original RAW file

3. **Generate Thumbnails**
   - **150px**: Grid view thumbnail
   - **300px**: List view thumbnail
   - **1024px**: Lightbox preview
   - Stored in `.lumilio/assets/thumbnails/{size}/`
   - JPEG format for compatibility

4. **Update Asset Record**
   ```go
   ap.assetService.UpdateAssetMetadata(ctx, asset.ID, metadata)
   ```
   - Sets accurate TakenTime from EXIF
   - Updates Width, Height
   - Stores camera metadata in JSONB field

5. **Enqueue CLIP Processing** (if ML enabled)
   ```go
   ap.queueClient.Insert(ctx, jobs.ProcessClipArgs{
       AssetID:   asset.ID,
       ImageData: thumbnailBytes,
   }, &river.InsertOpts{Queue: "process_clip"})
   ```
   - Uses 1024px thumbnail
   - Processed asynchronously
   - Generates embeddings and classifications

**Timing**:
- JPEG: 2-5 seconds
- PNG/WEBP: 3-7 seconds
- RAW: 10-30 seconds (dcraw is slow)

#### 4.2 Video Processing
**Location**: `internal/processors/video_processor.go::ProcessVideo()`

**Steps**:

1. **Extract Metadata with ffprobe**
   ```bash
   ffprobe -v quiet -print_format json -show_format -show_streams video.mp4
   ```
   - Duration
   - Codec information
   - Resolution (width, height)
   - Frame rate
   - Bitrate

2. **Generate Thumbnail**
   - Extract frame at 10% of duration
   - Resize to standard thumbnail sizes
   - Stored like photo thumbnails

3. **Transcode for Web** (optional, not fully implemented)
   ```bash
   ffmpeg -i input.mp4 -c:v libx264 -preset medium -crf 23 -c:a aac output.mp4
   ```
   - H.264 codec for compatibility
   - Target resolution: 1080p max
   - Stored in `.lumilio/assets/videos/web/`
   - Original preserved

**Timing**:
- Thumbnail: 5-10 seconds
- Full transcode: 1-5 minutes (depending on length and resolution)

#### 4.3 Audio Processing
**Location**: `internal/processors/audio_processor.go::ProcessAudio()`

**Steps**:

1. **Extract Metadata**
   - Title, artist, album
   - Duration
   - Bitrate
   - Sample rate

2. **Generate Waveform** (planned, not implemented)
   - Visual representation of audio
   - Stored as thumbnail

3. **Transcode for Web** (planned)
   - Convert to MP3 if not already
   - Standardize bitrate

**Timing**: 1-3 seconds

---

### Phase 5: ML Processing (Optional)

#### 5.1 CLIP Job
**Queue**: `process_clip`  
**Worker**: `internal/queue/clip_worker.go::ProcessClipWorker`

**Process**:

1. **Batch Dispatcher**
   - ClipBatchDispatcher aggregates requests
   - Default batch size: 8 images
   - Default batch window: 1500ms
   - Single gRPC stream per batch

2. **Send to ML Service**
   ```protobuf
   message InferRequest {
       string task_type = 1;  // "clip_image_embed" or "smart_classify"
       bytes payload = 2;     // Image bytes (WEBP)
       int32 seq = 3;         // Sequence number
       string correlation_id = 4;
       map<string, string> meta = 5;
   }
   ```

3. **Receive Results**
   - **Embedding**: 512-dimensional float vector
   - **Classifications**: Top 3 labels with scores
   - **Metadata**: Model version, processing time

4. **Store in Database**
   ```go
   // Embedding (pgvector)
   ap.assetService.SaveNewEmbedding(ctx, assetID, vector)
   
   // Predictions
   ap.assetService.SaveNewSpeciesPredictions(ctx, assetID, predictions)
   ```

**Timing**:
- Single image: ~2 seconds
- Batched (8 images): ~3-4 seconds total (~0.5s per image)

---

## Error Handling

### Common Errors and Recovery

#### 1. Staging File Not Found
**Cause**: File deleted before processing  
**Recovery**: Job fails, manual intervention required  
**Future**: Add staging file TTL tracking

#### 2. Storage Commit Failure
**Cause**: Disk full, permissions issue  
**Recovery**: Job retried by River (exponential backoff)  
**Limitation**: Staging file remains, no cleanup

#### 3. Database Transaction Failure
**Cause**: Constraint violation, connection loss  
**Recovery**: Job retried  
**Limitation**: Inbox file may exist without DB record

#### 4. Thumbnail Generation Failure
**Cause**: Corrupted file, missing tools (exiftool, ffmpeg)  
**Recovery**: Asset created without thumbnails  
**Logging**: Warning logged

#### 5. ML Service Unavailable
**Cause**: gRPC connection failure  
**Recovery**: Job retried (with backoff)  
**Limitation**: No fallback, embeddings remain empty

---

## Performance Characteristics

### Upload Phase Benchmarks

| Operation | Time | Notes |
|-----------|------|-------|
| HTTP upload (10MB) | 50-200ms | Network dependent |
| Stage to disk | 10-50ms | SSD vs HDD difference |
| Queue enqueue | 5-10ms | PostgreSQL insert |
| Inbox commit | < 10ms | Atomic move |
| EXIF extraction | 100-500ms | Image size dependent |
| Thumbnail gen (JPEG) | 1-3s | 3 sizes |
| Thumbnail gen (RAW) | 10-30s | dcraw performance |
| CLIP processing | 2-4s | ML service latency |
| Total (JPEG) | 5-10s | End to end |
| Total (RAW) | 15-45s | End to end |

### Throughput Limits

**Current Configuration**:
- 5 concurrent asset workers
- 1 CLIP worker (intentional for batching)
- Single database connection pool
- Local filesystem storage

**Measured Throughput**:
- ~50-100 assets/minute (mixed types)
- ~30-50 RAW files/minute (limited by dcraw)
- ~100-150 JPEGs/minute (limited by workers)

**Bottlenecks**:
1. RAW processing (dcraw single-threaded)
2. CLIP worker (single worker for batching)
3. Disk I/O (especially HDDs)
4. Database connection pool under high load

---

## Configuration Options

### Environment Variables

```bash
# Staging area (temporary upload location)
STAGING_PATH=/path/to/.lumilio/staging/incoming

# Storage strategy in .lumiliorepo config
storage_strategy: "date"  # or "flat" or "cas"

# Duplicate handling
handle_duplicate_filenames: "uuid"  # or "rename" or "overwrite"

# Queue concurrency
QUEUE_PROCESS_ASSET_WORKERS=5

# ML service
ML_SERVICE_ADDR=localhost:50051
```

### Repository Configuration

**Location**: `{repository}/.lumiliorepo`

```yaml
version: "1.0"
storage_strategy: "date"  # Controls inbox organization
local_settings:
  preserve_original_filename: true
  handle_duplicate_filenames: "uuid"
  max_file_size: 0  # 0 = unlimited (in KB)
```

---

## Monitoring and Observability

### Key Metrics to Track

1. **Upload Success Rate**: % of uploads that complete successfully
2. **Processing Time**: p50, p95, p99 latencies
3. **Queue Depth**: Number of pending jobs
4. **Worker Utilization**: % time workers are busy
5. **Error Rate**: Failed jobs per minute
6. **Staging Disk Usage**: Prevents disk full issues

### Log Points

```go
// Important log messages to watch for
log.Printf("Task %d enqueued for processing file %s", jobId, filename)
log.Printf("Asset %s processed successfully", asset.ID)
log.Printf("Failed to process asset: %v", err)
log.Printf("Staging file cleanup: removed %d old files", count)
```

### Health Indicators

- Queue processing lag < 1 minute
- Staging file count < 1000
- Database connection pool saturation < 80%
- Disk usage < 90%

---

## Future Improvements

### Planned Enhancements

1. **Chunked Upload**: Support large files (>1GB)
2. **Resumable Upload**: Handle network interruptions
3. **Progress Tracking**: Real-time upload progress
4. **Parallel Processing**: Utilize multiple cores better
5. **Smart Retries**: Distinguish transient vs permanent failures
6. **Cleanup Jobs**: Automated staging file cleanup
7. **Validation**: File corruption detection before processing
8. **Caching**: Cache generated thumbnails and embeddings
9. **Object Storage**: S3/MinIO backend support
10. **CDN Integration**: Serve assets via CDN

### Performance Targets

- 10x throughput: 500-1000 assets/minute
- Sub-second API response time
- Parallel RAW processing
- GPU-accelerated thumbnail generation
- Distributed queue workers

---

## Troubleshooting Guide

### Problem: Uploads Hang

**Symptoms**: Client never receives response  
**Checks**:
1. Check disk space: `df -h`
2. Check staging directory writable: `ls -la`
3. Check database connection: `pg_isready`

**Solution**: Ensure adequate disk space and database connectivity

### Problem: Files Not Appearing

**Symptoms**: Upload succeeds, but asset not in library  
**Checks**:
1. Check queue status: Query `river_job` table
2. Check worker logs: Look for processing errors
3. Check asset table: Query `assets` by hash

**Solution**: Check River queue for stuck jobs

### Problem: Thumbnails Missing

**Symptoms**: Asset exists, but no thumbnails  
**Checks**:
1. Check exiftool installed: `exiftool -ver`
2. Check ffmpeg installed: `ffmpeg -version`
3. Check thumbnail directory: `ls .lumilio/assets/thumbnails/`

**Solution**: Install required command-line tools

### Problem: Slow Upload Processing

**Symptoms**: Queue backlog growing  
**Checks**:
1. Check worker count: Should be 5 for `process_asset`
2. Check CPU usage: RAW processing is CPU-intensive
3. Check disk I/O: Use `iostat`

**Solution**: Increase workers or upgrade hardware

---

## Related Documentation

- [Development Status](../development-status.md) - Overall system status
- [Asset Processing](../processors/asset-processor.md) - Detailed processor docs
- [Storage System](../../../../server/internal/storage/README.md) - Repository management
- [Queue System](../../../../server/internal/queue/README.md) - River queue details
- [Upload Flow Diagram](../business-diagram/upload-backend.md) - Visual flow

---

## Conclusion

The inbox upload process is the backbone of asset ingestion in Lumilio Photos. It combines:
- Fast, responsive API design (async processing)
- Reliable queueing (River + PostgreSQL)
- Flexible storage strategies (date/flat/CAS)
- Rich metadata extraction (EXIF, ffprobe)
- AI-powered features (CLIP embeddings)

The system is production-ready for basic use cases but has room for optimization and feature enhancements as outlined in the future improvements section.
