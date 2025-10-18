package processors

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/utils/errgroup"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	"strconv"
	"strings"
	"time"
)

// VideoInfo holds video metadata
type VideoInfo struct {
	Width    int
	Height   int
	Duration float64
	Codec    string
	Format   string
}

func (ap *AssetProcessor) processVideoAsset(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	fileReader io.Reader,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Minute) // Videos take longer
	defer cancel()

	// For large files, use streaming approach to avoid full buffering
	// Create temporary file only if needed for metadata extraction
	tempFile, err := os.CreateTemp("", "video_processing_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy input to temp file for metadata extraction
	if _, err := io.Copy(tempFile, fileReader); err != nil {
		return fmt.Errorf("copy to temp file: %w", err)
	}
	tempFile.Close()

	// Get video info first
	videoInfo, err := ap.getVideoInfo(tempFile.Name())
	if err != nil {
		return fmt.Errorf("get video info: %w", err)
	}

	g := errgroup.NewFaultTolerant()

	// Goroutine 1: Extract metadata
	g.Go(func() error {
		return ap.extractVideoMetadata(timeoutCtx, asset, tempFile.Name(), videoInfo)
	})

	// Goroutine 2: Transcode video (smart strategy)
	g.Go(func() error {
		return ap.transcodeVideoSmart(timeoutCtx, repository.Path, asset, tempFile.Name(), videoInfo)
	})

	// Goroutine 3: Generate thumbnail
	g.Go(func() error {
		return ap.generateVideoThumbnail(timeoutCtx, repository.Path, asset, tempFile.Name(), videoInfo)
	})

	// Wait for all tasks to complete, but don't fail the entire process if some tasks fail
	errors := g.Wait()
	if len(errors) > 0 {
		// Log individual errors but don't fail the entire process
		for _, err := range errors {
			fmt.Printf("Video processing partial failure: %v\n", err)
		}
		// Return success even if some tasks failed, as partial processing is acceptable
	}

	return nil
}

func (ap *AssetProcessor) extractVideoMetadata(ctx context.Context, asset *repo.Asset, videoPath string, videoInfo *VideoInfo) error {
	// Use existing exif extractor for video metadata
	file, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("open video file: %w", err)
	}
	defer file.Close()

	// Configure extractor with optimized settings for videos, including fast mode
	config := &exif.Config{
		MaxFileSize: 20 * 1024 * 1024 * 1024, // 20GB
		Timeout:     60 * time.Second,
		BufferSize:  128 * 1024,
		FastMode:    true, // Use fast mode for videos to avoid full file scan
	}
	extractor := exif.NewExtractor(config)
	defer extractor.Close()

	req := &exif.StreamingExtractRequest{
		Reader:    file,
		AssetType: dbtypes.AssetTypeVideo,
		Filename:  asset.OriginalFilename,
		Size:      asset.FileSize,
	}

	result, err := extractor.ExtractFromStream(ctx, req)
	if err != nil {
		return fmt.Errorf("extract metadata: %w", err)
	}

	if meta, ok := result.Metadata.(*dbtypes.VideoSpecificMetadata); ok {
		// Add duration info to asset record
		if err := ap.assetService.UpdateAssetDuration(ctx, asset.AssetID.Bytes, videoInfo.Duration); err != nil {
			return fmt.Errorf("update duration: %w", err)
		}

		// Add dimensions to asset record
		if err := ap.assetService.UpdateAssetDimensions(ctx, asset.AssetID.Bytes, int32(videoInfo.Width), int32(videoInfo.Height)); err != nil {
			return fmt.Errorf("update dimensions: %w", err)
		}

		sm, err := dbtypes.MarshalMeta(meta)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}

		if err := ap.assetService.UpdateAssetMetadata(ctx, asset.AssetID.Bytes, sm); err != nil {
			return fmt.Errorf("save metadata: %w", err)
		}
	}

	return nil
}

func (ap *AssetProcessor) transcodeVideoSmart(ctx context.Context, repoPath string, asset *repo.Asset, videoPath string, videoInfo *VideoInfo) error {
	maxHeight := 1080

	// Smart transcoding strategy
	if videoInfo.Height <= maxHeight && strings.ToLower(videoInfo.Format) == "mp4" && strings.Contains(strings.ToLower(videoInfo.Codec), "h264") {
		// Video is already in good format and resolution, just copy to storage
		return ap.copyVideoAsWebVersion(ctx, repoPath, asset, videoPath, "web")
	}

	if videoInfo.Height <= maxHeight {
		// Video resolution is good, just transcode format/codec
		outputPath, err := ap.transcodeVideoToMP4(ctx, videoPath, videoInfo.Width, videoInfo.Height)
		if err != nil {
			return fmt.Errorf("transcode to mp4: %w", err)
		}
		defer os.Remove(outputPath)

		return ap.saveTranscodedVideo(ctx, repoPath, asset, outputPath, "web")
	} else {
		// Video needs downscaling + transcoding
		// Calculate new dimensions maintaining aspect ratio
		aspectRatio := float64(videoInfo.Width) / float64(videoInfo.Height)
		newWidth := int(float64(maxHeight) * aspectRatio)
		// Ensure even dimensions for H.264
		if newWidth%2 != 0 {
			newWidth--
		}

		// Generate 1080p version
		outputPath1080p, err := ap.transcodeVideoToMP4(ctx, videoPath, newWidth, maxHeight)
		if err != nil {
			return fmt.Errorf("transcode to 1080p: %w", err)
		}
		defer os.Remove(outputPath1080p)

		if err := ap.saveTranscodedVideo(ctx, repoPath, asset, outputPath1080p, "web"); err != nil {
			return fmt.Errorf("save 1080p version: %w", err)
		}

		// Do not save original copy; only the web (downscaled/transcoded) version is kept
		return nil
	}
}

