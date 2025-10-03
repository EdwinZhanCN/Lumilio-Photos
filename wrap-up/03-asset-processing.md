# Asset Processing Pipeline - Detailed Analysis

## Overview

The asset processing pipeline is a type-specific, multi-stage system that transforms uploaded files into optimized, searchable, and accessible media. It handles photos, videos, and audio files with specialized sub-processors for each type.

## Architecture

### Processing Flow

```
┌──────────────┐
│ Staged File  │
└──────┬───────┘
       │
       ▼
┌──────────────────┐
│ AssetProcessor   │ ← Entry point
└──────┬───────────┘
       │
       ├─────────────────────────┐
       │                         │
       ▼                         ▼
┌─────────────┐          ┌─────────────┐
│   Detect    │          │   Record    │
│  File Type  │          │  in DB      │
└──────┬──────┘          └─────────────┘
       │
       ├───────────┬──────────┬─────────┐
       │           │          │         │
       ▼           ▼          ▼         ▼
┌──────────┐ ┌─────────┐ ┌──────┐ ┌────────┐
│  Photo   │ │  Video  │ │ Audio│ │Unknown │
│Processor │ │Processor│ │Proc. │ │        │
└────┬─────┘ └────┬────┘ └──┬───┘ └────────┘
     │            │          │
     └─────┬──────┴──────────┘
           ▼
    ┌─────────────┐
    │  Post-Proc  │
    │  (ML, etc)  │
    └─────────────┘
```

### Key Components

**Location**: `server/internal/processors/`

- **AssetProcessor** (`asset_processor.go`): Entry point and orchestration
- **PhotoProcessor** (`photo_processor.go`): Photo-specific processing
- **VideoProcessor** (`video_processor.go`): Video transcoding and extraction
- **AudioProcessor** (`audio_processor.go`): Audio metadata and waveforms

## AssetProcessor - Entry Point

### Responsibilities

1. Validate staged file
2. Determine asset type
3. Create database record
4. Commit file to inbox
5. Delegate to type-specific processor
6. Trigger ML processing

### Processing Flow

**Location**: `server/internal/processors/asset_processor.go`

```go
func (ap *AssetProcessor) ProcessAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
    // 1. Validate staged file
    info, err := os.Stat(task.StagedPath)
    if err != nil {
        return nil, fmt.Errorf("staged file not found: %w", err)
    }
    
    // 2. Get repository
    repository, err := ap.getRepository(ctx, task.RepositoryID)
    if err != nil {
        return nil, err
    }
    
    // 3. Determine content type
    contentType := file.DetermineAssetType(task.ContentType)
    
    // 4. Commit to inbox
    hash := calculateFileHash(task.StagedPath)
    stagingFile := &storage.StagingFile{...}
    err = ap.stagingManager.CommitStagingFileToInbox(stagingFile, hash)
    
    // 5. Create asset record
    asset, err := ap.createAssetRecord(ctx, repository, task, hash, contentType)
    
    // 6. Open file for processing
    file, err := os.Open(inboxPath)
    defer file.Close()
    
    // 7. Delegate to type-specific processor
    switch contentType {
    case "photo":
        return ap.processPhotoAsset(ctx, repository, asset, file)
    case "video":
        return ap.processVideoAsset(ctx, repository, asset, file)
    case "audio":
        return ap.processAudioAsset(ctx, repository, asset, file)
    default:
        return asset, nil // Unknown type, no further processing
    }
}
```

### Content Type Detection

```go
func DetermineAssetType(mimeType string) string {
    switch {
    case strings.HasPrefix(mimeType, "image/"):
        return "photo"
    case strings.HasPrefix(mimeType, "video/"):
        return "video"
    case strings.HasPrefix(mimeType, "audio/"):
        return "audio"
    default:
        return "unknown"
    }
}
```

**Supported Types**:
- **Photo**: JPEG, PNG, GIF, HEIC, WebP, RAW (CR2, NEF, ARW, DNG, etc.)
- **Video**: MP4, MOV, AVI, MKV, WebM, FLV
- **Audio**: MP3, WAV, FLAC, AAC, OGG

## Photo Processing Pipeline

### Overview

Photo processing is the most complex pipeline, involving:
1. RAW file detection and conversion
2. EXIF metadata extraction
3. Thumbnail generation (multiple sizes)
4. CLIP embedding calculation
5. Smart classification (species, objects)

### Main Entry Point

