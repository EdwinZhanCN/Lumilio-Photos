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
	"runtime"
	"strconv"
	"strings"
	"time"

	"server/config"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/execution"
	"server/internal/storage"
	"server/internal/utils/exif"
	fileutil "server/internal/utils/file"
	"server/internal/utils/sysproc"
)

// VideoInfo holds video metadata.
type VideoInfo struct {
	Width     int
	Height    int
	Duration  float64
	Codec     string
	Bitrate   int     // bit/s
	FrameRate float64 // fps
	Format    string
}

// extractVideoMetadata updates the asset with ffprobe/EXIF-derived metadata.
func (ap *AssetProcessor) extractVideoMetadata(ctx context.Context, asset *repo.Asset, fileSize int64, reader io.Reader, videoInfo *VideoInfo) (MetadataResult, error) {
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
		Reader:    reader,
		AssetType: dbtypes.AssetTypeVideo,
		Filename:  asset.OriginalFilename,
		Size:      fileSize,
	}

	result, err := extractor.ExtractFromStream(ctx, req)
	if err != nil {
		return MetadataResult{}, fmt.Errorf("extract metadata: %w", err)
	}
	if result.Error != nil {
		return MetadataResult{}, fmt.Errorf("extract metadata: %w", result.Error)
	}

	meta, ok := result.Metadata.(*dbtypes.VideoSpecificMetadata)
	if !ok {
		return MetadataResult{}, fmt.Errorf("unexpected metadata type for video: %T", result.Metadata)
	}

	if videoInfo.Codec != "" {
		meta.Codec = videoInfo.Codec
	}
	if videoInfo.Bitrate > 0 {
		meta.Bitrate = videoInfo.Bitrate
	}
	if videoInfo.FrameRate > 0 {
		meta.FrameRate = videoInfo.FrameRate
	}
	common := result.Common
	if videoInfo.Width > 0 && videoInfo.Height > 0 {
		width, height := int32(videoInfo.Width), int32(videoInfo.Height)
		common.Width, common.Height = &width, &height
	}
	if videoInfo.Duration > 0 {
		duration := videoInfo.Duration
		common.Duration = &duration
	}
	sm, err := dbtypes.MarshalMeta(meta)
	if err != nil {
		return MetadataResult{}, fmt.Errorf("marshal metadata: %w", err)
	}
	relation := repo.InitialMediaRelation(&fileutil.ValidationResult{MimeType: asset.MimeType}, asset.OriginalFilename)
	return MetadataResult{AssetID: asset.AssetID, SourceContentID: asset.ContentID, Metadata: sm, Common: common, ExifRaw: dbtypes.JSON(result.Raw), ComponentRelation: string(relation)}, nil
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

	args := buildTranscodeArgs(inputPath, outputPath, scaleFilter, approxWidth, approxHeight, cfg, ap.toolSession)
	cmd := exec.CommandContext(ctx, ap.toolsConfig.FFmpegCommand(), args...)
	sysproc.HideConsole(cmd)

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg transcode failed: %w", err)
	}

	return outputPath, nil
}

// ResolveHardwareAccel translates "auto" or requested hardware acceleration mode
// to the actual acceleration backend supported by the host operating system/hardware.
func ResolveHardwareAccel(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode != "auto" {
		return mode
	}
	if runtime.GOOS == "darwin" {
		return "videotoolbox"
	}
	if _, err := os.Stat("/dev/dri/renderD128"); err == nil {
		return "vaapi"
	}
	return "none"
}

