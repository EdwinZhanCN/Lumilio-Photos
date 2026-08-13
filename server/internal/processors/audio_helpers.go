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
	"server/internal/storage"
	"server/internal/utils/exif"
	"server/internal/utils/sysproc"
)

// AudioInfo holds audio metadata.
type AudioInfo struct {
	Duration   float64
	SampleRate int
	Channels   int
	Bitrate    int // bit/s
	Codec      string
	Format     string
}

// extractAudioMetadata updates the asset with ffprobe/EXIF-derived metadata.
func (ap *AssetProcessor) extractAudioMetadata(ctx context.Context, asset *repo.Asset, reader io.Reader, audioInfo *AudioInfo) error {
	config := &exif.Config{
		ExifToolPath: ap.toolsConfig.ExifToolCommand(),
		MaxFileSize:  2 * 1024 * 1024 * 1024, // 2GB
		Timeout:      60 * time.Second,
		BufferSize:   128 * 1024,
		FastMode:     true,
		IncludeRaw:   true,
	}
	extractor := exif.NewExtractor(config)
	defer extractor.Close()

	req := &exif.StreamingExtractRequest{
		Reader:    reader,
		AssetType: dbtypes.AssetTypeAudio,
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

	meta, ok := result.Metadata.(*dbtypes.AudioSpecificMetadata)
	if !ok {
		return fmt.Errorf("unexpected metadata type for audio: %T", result.Metadata)
	}

	if audioInfo.Codec != "" {
		meta.Codec = audioInfo.Codec
	}
	if audioInfo.Bitrate > 0 {
		meta.Bitrate = audioInfo.Bitrate
	}
	if audioInfo.SampleRate > 0 {
		meta.SampleRate = audioInfo.SampleRate
	}
	if audioInfo.Channels > 0 {
		meta.Channels = audioInfo.Channels
	}
	common := result.Common
	if audioInfo.Duration > 0 {
		duration := audioInfo.Duration
		common.Duration = &duration
	}

	sm, err := dbtypes.MarshalMeta(meta)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}

	if err := ap.assetService.UpdateAssetExtractedMetadata(ctx, asset.AssetID, sm, common, result.Raw); err != nil {
		return fmt.Errorf("save metadata: %w", err)
	}
	ap.reconcileComponentRelation(ctx, asset, false, asset.MimeType)
	ap.enqueueDetectStacks(ctx, asset)

	return nil
}

// transcodeAudioSmart applies a best-effort, resource-aware transcoding strategy.
func (ap *AssetProcessor) transcodeAudioSmart(ctx context.Context, files *storage.RepositoryFS, sourcePath storage.RepositoryPath, asset *repo.Asset, audioPath string, audioInfo *AudioInfo) error {
	if strings.ToLower(audioInfo.Format) == "mp3" && audioInfo.Bitrate >= 128000 && audioInfo.Bitrate <= 320000 {
		return copyAudioForWeb(files, sourcePath, asset, "web")
	}

	outputPath, err := ap.transcodeAudioToMP3(ctx, audioPath, audioInfo)
	if err != nil {
		return fmt.Errorf("transcode to mp3: %w", err)
	}
	defer os.Remove(outputPath)

	return ap.saveTranscodedAudio(files, asset, outputPath, "web")
}

// transcodeAudioToMP3 runs ffmpeg to produce an MP3 at a reasonable bitrate.
func (ap *AssetProcessor) transcodeAudioToMP3(ctx context.Context, inputPath string, audioInfo *AudioInfo) (string, error) {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("transcoded_mp3_%s.mp3", filepath.Base(inputPath)))

	targetBitrate := "192k"
	if audioInfo.Bitrate > 0 && audioInfo.Bitrate < 192000 {
		targetBitrate = "128k"
	}

	cmd := exec.CommandContext(ctx, ap.toolsConfig.FFmpegCommand(),
		"-i", inputPath,
		"-c:a", "libmp3lame",
		"-b:a", targetBitrate,
		"-q:a", "2",
		"-ar", "44100",
		"-ac", "2",
		"-f", "mp3",
		"-y",
		outputPath,
	)
	sysproc.HideConsole(cmd)

	if audioInfo.Channels == 1 {
		cmd.Args[len(cmd.Args)-4] = "1" // keep mono if source is mono
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg audio transcode failed: %w", err)
	}

	return outputPath, nil
}