**Location**: `server/internal/processors/photo_processor.go`

```go
func (ap *AssetProcessor) processPhotoAsset(
    ctx context.Context,
    repository repo.Repository,
    asset *repo.Asset,
    fileReader io.Reader,
) error {
    // Check if RAW file
    isRAW := raw.IsRAWFile(asset.OriginalFilename)
    
    if isRAW {
        return ap.processRAWAsset(ctx, repository, asset, fileReader)
    } else {
        return ap.processStandardPhotoAsset(ctx, repository, asset, fileReader)
    }
}
```

### RAW File Processing

RAW files require special handling to extract usable preview data:

```go
func (ap *AssetProcessor) processRAWAsset(
    ctx context.Context,
    repository repo.Repository,
    asset *repo.Asset,
    fileReader io.Reader,
) error {
    // Read RAW data
    data, err := io.ReadAll(fileReader)
    if err != nil {
        return fmt.Errorf("failed to read RAW data: %w", err)
    }
    
    // Create RAW processor
    rawOpts := raw.DefaultProcessingOptions()
    rawOpts.PreferEmbedded = true  // Use embedded JPEG if available
    rawProcessor := raw.NewProcessor(rawOpts)
    
    // Process RAW file (timeout: 45 seconds)
    ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
    defer cancel()
    
    rawResult, err := rawProcessor.ProcessRAW(ctx, bytes.NewReader(data), asset.OriginalFilename)
    if err != nil {
        return fmt.Errorf("failed to process RAW: %w", err)
    }
    
    // Extract preview (embedded JPEG or rendered image)
    previewData := rawResult.PreviewData
    
    // Continue with standard processing using preview
    return ap.processStandardPhotoAsset(ctx, repository, asset, bytes.NewReader(previewData))
}
```

**RAW Processing Strategies**:
1. **Embedded Preview**: Extract embedded JPEG preview (fast, ~500ms)
2. **Full Render**: Render RAW data to JPEG (slow, ~5-15 seconds)

**Supported RAW Formats**:
- Canon: CR2, CR3
- Nikon: NEF, NRW
- Sony: ARW, SRF, SR2
- Fujifilm: RAF
- Olympus: ORF
- Pentax: PEF, DNG
- Adobe: DNG (universal)

### Standard Photo Processing

```go
func (ap *AssetProcessor) processStandardPhotoAsset(
    ctx context.Context,
    repository repo.Repository,
    asset *repo.Asset,
    fileReader io.Reader,
) error {
    // Use pipes for concurrent processing
    exifR, exifW := io.Pipe()
    clipR, clipW := io.Pipe()
    thumbR, thumbW := io.Pipe()
    
    // Create multi-writer to fan out data
    multiWriter := io.MultiWriter(exifW, clipW, thumbW)
    
    // Start goroutines
    g, gCtx := errgroup.WithContext(ctx)
    
    // Goroutine 1: Extract EXIF metadata
    g.Go(func() error {
        defer exifR.Close()
        return ap.extractAndSaveEXIF(gCtx, asset, exifR)
    })
    
    // Goroutine 2: Generate thumbnails
    g.Go(func() error {
        defer clipR.Close()
        return ap.generateThumbnails(gCtx, repository, asset, thumbR)
    })
    
    // Goroutine 3: Prepare for CLIP processing
    var clipData []byte
    g.Go(func() error {
        defer clipR.Close()
        var err error
        clipData, err = prepareImageForCLIP(clipR)
        return err
    })
    
    // Copy data to all pipes
    _, err := io.Copy(multiWriter, fileReader)
    exifW.Close()
    clipW.Close()
    thumbW.Close()
    
    if err != nil {
        return fmt.Errorf("failed to copy data: %w", err)
    }
    
    // Wait for all goroutines
    if err := g.Wait(); err != nil {
        return err
    }
    
    // Enqueue CLIP processing if enabled
    if ap.appConfig.CLIPEnabled && len(clipData) > 0 {
        return ap.enqueueCLIPProcessing(ctx, asset.AssetID, clipData)
    }
    
    return nil
}
```

### EXIF Extraction

**Location**: `server/internal/utils/exif/extractor.go`

