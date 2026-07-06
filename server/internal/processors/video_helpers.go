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

	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/utils/exif"
	"server/internal/utils/imaging"
	"server/internal/utils/sysproc"
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
		ExifToolPath: ap.toolsConfig.ExifToolCommand(),
		MaxFileSize:  20 * 1024 * 1024 * 1024, // 20GB
		Timeout:      60 * time.Second,        // 60s
		BufferSize:   128 * 1024,
		FastMode:     true,
		IncludeRaw:   true,
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
	if result.Error != nil {
		return fmt.Errorf("extract metadata: %w", result.Error)
	}

	meta, ok := result.Metadata.(*dbtypes.VideoSpecificMetadata)
	if !ok {
		return fmt.Errorf("unexpected metadata type for video: %T", result.Metadata)
	}

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
	if err := ap.assetService.UpdateAssetMetadataWithExifRaw(ctx, asset.AssetID.Bytes, sm, result.Raw); err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}
	ap.enqueueLivePhotoMatcher(ctx, asset, meta.ContentIdentifier)

	return nil
}

// transcodeVideoSmart applies a best-effort, resource-aware transcoding strategy.
// Constrains by the longer side: landscape videos are capped at 1080p height,
// portrait videos are capped at 1080p width.
func (ap *AssetProcessor) transcodeVideoSmart(ctx context.Context, repoPath string, asset *repo.Asset, videoPath string, videoInfo *VideoInfo, cfg config.TranscodeConfig) error {
	maxDimension := 1080
	longSide := videoInfo.Width
	if videoInfo.Height > longSide {
		longSide = videoInfo.Height
	}

	isLandscape := videoInfo.Width >= videoInfo.Height

	// Already within bounds: copy if H.264 MP4, otherwise transcode at original size.
	if longSide <= maxDimension {
		if isLandscape && strings.ToLower(videoInfo.Format) == "mp4" && strings.Contains(strings.ToLower(videoInfo.Codec), "h264") {
			return ap.copyVideoAsWebVersion(ctx, repoPath, asset, videoPath, "web")
		}
		scaleFilter := buildScaleFilter(videoInfo.Width, videoInfo.Height, videoInfo.Width, videoInfo.Height)
		outputPath, err := ap.transcodeVideoToMP4(ctx, videoPath, scaleFilter, videoInfo.Width, videoInfo.Height, cfg)
		if err != nil {
			return fmt.Errorf("transcode to mp4: %w", err)
		}
		defer os.Remove(outputPath)
		return ap.saveTranscodedVideo(ctx, repoPath, asset, outputPath, "web")
	}

	// Scale down: constrain the longer side to maxDimension, let ffmpeg compute
	// the other dimension precisely with -2 (preserves aspect ratio, ensures even).
	var scaleFilter string
	var approxWidth, approxHeight int
	if isLandscape {
		scaleFilter = fmt.Sprintf("scale=-2:%d", maxDimension)
		approxWidth = int(float64(maxDimension) * float64(videoInfo.Width) / float64(videoInfo.Height))
		approxHeight = maxDimension
	} else {
		scaleFilter = fmt.Sprintf("scale=%d:-2", maxDimension)
		approxWidth = maxDimension
		approxHeight = int(float64(maxDimension) * float64(videoInfo.Height) / float64(videoInfo.Width))
	}

	outputPath, err := ap.transcodeVideoToMP4(ctx, videoPath, scaleFilter, approxWidth, approxHeight, cfg)
	if err != nil {
		return fmt.Errorf("transcode to %dp: %w", maxDimension, err)
	}
	defer os.Remove(outputPath)

	if err := ap.saveTranscodedVideo(ctx, repoPath, asset, outputPath, "web"); err != nil {
		return fmt.Errorf("save %dp version: %w", maxDimension, err)
	}

	return nil
}

// buildScaleFilter returns an ffmpeg scale filter string. Uses -2 for one
// dimension so ffmpeg computes it precisely while keeping aspect ratio and
// ensuring even dimensions.
func buildScaleFilter(srcW, srcH, targetW, targetH int) string {
	if srcW >= srcH {
		// landscape: constrain by height
		return fmt.Sprintf("scale=-2:%d", targetH)
	}
	// portrait: constrain by width
	return fmt.Sprintf("scale=%d:-2", targetW)
}