func (ap *AssetProcessor) transcodeVideoToMP4(ctx context.Context, inputPath string, width, height int) (string, error) {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("transcoded_%d_%s.mp4", height, filepath.Base(inputPath)))

	// Optimize ffmpeg settings for large files
	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-c:v", "libx264", // H.264 codec
		"-preset", "medium", // Balance speed/compression
		"-crf", "23", // Good quality constant rate factor
		"-maxrate", "5000k", // Max bitrate for 1080p
		"-bufsize", "10000k", // Buffer size
		"-vf", fmt.Sprintf("scale=%d:%d", width, height), // Scale video
		"-c:a", "aac", // AAC audio
		"-b:a", "128k", // Audio bitrate
		"-movflags", "+faststart", // Enable web streaming
		"-avoid_negative_ts", "make_zero", // Handle timestamp issues
		"-threads", "0", // Use all available CPU threads
		"-f", "mp4",
		"-y", // Overwrite output file
		outputPath,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg transcode failed: %w", err)
	}

	return outputPath, nil
}

func (ap *AssetProcessor) copyVideoAsWebVersion(ctx context.Context, repoPath string, asset *repo.Asset, videoPath, version string) error {
	videoFile, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("open video file: %w", err)
	}
	defer videoFile.Close()

	return ap.assetService.SaveVideoVersion(ctx, repoPath, videoFile, asset, version)
}

func (ap *AssetProcessor) saveTranscodedVideo(ctx context.Context, repoPath string, asset *repo.Asset, outputPath, version string) error {
	transcodedFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open transcoded file: %w", err)
	}
	defer transcodedFile.Close()

	return ap.assetService.SaveVideoVersion(ctx, repoPath, transcodedFile, asset, version)
}

func (ap *AssetProcessor) generateVideoThumbnail(ctx context.Context, repoPath string, asset *repo.Asset, videoPath string, info *VideoInfo) error {
	// Generate thumbnail at 1 second mark (or 10% of duration, whichever is smaller)
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("thumb_%s.jpg", asset.AssetID))
	defer os.Remove(outputPath)

	// Try to get video duration for better thumbnail timing
	duration := info.Duration
	thumbnailTime := "00:00:01"
	if duration > 0 && duration < 10 {
		// For short videos, take thumbnail at 10% of duration
		thumbnailSeconds := duration * 0.1
		thumbnailTime = fmt.Sprintf("00:00:%02d", int(thumbnailSeconds))
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", thumbnailTime, // seek before input
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease",
		"-threads", "1",
		"-f", "mjpeg",
		"-y",
		outputPath,
	)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("generate thumbnail: %w\nstderr: %s", err, stderr.String())
	}

	// Generate multiple thumbnail sizes using existing imaging utils
	thumbnailFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open thumbnail: %w", err)
	}
	defer thumbnailFile.Close()

	// Use existing thumbnail sizes from photo processor
	outputs := make(map[string]io.Writer, len(thumbnailSizes))
	buffers := make(map[string]*bytes.Buffer, len(thumbnailSizes))
	for name := range thumbnailSizes {
		buf := &bytes.Buffer{}
		buffers[name] = buf
		outputs[name] = buf
	}

	if err := imaging.StreamThumbnails(thumbnailFile, thumbnailSizes, outputs); err != nil {
		return fmt.Errorf("generate thumbnails: %w", err)
	}

	for name, buf := range buffers {
		if buf.Len() == 0 {
			continue
		}
		if err := ap.assetService.SaveNewThumbnail(ctx, repoPath, buf, asset, name); err != nil {
			return fmt.Errorf("save thumbnail %s: %w", name, err)
		}
	}

	return nil
}

func (ap *AssetProcessor) getVideoInfo(videoPath string) (*VideoInfo, error) {
	// Get video information using ffprobe with JSON output
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "v:0", // First video stream
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	// Parse JSON output
	var probeData struct {
		Streams []struct {
			Width     int    `json:"width"`
			Height    int    `json:"height"`
			CodecName string `json:"codec_name"`
			Duration  string `json:"duration"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probeData); err != nil {
		return nil, fmt.Errorf("parse ffprobe json: %w", err)
	}

	info := &VideoInfo{}

	// Extract video stream info
	if len(probeData.Streams) > 0 {
		stream := probeData.Streams[0]
		info.Width = stream.Width
		info.Height = stream.Height
		info.Codec = stream.CodecName

		// Try to get duration from stream first
		if stream.Duration != "" {
			if duration, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
				info.Duration = duration
			}
		}
	}

	// Extract format info
	info.Format = probeData.Format.FormatName

	// If duration not set from stream, try format
	if info.Duration == 0 && probeData.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			info.Duration = duration
		}
	}

	return info, nil
}