```go
func (ap *AssetProcessor) extractAndSaveEXIF(
    ctx context.Context,
    asset *repo.Asset,
    reader io.Reader,
) error {
    // Extract EXIF data
    extractor := exif.NewExtractor(nil)
    exifData, err := extractor.Extract(reader)
    if err != nil {
        // Non-fatal: not all images have EXIF
        log.Printf("EXIF extraction failed: %v", err)
        return nil
    }
    
    // Update asset with EXIF metadata
    updates := map[string]interface{}{}
    
    if exifData.DateTime != nil {
        updates["taken_at"] = exifData.DateTime
    }
    
    if exifData.GPS != nil {
        updates["latitude"] = exifData.GPS.Latitude
        updates["longitude"] = exifData.GPS.Longitude
    }
    
    if exifData.Camera != nil {
        updates["camera_make"] = exifData.Camera.Make
        updates["camera_model"] = exifData.Camera.Model
    }
    
    if exifData.Lens != nil {
        updates["lens_model"] = exifData.Lens.Model
    }
    
    if exifData.Exposure != nil {
        updates["iso"] = exifData.Exposure.ISO
        updates["aperture"] = exifData.Exposure.Aperture
        updates["shutter_speed"] = exifData.Exposure.ShutterSpeed
        updates["focal_length"] = exifData.Exposure.FocalLength
    }
    
    // Save to database
    return ap.assetService.UpdateAssetMetadata(ctx, asset.AssetID, updates)
}
```

**Extracted EXIF Fields**:
- **Temporal**: Taken at, timezone
- **Location**: GPS coordinates, altitude
- **Camera**: Make, model, serial number
- **Lens**: Model, focal length
- **Exposure**: ISO, aperture (f-stop), shutter speed
- **Image**: Width, height, orientation, color space
- **Copyright**: Artist, copyright notice

### Thumbnail Generation

Multiple thumbnail sizes are generated for different use cases:

```go
var thumbnailSizes = map[string][2]int{
    "small":  {400, 400},    // Grid view
    "medium": {800, 800},    // Detail view
    "large":  {1920, 1920},  // Lightbox/fullscreen
}

func (ap *AssetProcessor) generateThumbnails(
    ctx context.Context,
    repository repo.Repository,
    asset *repo.Asset,
    reader io.Reader,
) error {
    // Read image data
    imageData, err := io.ReadAll(reader)
    if err != nil {
        return err
    }
    
    // Create bimg instance for fast image processing
    image := bimg.NewImage(imageData)
    
    g, gCtx := errgroup.WithContext(ctx)
    
    // Generate each thumbnail size concurrently
    for sizeName, dimensions := range thumbnailSizes {
        sizeName := sizeName
        width, height := dimensions[0], dimensions[1]
        
        g.Go(func() error {
            return ap.generateThumbnail(gCtx, repository, asset, image, sizeName, width, height)
        })
    }
    
    return g.Wait()
}

func (ap *AssetProcessor) generateThumbnail(
    ctx context.Context,
    repository repo.Repository,
    asset *repo.Asset,
    image *bimg.Image,
    sizeName string,
    maxWidth, maxHeight int,
) error {
    // Resize with aspect ratio preserved
    options := bimg.Options{
        Width:   maxWidth,
        Height:  maxHeight,
        Crop:    false,
        Enlarge: false,
        Quality: 85,
        Type:    bimg.JPEG,
    }
    
    thumbnailData, err := image.Process(options)
    if err != nil {
        return fmt.Errorf("resize failed: %w", err)
    }
    
    // Save thumbnail to storage
    thumbnailPath := filepath.Join(
        repository.Path,
        ".lumilio/assets/thumbnails",
        sizeName,
        fmt.Sprintf("%s.jpg", asset.AssetID.String()),
    )
    
    err = os.WriteFile(thumbnailPath, thumbnailData, 0644)
    if err != nil {
        return fmt.Errorf("save thumbnail: %w", err)
    }
    
    // Record in database
    return ap.assetService.CreateThumbnail(ctx, repo.Thumbnail{
        AssetID:    asset.AssetID,
        Size:       sizeName,
        FilePath:   thumbnailPath,
        FileSize:   int64(len(thumbnailData)),
        Width:      maxWidth,
        Height:     maxHeight,
        MimeType:   "image/jpeg",
        CreatedAt:  time.Now(),
    })
}
```

**Thumbnail Strategy**:
- Aspect ratio preserved (no cropping by default)
- JPEG format (lossy compression)
- 85% quality (good balance of size vs. quality)
- Progressive encoding for faster loading
- Generated concurrently for performance

### CLIP Processing

CLIP (Contrastive Language-Image Pre-training) enables semantic search:

