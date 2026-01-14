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
	"strconv"
	"strings"
	"time"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
)

// VideoInfo holds video metadata.
type VideoInfo struct {
	Width    int
	Height   int
	Duration float64
	Codec    string
	Format   string
}

// extractVideoMetadata updates the asset with ffprobe/EXIF-derived metadata.
func (ap *AssetProcessor) extractVideoMetadata(ctx context.Context, asset *repo.Asset, videoPath string, videoInfo *VideoInfo) error {
	file, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("open video file: %w", err)
	}
	defer file.Close()

	config := &exif.Config{
		MaxFileSize: 20 * 1024 * 1024 * 1024, // 20GB
		Timeout:     60 * time.Second,        // 60s
		BufferSize:  128 * 1024,
		FastMode:    true,
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
		if err := ap.assetService.UpdateAssetDuration(ctx, asset.AssetID.Bytes, videoInfo.Duration); err != nil {
			return fmt.Errorf("update duration: %w", err)
		}
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

// transcodeVideoSmart applies a best-effort, resource-aware transcoding strategy.
func (ap *AssetProcessor) transcodeVideoSmart(ctx context.Context, repoPath string, asset *repo.Asset, videoPath string, videoInfo *VideoInfo) error {
	maxHeight := 1080

	if videoInfo.Height <= maxHeight && strings.ToLower(videoInfo.Format) == "mp4" && strings.Contains(strings.ToLower(videoInfo.Codec), "h264") {
		return ap.copyVideoAsWebVersion(ctx, repoPath, asset, videoPath, "web")
	}

	if videoInfo.Height <= maxHeight {
		outputPath, err := ap.transcodeVideoToMP4(ctx, videoPath, videoInfo.Width, videoInfo.Height)
		if err != nil {
			return fmt.Errorf("transcode to mp4: %w", err)
		}
		defer os.Remove(outputPath)

		return ap.saveTranscodedVideo(ctx, repoPath, asset, outputPath, "web")
	}

	aspectRatio := float64(videoInfo.Width) / float64(videoInfo.Height)
	newWidth := int(float64(maxHeight) * aspectRatio)
	if newWidth%2 != 0 {
		newWidth--
	}

	outputPath1080p, err := ap.transcodeVideoToMP4(ctx, videoPath, newWidth, maxHeight)
	if err != nil {
		return fmt.Errorf("transcode to 1080p: %w", err)
	}
	defer os.Remove(outputPath1080p)

	if err := ap.saveTranscodedVideo(ctx, repoPath, asset, outputPath1080p, "web"); err != nil {
		return fmt.Errorf("save 1080p version: %w", err)
	}

	return nil
}

// transcodeVideoToMP4 runs ffmpeg to produce an H.264/AAC MP4 at the target size.
func (ap *AssetProcessor) transcodeVideoToMP4(ctx context.Context, inputPath string, width, height int) (string, error) {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("transcoded_%d_%s.mp4", height, filepath.Base(inputPath)))

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-maxrate", "5000k",
		"-bufsize", "10000k",
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-avoid_negative_ts", "make_zero",
		"-threads", "0",
		"-f", "mp4",
		"-y",
		outputPath,
	)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg transcode failed: %w", err)
	}

	return outputPath, nil
}

// copyVideoAsWebVersion saves the provided video file as the web version.
func (ap *AssetProcessor) copyVideoAsWebVersion(ctx context.Context, repoPath string, asset *repo.Asset, videoPath, version string) error {
	videoFile, err := os.Open(videoPath)
	if err != nil {
		return fmt.Errorf("open video file: %w", err)
	}
	defer videoFile.Close()

	return ap.assetService.SaveVideoVersion(ctx, repoPath, videoFile, asset, version)
}

// saveTranscodedVideo saves a transcoded output as the web version.
func (ap *AssetProcessor) saveTranscodedVideo(ctx context.Context, repoPath string, asset *repo.Asset, outputPath, version string) error {
	transcodedFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open transcoded file: %w", err)
	}
	defer transcodedFile.Close()

	return ap.assetService.SaveVideoVersion(ctx, repoPath, transcodedFile, asset, version)
}

// generateVideoThumbnail creates thumbnails from a representative video frame.
func (ap *AssetProcessor) generateVideoThumbnail(ctx context.Context, repoPath string, asset *repo.Asset, videoPath string, info *VideoInfo) error {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("thumb_%s.jpg", asset.AssetID))
	defer os.Remove(outputPath)

	thumbnailTime := "00:00:01"
	if info.Duration > 0 && info.Duration < 10 {
		thumbnailSeconds := info.Duration * 0.1
		thumbnailTime = fmt.Sprintf("00:00:%02d", int(thumbnailSeconds))
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-ss", thumbnailTime,
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

	thumbnailFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open thumbnail: %w", err)
	}
	defer thumbnailFile.Close()

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

// getVideoInfo probes the video using ffprobe to collect dimensions, codec, format, and duration.
func (ap *AssetProcessor) getVideoInfo(videoPath string) (*VideoInfo, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "v:0",
		videoPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

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

	if len(probeData.Streams) > 0 {
		stream := probeData.Streams[0]
		info.Width = stream.Width
		info.Height = stream.Height
		info.Codec = stream.CodecName

		if stream.Duration != "" {
			if duration, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
				info.Duration = duration
			}
		}
	}

	info.Format = probeData.Format.FormatName

	if info.Duration == 0 && probeData.Format.Duration != "" {
		if duration, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			info.Duration = duration
		}
	}

	return info, nil
}