// bitrateForResolution computes maxrate/bufsize based on pixel count.
func bitrateForResolution(width, height int) (maxrate, bufsize string) {
	pixels := width * height
	rate := pixels / 300 // kbps, e.g. 1920×1080 → ~6912k
	if rate < 2000 {
		rate = 2000
	}
	return fmt.Sprintf("%dk", rate), fmt.Sprintf("%dk", rate*2)
}

// transcodeVideoToMP4 runs ffmpeg to produce an H.264/AAC MP4.
// scaleFilter is the ffmpeg scale expression (e.g. "scale=-2:1080").
// approxWidth/approxHeight are used for bitrate estimation and output filename.
func (ap *AssetProcessor) transcodeVideoToMP4(ctx context.Context, inputPath string, scaleFilter string, approxWidth, approxHeight int, cfg config.TranscodeConfig) (string, error) {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("transcoded_%d_%s.mp4", approxHeight, filepath.Base(inputPath)))

	args := buildTranscodeArgs(inputPath, outputPath, scaleFilter, approxWidth, approxHeight, cfg)
	cmd := exec.CommandContext(ctx, ap.toolsConfig.FFmpegCommand(), args...)
	sysproc.HideConsole(cmd)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg transcode failed: %w", err)
	}

	return outputPath, nil
}

func buildTranscodeArgs(inputPath, outputPath, scaleFilter string, approxWidth, approxHeight int, cfg config.TranscodeConfig) []string {
	scaleExpr := scaleFilter[len("scale="):] // w:h portion, reused for VAAPI
	maxrate, bufsize := bitrateForResolution(approxWidth, approxHeight)

	switch cfg.HardwareAccel {
	case "vaapi":
		return []string{
			"-vaapi_device", "/dev/dri/renderD128",
			"-hwaccel", "vaapi",
			"-hwaccel_output_format", "vaapi",
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-vf", "scale_vaapi=" + scaleExpr,
			"-c:v", "h264_vaapi",
			"-qp", "23",
			"-maxrate", maxrate,
			"-bufsize", bufsize,
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
			"-avoid_negative_ts", "make_zero",
			"-f", "mp4",
			"-y",
			outputPath,
		}
	case "nvenc":
		return []string{
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-c:v", "h264_nvenc",
			"-preset", "p4",
			"-qp", "23",
			"-maxrate", maxrate,
			"-bufsize", bufsize,
			"-vf", scaleFilter,
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
			"-avoid_negative_ts", "make_zero",
			"-f", "mp4",
			"-y",
			outputPath,
		}
	default:
		return []string{
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-c:v", "libx264",
			"-preset", "medium",
			"-crf", "23",
			"-maxrate", maxrate,
			"-bufsize", bufsize,
			"-vf", scaleFilter,
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
			"-avoid_negative_ts", "make_zero",
			"-threads", "0",
			"-f", "mp4",
			"-y",
			outputPath,
		}
	}
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
func (ap *AssetProcessor) generateVideoThumbnail(ctx context.Context, repoPath string, asset *repo.Asset, videoPath string, info *VideoInfo, cfg config.TranscodeConfig) error {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("thumb_%s.jpg", asset.AssetID))
	defer os.Remove(outputPath)

	thumbnailTime := "00:00:01"
	if info.Duration > 0 && info.Duration < 10 {
		thumbnailSeconds := info.Duration * 0.1
		thumbnailTime = fmt.Sprintf("00:00:%02d", int(thumbnailSeconds))
	}

	args := []string{}

	if cfg.HardwareAccel == "vaapi" {
		args = append(args,
			"-hwaccel", "vaapi",
			"-vaapi_device", "/dev/dri/renderD128",
		)
	}

	args = append(args,
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

	cmd := exec.CommandContext(ctx, ap.toolsConfig.FFmpegCommand(), args...)
	sysproc.HideConsole(cmd)

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
	cmd := exec.Command(ap.toolsConfig.FFprobeCommand(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "v:0",
		videoPath,
	)
	sysproc.HideConsole(cmd)

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