```go
func (ap *AssetProcessor) enqueueCLIPProcessing(
    ctx context.Context,
    assetID pgtype.UUID,
    imageData []byte,
) error {
    // Enqueue CLIP processing job
    _, err := ap.queueClient.Insert(ctx,
        jobs.ProcessClipArgs{
            AssetID:   assetID,
            ImageData: imageData,
        },
        &river.InsertOpts{
            Queue: "process_clip",
        },
    )
    
    return err
}
```

The CLIP worker processes batches for efficiency:

```go
// Separate worker (clip_worker.go)
func (w *ProcessClipWorker) Work(ctx context.Context, job *river.Job[ProcessClipArgs]) error {
    // Submit to batch dispatcher
    result, err := w.Dispatcher.Submit(ctx, 
        job.Args.AssetID, 
        job.Args.ImageData,
        "image/webp",
    )
    if err != nil {
        return err
    }
    
    // Save embedding vector (pgvector)
    err = w.AssetService.SaveNewEmbedding(ctx, 
        job.Args.AssetID, 
        result.Embedding,
    )
    
    // Save classification labels
    err = w.AssetService.SaveNewSpeciesPredictions(ctx,
        job.Args.AssetID,
        result.Labels,
    )
    
    return err
}
```

**CLIP Features**:
- **Embedding Vector**: 512-dimensional vector for semantic similarity
- **Smart Classification**: Species, objects, scenes (top-3 predictions)
- **Batch Processing**: 8 images per batch for GPU efficiency
- **Async Processing**: Doesn't block upload pipeline

## Video Processing Pipeline

### Overview

Video processing focuses on:
1. Metadata extraction (codec, resolution, duration)
2. Smart transcoding (only if needed)
3. Thumbnail extraction from frames
4. Preview clip generation

### Main Entry Point

**Location**: `server/internal/processors/video_processor.go`

```go
func (ap *AssetProcessor) processVideoAsset(
    ctx context.Context,
    repository repo.Repository,
    asset *repo.Asset,
    fileReader io.Reader,
) error {
    // Videos can take long time to process
    ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
    defer cancel()
    
    // Create temp file (ffmpeg requires file path)
    tempFile, err := os.CreateTemp("", "video_*.tmp")
    if err != nil {
        return err
    }
    defer os.Remove(tempFile.Name())
    
    // Copy to temp file
    io.Copy(tempFile, fileReader)
    tempFile.Close()
    
    // Get video info
    videoInfo, err := ap.getVideoInfo(tempFile.Name())
    if err != nil {
        return err
    }
    
    g, gCtx := errgroup.WithContext(ctx)
    
    // Parallel processing
    g.Go(func() error {
        return ap.extractVideoMetadata(gCtx, asset, tempFile.Name(), videoInfo)
    })
    
    g.Go(func() error {
        return ap.transcodeVideoSmart(gCtx, asset, tempFile.Name(), videoInfo)
    })
    
    g.Go(func() error {
        return ap.generateVideoThumbnail(gCtx, asset, tempFile.Name())
    })
    
    return g.Wait()
}
```

### Video Info Extraction

Using ffprobe to get video metadata:

```go
func (ap *AssetProcessor) getVideoInfo(videoPath string) (*VideoInfo, error) {
    // Run ffprobe
    cmd := exec.Command("ffprobe",
        "-v", "error",
        "-select_streams", "v:0",
        "-show_entries", "stream=width,height,duration,codec_name",
        "-show_entries", "format=format_name",
        "-of", "json",
        videoPath,
    )
    
    output, err := cmd.Output()
    if err != nil {
        return nil, err
    }
    
    // Parse JSON output
    var result struct {
        Streams []struct {
            Width     int     `json:"width"`
            Height    int     `json:"height"`
            Duration  string  `json:"duration"`
            CodecName string  `json:"codec_name"`
        } `json:"streams"`
        Format struct {
            FormatName string `json:"format_name"`
        } `json:"format"`
    }
    
    json.Unmarshal(output, &result)
    
    duration, _ := strconv.ParseFloat(result.Streams[0].Duration, 64)
    
    return &VideoInfo{
        Width:    result.Streams[0].Width,
        Height:   result.Streams[0].Height,
        Duration: duration,
        Codec:    result.Streams[0].CodecName,
        Format:   result.Format.FormatName,
    }, nil
}
```

### Smart Transcoding

