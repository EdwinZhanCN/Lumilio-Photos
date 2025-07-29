# EXIF Metadata Extractor

A high-performance, streaming EXIF metadata extractor for photos, videos, and audio files using Go's concurrency features and the exiftool command line utility.

## Features

- 🚀 **Streaming processing** with `io.Reader` support
- ⚡ **Concurrent extraction** with worker pools
- 📱 **Multi-format support** (photos, videos, audio)
- 🛡️ **Robust error handling** and timeouts
- 💾 **Memory efficient** with buffered I/O
- 🔧 **Configurable** extraction parameters

## Prerequisites

Install exiftool on your system:

```bash
# macOS
brew install exiftool

# Ubuntu/Debian
sudo apt-get install exiftool

# Windows (download from https://exiftool.org/)
```

## Quick Start

### Basic Usage

```go
package main

import (
    "context"
    "fmt"
    "os"
    "server/internal/models"
    "server/internal/utils/exif"
)

func main() {
    // Check if exiftool is available
    if !exif.IsExifToolAvailable() {
        panic("exiftool not found")
    }

    // Create extractor with default config
    extractor := exif.NewExtractor(nil)
    defer extractor.Close()

    // Open a photo file
    file, err := os.Open("photo.jpg")
    if err != nil {
        panic(err)
    }
    defer file.Close()

    // Get file info
    info, _ := file.Stat()

    // Create extraction request
    req := &exif.StreamingExtractRequest{
        Reader:    file,
        AssetType: models.AssetTypePhoto,
        Filename:  "photo.jpg",
        Size:      info.Size(),
    }

    // Extract metadata
    result, err := extractor.ExtractFromStream(context.Background(), req)
    if err != nil {
        panic(err)
    }

    // Cast to photo metadata
    if photoMeta, ok := result.Metadata.(*models.PhotoSpecificMetadata); ok {
        fmt.Printf("Camera: %s\n", photoMeta.CameraModel)
        fmt.Printf("ISO: %d\n", photoMeta.IsoSpeed)
        if photoMeta.TakenTime != nil {
            fmt.Printf("Taken: %s\n", photoMeta.TakenTime.Format("2006-01-02 15:04:05"))
        }
    }
}
```

### Batch Processing

```go
func processBatch() {
    extractor := exif.NewExtractor(nil)
    defer extractor.Close()

    // Prepare multiple requests
    var requests []*exif.StreamingExtractRequest

    files := []string{"photo1.jpg", "video1.mp4", "audio1.mp3"}
    for _, filename := range files {
        file, _ := os.Open(filename)
        info, _ := file.Stat()
        
        // Determine asset type from extension
        var assetType models.AssetType
        switch {
        case strings.HasSuffix(filename, ".jpg"), strings.HasSuffix(filename, ".jpeg"):
            assetType = models.AssetTypePhoto
        case strings.HasSuffix(filename, ".mp4"), strings.HasSuffix(filename, ".mov"):
            assetType = models.AssetTypeVideo
        case strings.HasSuffix(filename, ".mp3"), strings.HasSuffix(filename, ".wav"):
            assetType = models.AssetTypeAudio
        }

        requests = append(requests, &exif.StreamingExtractRequest{
            Reader:    file,
            AssetType: assetType,
            Filename:  filename,
            Size:      info.Size(),
        })
    }

    // Process all files concurrently
    results, err := extractor.ExtractBatch(context.Background(), requests)
    if err != nil {
        fmt.Printf("Batch processing error: %v\n", err)
    }

    // Process results
    for i, result := range results {
        if result.Error != nil {
            fmt.Printf("File %d error: %v\n", i, result.Error)
            continue
        }

        switch result.Type {
        case models.AssetTypePhoto:
            photoMeta := result.Metadata.(*models.PhotoSpecificMetadata)
            fmt.Printf("Photo: %s, ISO: %d\n", photoMeta.CameraModel, photoMeta.IsoSpeed)
        case models.AssetTypeVideo:
            videoMeta := result.Metadata.(*models.VideoSpecificMetadata)
            fmt.Printf("Video: %s, FPS: %.2f\n", videoMeta.Codec, videoMeta.FrameRate)
        case models.AssetTypeAudio:
            audioMeta := result.Metadata.(*models.AudioSpecificMetadata)
            fmt.Printf("Audio: %s - %s\n", audioMeta.Artist, audioMeta.Title)
        }
    }
}
```