func buildTranscodeArgs(inputPath, outputPath, scaleFilter string, approxWidth, approxHeight int, cfg config.TranscodeConfig, session execution.ToolSession) []string {
	scaleExpr := scaleFilter[len("scale="):] // w:h portion, reused for VAAPI
	maxrate, bufsize := bitrateForResolution(approxWidth, approxHeight)

	accel := session.HardwareAccel
	if accel == "" {
		// The runtime always passes the resolved session. Keeping this branch
		// makes the pure argv builder usable by narrowly scoped unit tests.
		accel = cfg.HardwareAccel
	}

	switch accel {
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
		args := []string{
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-c:v", "h264_nvenc",
		}
		args = append(args, session.NVENCPresetArg()...)
		args = append(args,
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
		)
		return args
	case "qsv":
		return []string{
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-c:v", "h264_qsv",
			"-global_quality", "23",
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
	case "videotoolbox":
		return []string{
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-c:v", "h264_videotoolbox",
			"-realtime", "0",
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
		args := []string{
			"-i", inputPath,
			"-map", "0:v:0",
			"-map", "0:a?",
			"-c:v", "libx264",
		}
		args = append(args, session.FFmpegPresetArg()...)
		args = append(args,
			"-crf", "23",
			"-maxrate", maxrate,
			"-bufsize", bufsize,
			"-vf", scaleFilter,
			"-pix_fmt", "yuv420p",
			"-c:a", "aac",
			"-b:a", "128k",
			"-movflags", "+faststart",
			"-avoid_negative_ts", "make_zero",
		)
		args = append(args, session.FFmpegThreadsArg()...)
		args = append(args,
			"-f", "mp4",
			"-y",
			outputPath,
		)
		return args
	}
}

// copyVideoAsWebVersion saves the provided video file as the web version.
func copyVideoAsWebVersion(ctx context.Context, files *storage.RepositoryFS, sourcePath storage.RepositoryPath, asset *repo.Asset, pipelineVersion, version string) error {
	videoFile, err := files.OpenMedia(sourcePath)
	if err != nil {
		return fmt.Errorf("open video file: %w", err)
	}
	defer videoFile.Close()

	return saveVideoVersion(ctx, files, videoFile, asset, pipelineVersion, version)
}

// saveTranscodedVideo saves a transcoded output as the web version.
func (ap *AssetProcessor) saveTranscodedVideo(ctx context.Context, files *storage.RepositoryFS, asset *repo.Asset, outputPath, pipelineVersion, version string) error {
	transcodedFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open transcoded file: %w", err)
	}
	defer transcodedFile.Close()

	return saveVideoVersion(ctx, files, transcodedFile, asset, pipelineVersion, version)
}

// extractVideoThumbnailFrame creates one representative JPEG. Scaling and
// publication are separate runtime admissions.
func (ap *AssetProcessor) extractVideoThumbnailFrame(ctx context.Context, asset *repo.Asset, videoPath string, info *VideoInfo) (string, error) {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("thumb_%s.jpg", asset.AssetID))

	thumbnailTime := "00:00:01"
	if info.Duration > 0 && info.Duration < 10 {
		thumbnailSeconds := info.Duration * 0.1
		thumbnailTime = fmt.Sprintf("00:00:%02d", int(thumbnailSeconds))
	}

	args := []string{}

	accel := ap.toolSession.HardwareAccel
	switch accel {
	case "vaapi":
		args = append(args,
			"-hwaccel", "vaapi",
			"-vaapi_device", "/dev/dri/renderD128",
		)
	case "videotoolbox":
		args = append(args,
			"-hwaccel", "videotoolbox",
		)
	case "nvenc":
		args = append(args,
			"-hwaccel", "cuda",
		)
	case "qsv":
		args = append(args,
			"-hwaccel", "qsv",
		)
	}

	args = append(args,
		"-ss", thumbnailTime,
		"-i", videoPath,
		"-vframes", "1",
		"-q:v", "2",
		"-vf", "scale='min(1920,iw)':'min(1080,ih)':force_original_aspect_ratio=decrease",
	)
	args = append(args, ap.toolSession.FFmpegThreadsArg()...)
	args = append(args,
		"-f", "mjpeg",
		"-y",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, ap.toolsConfig.FFmpegCommand(), args...)
	sysproc.HideConsole(cmd)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		_ = os.Remove(outputPath)
		return "", fmt.Errorf("generate thumbnail: %w\nstderr: %s", err, stderr.String())
	}
	return outputPath, nil
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

	var data struct {
		Streams []struct {
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			CodecName  string `json:"codec_name"`
			RFrameRate string `json:"r_frame_rate"`
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &data); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	info := &VideoInfo{}
	if len(data.Streams) > 0 {
		s := data.Streams[0]
		info.Width = s.Width
		info.Height = s.Height
		info.Codec = s.CodecName
		info.FrameRate = parseFrameRate(s.RFrameRate)
		if d, err := strconv.ParseFloat(s.Duration, 64); err == nil {
			info.Duration = d
		}
		if b, err := strconv.Atoi(s.BitRate); err == nil {
			info.Bitrate = b
		}
	}

	info.Format = data.Format.FormatName
	if info.Duration == 0 {
		if d, err := strconv.ParseFloat(data.Format.Duration, 64); err == nil {
			info.Duration = d
		}
	}
	if info.Bitrate == 0 {
		if b, err := strconv.Atoi(data.Format.BitRate); err == nil {
			info.Bitrate = b
		}
	}

	return info, nil
}

func parseFrameRate(s string) float64 {
	parts := strings.Split(s, "/")
	if len(parts) == 2 {
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den != 0 {
			return num / den
		}
	}
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return f
	}
	return 0
}