Only transcode if necessary (saves CPU/storage):

```go
func (ap *AssetProcessor) transcodeVideoSmart(
    ctx context.Context,
    asset *repo.Asset,
    videoPath string,
    info *VideoInfo,
) error {
    // Check if transcoding needed
    needsTranscode := false
    
    // Criteria 1: Codec not web-friendly
    if info.Codec != "h264" && info.Codec != "vp9" {
        needsTranscode = true
    }
    
    // Criteria 2: Resolution too high (> 1080p)
    if info.Height > 1080 {
        needsTranscode = true
    }
    
    // Criteria 3: Format not streamable
    if info.Format != "mp4" && info.Format != "webm" {
        needsTranscode = true
    }
    
    if !needsTranscode {
        log.Printf("Video already web-optimized, skipping transcode")
        return nil
    }
    
    // Perform transcoding
    outputPath := filepath.Join(
        asset.RepositoryPath,
        ".lumilio/assets/videos/web",
        fmt.Sprintf("%s.mp4", asset.AssetID),
    )
    
    // Target: H.264, AAC audio, MP4 container, ≤1080p
    cmd := exec.CommandContext(ctx, "ffmpeg",
        "-i", videoPath,
        "-c:v", "libx264",           // Video codec
        "-preset", "medium",          // Encoding speed/quality
        "-crf", "23",                 // Quality (0-51, lower = better)
        "-maxrate", "5M",             // Max bitrate
        "-bufsize", "10M",            // Buffer size
        "-vf", "scale=-2:min(ih\\,1080)", // Scale to max 1080p
        "-c:a", "aac",                // Audio codec
        "-b:a", "128k",               // Audio bitrate
        "-movflags", "+faststart",    // Enable streaming
        "-y",                         // Overwrite
        outputPath,
    )
    
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("transcoding failed: %w", err)
    }
    
    // Update database with transcoded path
    return ap.assetService.UpdateAsset(ctx, asset.AssetID, map[string]interface{}{
        "transcoded_path": outputPath,
        "is_transcoded":   true,
    })
}
```

**Transcoding Parameters**:
- **Codec**: H.264 (libx264) for broad compatibility
- **Quality**: CRF 23 (visually lossless)
- **Resolution**: Max 1080p (maintains aspect ratio)
- **Audio**: AAC 128kbps
- **Container**: MP4 with faststart flag (enables streaming)
- **Bitrate**: Max 5 Mbps (good quality, reasonable size)

### Thumbnail Extraction

Extract frame from video for thumbnail:

```go
func (ap *AssetProcessor) generateVideoThumbnail(
    ctx context.Context,
    asset *repo.Asset,
    videoPath string,
) error {
    outputPath := filepath.Join(
        asset.RepositoryPath,
        ".lumilio/assets/thumbnails/medium",
        fmt.Sprintf("%s.jpg", asset.AssetID),
    )
    
    // Extract frame at 1 second (or 10% into video)
    cmd := exec.CommandContext(ctx, "ffmpeg",
        "-ss", "00:00:01",           // Seek to 1 second
        "-i", videoPath,
        "-vframes", "1",             // Extract 1 frame
        "-vf", "scale=800:-1",       // Resize to 800px width
        "-q:v", "2",                 // Quality (1-31, lower = better)
        "-y",
        outputPath,
    )
    
    if err := cmd.Run(); err != nil {
        return fmt.Errorf("thumbnail extraction failed: %w", err)
    }
    
    // Record in database
    stat, _ := os.Stat(outputPath)
    return ap.assetService.CreateThumbnail(ctx, repo.Thumbnail{
        AssetID:   asset.AssetID,
        Size:      "medium",
        FilePath:  outputPath,
        FileSize:  stat.Size(),
        Width:     800,
        MimeType:  "image/jpeg",
        CreatedAt: time.Now(),
    })
}
```

## Audio Processing Pipeline

### Overview

Audio processing is simpler than video:
1. Metadata extraction (artist, album, duration)
2. Waveform generation
3. Optional transcoding to MP3

### Main Entry Point

**Location**: `server/internal/processors/audio_processor.go`