### Custom Configuration

```go
func customConfig() {
    // Create custom configuration
    config := &exif.Config{
        Timeout:       10 * time.Second,
        BufferSize:    16384,           // 16KB buffer
        MaxFileSize:   50 * 1024 * 1024, // 50MB max
        WorkerCount:   2,               // 2 concurrent workers
        RetryAttempts: 1,
        EnableCaching: true,
        CacheSize:     500,
    }

    extractor := exif.NewExtractor(config)
    defer extractor.Close()

    // Use extractor with custom config...
}
```

### Working with HTTP Requests

```go
func handleUpload(w http.ResponseWriter, r *http.Request) {
    // Parse multipart form
    r.ParseMultipartForm(32 << 20) // 32MB
    file, header, err := r.FormFile("image")
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }
    defer file.Close()

    // Extract metadata from uploaded file
    extractor := exif.NewExtractor(nil)
    defer extractor.Close()

    req := &exif.StreamingExtractRequest{
        Reader:    file,
        AssetType: models.AssetTypePhoto, // or detect from header.Filename
        Filename:  header.Filename,
        Size:      header.Size,
    }

    ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
    defer cancel()

    result, err := extractor.ExtractFromStream(ctx, req)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    // Return metadata as JSON
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(result.Metadata)
}
```

## Configuration Options

| Option | Default | Description |
|--------|---------|-------------|
| `Timeout` | 30s | Command execution timeout |
| `BufferSize` | 8KB | I/O buffer size |
| `MaxFileSize` | 100MB | Maximum file size |
| `WorkerCount` | 4 | Concurrent workers |
| `RetryAttempts` | 3 | Retry failed operations |
| `EnableCaching` | true | Enable metadata caching |
| `CacheSize` | 1000 | Maximum cache entries |

## Supported Formats

### Photos
- JPEG/JPG
- TIFF
- RAW formats (CR2, NEF, ARW, etc.)
- PNG (limited metadata)

### Videos
- MP4
- MOV
- AVI
- MKV
- And more...

### Audio
- MP3
- FLAC
- WAV
- AAC
- OGG

## Error Handling

```go
result, err := extractor.ExtractFromStream(ctx, req)
if err != nil {
    switch {
    case strings.Contains(err.Error(), "timeout"):
        // Handle timeout
    case strings.Contains(err.Error(), "file size"):
        // Handle file too large
    case strings.Contains(err.Error(), "exiftool"):
        // Handle exiftool errors
    default:
        // Handle other errors
    }
}

// Check for extraction errors
if result.Error != nil {
    // Handle metadata extraction specific errors
}
```

## Performance Tips

1. **Use batch processing** for multiple files
2. **Adjust worker count** based on system resources
3. **Configure appropriate timeouts** for your use case
4. **Enable caching** for repeated extractions
5. **Use appropriate buffer sizes** for your file sizes

## Testing ExifTool Installation

```go
// Check if exiftool is available
if !exif.IsExifToolAvailable() {
    log.Fatal("exiftool not found in PATH")
}

// Get version information
version, err := exif.GetExifToolVersion()
if err != nil {
    log.Fatal("Failed to get exiftool version:", err)
}
fmt.Printf("ExifTool version: %s\n", strings.TrimSpace(version))

// Validate installation
if err := exif.ValidateExifToolInstallation(); err != nil {
    log.Fatal("ExifTool validation failed:", err)
}
```

## License

This EXIF extractor is part of the Lumilio project.