// copyAudioForWeb saves the provided audio file as the web version.
func copyAudioForWeb(files *storage.RepositoryFS, sourcePath storage.RepositoryPath, asset *repo.Asset, version string) error {
	audioFile, err := files.OpenMedia(sourcePath)
	if err != nil {
		return fmt.Errorf("open audio file: %w", err)
	}
	defer audioFile.Close()

	return saveAudioVersion(files, audioFile, asset, version)
}

// saveTranscodedAudio saves a transcoded output as the web version.
func (ap *AssetProcessor) saveTranscodedAudio(files *storage.RepositoryFS, asset *repo.Asset, outputPath, version string) error {
	transcodedFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open transcoded file: %w", err)
	}
	defer transcodedFile.Close()

	return saveAudioVersion(files, transcodedFile, asset, version)
}

// generateWaveform produces a waveform thumbnail image (best-effort; non-fatal).
func (ap *AssetProcessor) generateWaveform(ctx context.Context, files *storage.RepositoryFS, asset *repo.Asset, audioPath string) error {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("waveform_%s.png", asset.AssetID))
	defer os.Remove(outputPath)

	cmd := exec.CommandContext(ctx, ap.toolsConfig.FFmpegCommand(),
		"-i", audioPath,
		"-filter_complex", "showwavespic=s=1200x200:colors=0x3b82f6[v]",
		"-map", "[v]",
		"-frames:v", "1",
		"-f", "image2",
		"-y",
		outputPath,
	)
	sysproc.HideConsole(cmd)

	if err := cmd.Run(); err != nil {
		return nil // optional: ignore errors
	}

	waveformFile, err := os.Open(outputPath)
	if err != nil {
		return nil // optional: ignore errors
	}
	defer waveformFile.Close()

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, waveformFile); err != nil {
		return nil // optional: ignore errors
	}

	return ap.saveThumbnail(ctx, files, buf, asset, "waveform")
}

// getAudioInfo probes the audio using ffprobe to collect duration, bitrate, codec, and format.
func (ap *AssetProcessor) getAudioInfo(audioPath string) (*AudioInfo, error) {
	cmd := exec.Command(ap.toolsConfig.FFprobeCommand(),
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		"-select_streams", "a:0",
		audioPath,
	)
	sysproc.HideConsole(cmd)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	var probeData struct {
		Streams []struct {
			SampleRate string `json:"sample_rate"`
			Channels   int    `json:"channels"`
			CodecName  string `json:"codec_name"`
			BitRate    string `json:"bit_rate"`
			Duration   string `json:"duration"`
		} `json:"streams"`
		Format struct {
			FormatName string `json:"format_name"`
			Duration   string `json:"duration"`
			BitRate    string `json:"bit_rate"`
		} `json:"format"`
	}

	if err := json.Unmarshal(output, &probeData); err != nil {
		return nil, fmt.Errorf("parse ffprobe json: %w", err)
	}

	info := &AudioInfo{}

	if len(probeData.Streams) > 0 {
		stream := probeData.Streams[0]
		if sr, err := strconv.Atoi(stream.SampleRate); err == nil {
			info.SampleRate = sr
		}
		info.Channels = stream.Channels
		info.Codec = stream.CodecName
		if br, err := strconv.Atoi(stream.BitRate); err == nil {
			info.Bitrate = br
		}
		if stream.Duration != "" {
			if dur, err := strconv.ParseFloat(stream.Duration, 64); err == nil {
				info.Duration = dur
			}
		}
	}

	info.Format = probeData.Format.FormatName
	if info.Bitrate == 0 && probeData.Format.BitRate != "" {
		if br, err := strconv.Atoi(probeData.Format.BitRate); err == nil {
			info.Bitrate = br
		}
	}
	if info.Duration == 0 && probeData.Format.Duration != "" {
		if dur, err := strconv.ParseFloat(probeData.Format.Duration, 64); err == nil {
			info.Duration = dur
		}
	}

	return info, nil
}