```go
func (ap *AssetProcessor) processAudioAsset(
    ctx context.Context,
    repository repo.Repository,
    asset *repo.Asset,
    fileReader io.Reader,
) error {
    // Create temp file
    tempFile, err := os.CreateTemp("", "audio_*.tmp")
    if err != nil {
        return err
    }
    defer os.Remove(tempFile.Name())
    
    io.Copy(tempFile, fileReader)
    tempFile.Close()
    
    g, gCtx := errgroup.WithContext(ctx)
    
    // Extract metadata
    g.Go(func() error {
        return ap.extractAudioMetadata(gCtx, asset, tempFile.Name())
    })
    
    // Generate waveform
    g.Go(func() error {
        return ap.generateWaveform(gCtx, asset, tempFile.Name())
    })
    
    // Transcode if needed
    g.Go(func() error {
        return ap.transcodeAudioIfNeeded(gCtx, asset, tempFile.Name())
    })
    
    return g.Wait()
}
```

### Metadata Extraction

```go
func (ap *AssetProcessor) extractAudioMetadata(
    ctx context.Context,
    asset *repo.Asset,
    audioPath string,
) error {
    // Use ffprobe for metadata
    cmd := exec.CommandContext(ctx, "ffprobe",
        "-v", "error",
        "-show_entries", "format_tags=artist,album,title,date,genre",
        "-show_entries", "format=duration",
        "-of", "json",
        audioPath,
    )
    
    output, err := cmd.Output()
    if err != nil {
        return err
    }
    
    // Parse and update database
    // ... (similar to video metadata)
}
```

### Waveform Generation

Visual representation for audio scrubbing:

```go
func (ap *AssetProcessor) generateWaveform(
    ctx context.Context,
    asset *repo.Asset,
    audioPath string,
) error {
    outputPath := filepath.Join(
        asset.RepositoryPath,
        ".lumilio/assets/audios/waveforms",
        fmt.Sprintf("%s.json", asset.AssetID),
    )
    
    // Extract audio samples
    cmd := exec.CommandContext(ctx, "ffmpeg",
        "-i", audioPath,
        "-ac", "1",                  // Mono
        "-ar", "8000",               // 8kHz sample rate
        "-f", "f32le",               // 32-bit float PCM
        "-",
    )
    
    samples, err := cmd.Output()
    if err != nil {
        return err
    }
    
    // Downsample to ~1000 points
    waveform := downsampleWaveform(samples, 1000)
    
    // Save as JSON
    data, _ := json.Marshal(waveform)
    return os.WriteFile(outputPath, data, 0644)
}
```

## Performance Optimization

### Concurrent Processing

All processors use goroutines for parallel operations:

```go
g, gCtx := errgroup.WithContext(ctx)

g.Go(func() error { return task1() })
g.Go(func() error { return task2() })
g.Go(func() error { return task3() })

return g.Wait()  // Wait for all, fail if any fails
```

**Benefits**:
- Photo processing: 3x faster (EXIF + thumbnails + CLIP in parallel)
- Video processing: 2x faster (metadata + transcode + thumbnail in parallel)

### Streaming with io.Pipe

Avoid buffering entire files in memory:

```go
exifR, exifW := io.Pipe()
thumbR, thumbW := io.Pipe()

// Fan out data
multiWriter := io.MultiWriter(exifW, thumbW)
io.Copy(multiWriter, fileReader)

// Concurrent consumers
go processEXIF(exifR)
go generateThumbnails(thumbR)
```

### Image Processing Library

**bimg** (libvips binding) for fast thumbnails:
- 5-10x faster than Go's image package
- Lower memory usage
- Better quality resizing

### Resource Limits

Timeouts prevent runaway processes:

```go
// Photo: 30 seconds
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)

// RAW: 45 seconds (more complex)
ctx, cancel := context.WithTimeout(ctx, 45*time.Second)

// Video: 30 minutes (transcoding is slow)
ctx, cancel := context.WithTimeout(ctx, 30*time.Minute)
```

## Error Handling

### Graceful Degradation

Non-fatal errors don't fail the entire pipeline:

```go
// EXIF extraction fails - continue without metadata
if err := extractEXIF(asset); err != nil {
    log.Printf("EXIF extraction failed: %v", err)
    // Don't return error - continue processing
}

// Thumbnail generation fails - continue without thumbnails
if err := generateThumbnails(asset); err != nil {
    log.Printf("Thumbnail generation failed: %v", err)
}
```

### Fatal Errors

Some errors must fail the pipeline:
- File not found
- Database write failure
- Storage full
- Invalid file format

### Retry Logic

River queue handles retries automatically:
- Max attempts: 5
- Backoff: Exponential (1s, 2s, 4s, 8s, 16s)
- Permanent failure after max attempts

