# Asset Processing and Retry System

## Overview

This document describes the comprehensive asset processing and retry system implemented in Lumilio Photos. The system provides robust error handling, selective task retry capabilities, and detailed status tracking for asset processing operations.

## Core Components

### 1. Asset Status Management

The system uses a JSONB field `assets.status` to track the complete lifecycle of asset processing:

```go
type AssetStatus struct {
    State     AssetState    `json:"state"`
    Message   string        `json:"message"`
    Errors    []ErrorDetail `json:"errors,omitempty"`
    UpdatedAt string        `json:"updated_at"`
}
```

**Supported States:**
- `processing`: Asset is currently being processed
- `complete`: Asset has been successfully processed
- `warning`: Asset was processed with some non-fatal errors
- `failed`: Asset processing failed completely

### 2. Error Detail Tracking

Each processing error is tracked with detailed information:

```go
type ErrorDetail struct {
    Task  string `json:"task"`
    Error string `json:"error"`
    Time  string `json:"time,omitempty"`
}
```

### 3. File Directory Flow

```
staging/
├── incoming/     # New uploads awaiting processing
├── failed/       # Files with fatal processing errors
└── inbox/        # Successfully processed files (complete/warning)
```

## Processing Pipeline

### Initial Processing Flow (`AssetProcessor.ProcessAsset`)

1. **Task Reception**: Worker receives `process_asset` task
2. **Initialization**: Create asset record with `storage_path = NULL` and initial processing status
3. **Parallel Processing**: Use `errgroup.FaultTolerant` for parallel task execution
4. **Error Collection**: Collect all sub-task errors without stopping on first failure
5. **Status Decision**: Determine final status based on collected errors
6. **File Movement**: Move file to appropriate directory based on outcome

### Status Decision Logic

```go
if len(processingErrors) == 0 {
    // All processing succeeded
    finalStatus = status.NewCompleteStatus()
    Move file to inbox/
} else {
    // Some processing failed
    finalStatus = status.NewWarningStatus("Asset processed with warnings", processingErrors)
    Move file to inbox/ (file is still usable)
}
```

## Retry System

### Automatic Retry (Queue-Based)

- **Trigger**: Transient errors (database connection issues, temporary file system unavailability)
- **Mechanism**: River queue automatically retries failed jobs (up to 25 times)
- **Safety**: Files remain in staging during retry attempts

### Manual Retry (API-Based)

#### Full Retry
- **Use Case**: Complete reprocessing of asset
- **API**: `POST /api/v1/assets/{id}/reprocess` (empty body or `{"force_full_retry": true}`)
- **Behavior**: Resets status and enqueues new `process_asset` job

#### Selective Retry
- **Use Case**: Retry specific failed tasks only
- **API**: `POST /api/v1/assets/{id}/reprocess` with `{"tasks": ["task1", "task2"]}`
- **Behavior**: Enqueues `retry_asset` job targeting only specified tasks

### Supported Task Names

- `extract_exif` - EXIF metadata extraction
- `extract_metadata` - General metadata extraction
- `generate_thumbnails` - Thumbnail generation
- `save_thumbnails` - Thumbnail storage
- `transcode_video` - Video transcoding
- `transcode_audio` - Audio transcoding
- `generate_web_version` - Web-optimized version generation
- `clip_processing` - CLIP AI processing
- `raw_processing` - RAW file processing

## Error Classification

### Fatal Errors (Prevent Retry)
- `initial_validation` - File validation failed
- `file_read` - Cannot read file
- `file_corrupted` - File is corrupted

### Non-Fatal Errors (Allow Retry)
- All other task errors (thumbnails, metadata, transcoding, etc.)

## Implementation Details

### AssetProcessor Enhancements

The system now includes enhanced error collection:

```go
func (ap *AssetProcessor) processPhotoAssetWithErrors(
    ctx context.Context, 
    repository repo.Repository, 
    asset *repo.Asset, 
    fileReader io.Reader,
) []status.ErrorDetail {
    // Process and return detailed error information
    // instead of single error
}
```

### AssetRetryProcessor

New specialized processor for selective retry:

```go
type AssetRetryProcessor struct {
    // Dependencies same as AssetProcessor
}

func (arp *AssetRetryProcessor) RetryAsset(
    ctx context.Context, 
    task AssetRetryPayload,
) error {
    // Handle selective retry logic
}
```

### Queue Configuration

```go
Queues: map[string]river.QueueConfig{
    "process_asset": {MaxWorkers: 5},
    "process_clip":  {MaxWorkers: 1},
    "retry_asset":   {MaxWorkers: 2},  // New queue for retry jobs
}
```

## API Endpoints

### Reprocess Asset

```http
POST /api/v1/assets/{id}/reprocess
Content-Type: application/json

{
    "tasks": ["generate_thumbnails", "clip_processing"],
    "force_full_retry": false
}
```

**Response:**
```json
{
    "asset_id": "550e8400-e29b-41d4-a716-446655440000",
    "status": "queued",
    "message": "Selective retry job queued successfully",
    "retry_tasks": ["generate_thumbnails", "clip_processing"]
}
```

## Benefits

1. **Granular Error Tracking**: Know exactly which tasks failed and why
2. **Selective Recovery**: Retry only failed tasks instead of full reprocessing
3. **User Experience**: Assets remain usable even with partial processing failures
4. **Operational Efficiency**: Reduce unnecessary reprocessing of successful tasks
5. **Debugging**: Detailed error logs for troubleshooting

## Monitoring and Maintenance

- Monitor `warning` state assets for patterns of partial failures
- Use `status.errors` field for debugging processing issues
- Implement alerting for high rates of `failed` state assets
- Regularly review `staging/failed` directory for permanently failed files

## Future Enhancements

1. **Bulk Retry Operations**: API for retrying multiple assets at once
2. **Retry Policies**: Configurable retry strategies per task type
3. **Progress Tracking**: Real-time progress updates for long-running retry operations
4. **Admin Dashboard**: Web interface for monitoring and managing retry operations