## Monitoring and Metrics

### Processing Statistics

```go
type ProcessingStats struct {
    TotalAssets     int64
    PhotosProcessed int64
    VideosProcessed int64
    AudiosProcessed int64
    
    AvgPhotoTime    time.Duration
    AvgVideoTime    time.Duration
    AvgAudioTime    time.Duration
    
    ThumbnailsGenerated int64
    TranscodesPerformed int64
    EXIFExtracted       int64
    CLIPEmbeddings      int64
    
    Failures            int64
    FailureRate         float64
}
```

### Performance Tracking

```go
func (ap *AssetProcessor) ProcessAsset(ctx context.Context, task AssetPayload) (*repo.Asset, error) {
    startTime := time.Now()
    defer func() {
        duration := time.Since(startTime)
        log.Printf("Asset processing completed in %v", duration)
        
        // Record metrics
        ap.metrics.RecordProcessingTime(task.ContentType, duration)
    }()
    
    // ... processing logic
}
```

## Configuration

### Environment Variables

```bash
# Processing timeouts
PHOTO_PROCESSING_TIMEOUT=30s
RAW_PROCESSING_TIMEOUT=45s
VIDEO_PROCESSING_TIMEOUT=30m

# Thumbnail settings
THUMBNAIL_QUALITY=85
THUMBNAIL_SIZES=400,800,1920

# Video transcoding
VIDEO_MAX_RESOLUTION=1080
VIDEO_TARGET_BITRATE=5M
VIDEO_CRF=23

# CLIP settings
CLIP_ENABLED=true
CLIP_BATCH_SIZE=8
CLIP_TIMEOUT=10s
```

### Repository Configuration

```yaml
processing:
  generate_thumbnails: true
  thumbnail_quality: 85
  
  video_transcode: "smart"  # "always", "never", "smart"
  video_max_resolution: 1080
  
  audio_generate_waveforms: true
  
  extract_metadata: true
  calculate_embeddings: true
```

## Testing

### Unit Tests

```go
func TestPhotoProcessor_Thumbnails(t *testing.T) {
    processor := setupTestProcessor(t)
    
    // Create test image
    testImage := createTestImage(t, 2000, 1500)
    
    // Process
    asset := createTestAsset(t)
    err := processor.generateThumbnails(context.Background(), asset, testImage)
    require.NoError(t, err)
    
    // Verify thumbnails exist
    for size := range thumbnailSizes {
        path := getThumbnailPath(asset, size)
        assert.FileExists(t, path)
    }
}
```

### Integration Tests

```go
func TestEndToEndPhotoProcessing(t *testing.T) {
    // Setup server
    server := setupTestServer(t)
    defer server.Close()
    
    // Upload photo
    resp := uploadFile(t, server.URL, "test.jpg")
    
    // Wait for processing
    waitForJobComplete(t, resp.TaskID)
    
    // Verify asset in database
    asset := getAsset(t, resp.AssetID)
    assert.NotNil(t, asset)
    
    // Verify thumbnails
    thumbnails := getThumbnails(t, resp.AssetID)
    assert.Len(t, thumbnails, 3)
    
    // Verify EXIF
    assert.NotNil(t, asset.TakenAt)
    assert.NotNil(t, asset.CameraMake)
    
    // Verify CLIP embedding
    embedding := getEmbedding(t, resp.AssetID)
    assert.NotNil(t, embedding)
    assert.Len(t, embedding.Vector, 512)
}
```

## Future Improvements

1. **GPU Acceleration**: Use GPU for thumbnail generation and transcoding
2. **Adaptive Bitrate**: Generate multiple video qualities
3. **Smart Cropping**: AI-powered thumbnail cropping
4. **Motion Photos**: Extract video from Live Photos
5. **HDR Support**: Tone-map HDR images for web display
6. **Facial Recognition**: Detect and tag faces
7. **Scene Detection**: Auto-tag photos by scene
8. **Duplicate Detection**: Perceptual hashing for near-duplicates

## Related Documentation

- [Asset Processor README](../docs/en/developer-documentation/backend/processors/asset-processor.md)
- [Photo Processor README](../docs/en/developer-documentation/backend/processors/photo-processor.md)
- [Queue System README](../server/internal/queue/README.md)
- [Inbox Upload Process](./01-inbox-upload.md)

---

*This document is part of the Lumilio Photos server wrap-up documentation.*
