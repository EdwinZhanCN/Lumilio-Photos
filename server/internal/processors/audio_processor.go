package processors

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/utils/exif"
	"strconv"
	"strings"
	"time"

	"server/internal/utils/errgroup"
)

// AudioInfo holds audio metadata
type AudioInfo struct {
	Duration   float64
	SampleRate int
	Channels   int
	Bitrate    int
	Codec      string
	Format     string
}

func (ap *AssetProcessor) processAudioAsset(
	ctx context.Context,
	repository repo.Repository,
	asset *repo.Asset,
	fileReader io.Reader,
) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	// Create temporary file for ffmpeg processing
	tempFile, err := os.CreateTemp("", "audio_processing_*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	defer os.Remove(tempFile.Name())
	defer tempFile.Close()

	// Copy input to temp file
	if _, err := io.Copy(tempFile, fileReader); err != nil {
		return fmt.Errorf("copy to temp file: %w", err)
	}
	tempFile.Close()

	// Get audio info first
	audioInfo, err := ap.getAudioInfo(tempFile.Name())
	if err != nil {
		return fmt.Errorf("get audio info: %w", err)
	}

	g := errgroup.NewFaultTolerant()

	// Goroutine 1: Extract metadata
	g.Go(func() error {
		return ap.extractAudioMetadata(timeoutCtx, asset, tempFile.Name(), audioInfo)
	})

	// Goroutine 2: Transcode audio (smart strategy)
	g.Go(func() error {
		return ap.transcodeAudioSmart(timeoutCtx, repository.Path, asset, tempFile.Name(), audioInfo)
	})

	// Goroutine 3: Generate waveform visualization (optional)
	g.Go(func() error {
		return ap.generateWaveform(timeoutCtx, repository.Path, asset, tempFile.Name())
	})

	// Wait for all tasks to complete, but don't fail the entire process if some tasks fail
	errors := g.Wait()
	if len(errors) > 0 {
		// Log individual errors but don't fail the entire process
		for _, err := range errors {
			fmt.Printf("Audio processing partial failure: %v\n", err)
		}
		// Return success even if some tasks failed, as partial processing is acceptable
	}

	return nil
}

func (ap *AssetProcessor) extractAudioMetadata(ctx context.Context, asset *repo.Asset, audioPath string, audioInfo *AudioInfo) error {
	// Use existing exif extractor for audio metadata
	file, err := os.Open(audioPath)
	if err != nil {
		return fmt.Errorf("open audio file: %w", err)
	}
	defer file.Close()

	// Configure extractor with optimized settings for audio, including fast mode
	config := &exif.Config{
		MaxFileSize: 2 * 1024 * 1024 * 1024, // 2GB
		Timeout:     60 * time.Second,
		BufferSize:  128 * 1024,
		FastMode:    true, // Use fast mode for audio to avoid full file scan
	}
	extractor := exif.NewExtractor(config)
	req := &exif.StreamingExtractRequest{
		Reader:    file,
		AssetType: dbtypes.AssetTypeAudio,
		Filename:  asset.OriginalFilename,
		Size:      asset.FileSize,
	}

	result, err := extractor.ExtractFromStream(ctx, req)
	if err != nil {
		return fmt.Errorf("extract metadata: %w", err)
	}

	if meta, ok := result.Metadata.(*dbtypes.AudioSpecificMetadata); ok {
		// Add duration info to asset record
		if err := ap.assetService.UpdateAssetDuration(ctx, asset.AssetID.Bytes, audioInfo.Duration); err != nil {
			return fmt.Errorf("update duration: %w", err)
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

func (ap *AssetProcessor) transcodeAudioSmart(ctx context.Context, repoPath string, asset *repo.Asset, audioPath string, audioInfo *AudioInfo) error {
	// Smart transcoding strategy for web compatibility
	if strings.ToLower(audioInfo.Format) == "mp3" && audioInfo.Bitrate >= 128 && audioInfo.Bitrate <= 320 {
		// Audio is already MP3 with good bitrate, just copy to storage
		return ap.copyAudioForWeb(ctx, repoPath, asset, audioPath, "web")
	}

	// Need to transcode to MP3
	outputPath, err := ap.transcodeAudioToMP3(ctx, audioPath, audioInfo)
	if err != nil {
		return fmt.Errorf("transcode to mp3: %w", err)
	}
	defer os.Remove(outputPath)

	// Just save the MP3 version
	return ap.saveTranscodedAudio(ctx, repoPath, asset, outputPath, "web")
}

func (ap *AssetProcessor) transcodeAudioToMP3(ctx context.Context, inputPath string, audioInfo *AudioInfo) (string, error) {
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("transcoded_mp3_%s.mp3", filepath.Base(inputPath)))

	// Determine optimal bitrate
	targetBitrate := "192k"
	if audioInfo.Bitrate > 0 && audioInfo.Bitrate < 192 {
		targetBitrate = "128k" // Don't artificially increase bitrate
	}

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", inputPath,
		"-c:a", "libmp3lame", // MP3 encoder
		"-b:a", targetBitrate, // Target bitrate
		"-q:a", "2", // High quality VBR
		"-ar", "44100", // Standard sample rate for web
		"-ac", "2", // Stereo (or mono if source is mono)
		"-f", "mp3",
		"-y", // Overwrite output file
		outputPath,
	)

	// If source is mono, keep it mono
	if audioInfo.Channels == 1 {
		cmd.Args[len(cmd.Args)-4] = "1" // Replace "-ac", "2" with "1"
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg audio transcode failed: %w", err)
	}

	return outputPath, nil
}

func (ap *AssetProcessor) copyAudioForWeb(ctx context.Context, repoPath string, asset *repo.Asset, audioPath, version string) error {
	audioFile, err := os.Open(audioPath)
	if err != nil {
		return fmt.Errorf("open audio file: %w", err)
	}
	defer audioFile.Close()

	return ap.assetService.SaveAudioVersion(ctx, repoPath, audioFile, asset, version)
}

func (ap *AssetProcessor) saveTranscodedAudio(ctx context.Context, repoPath string, asset *repo.Asset, outputPath, version string) error {
	transcodedFile, err := os.Open(outputPath)
	if err != nil {
		return fmt.Errorf("open transcoded file: %w", err)
	}
	defer transcodedFile.Close()

	return ap.assetService.SaveAudioVersion(ctx, repoPath, transcodedFile, asset, version)
}

func (ap *AssetProcessor) generateWaveform(ctx context.Context, repoPath string, asset *repo.Asset, audioPath string) error {
	// Generate waveform visualization image
	outputPath := filepath.Join(os.TempDir(), fmt.Sprintf("waveform_%s.png", asset.AssetID.Bytes))
	defer os.Remove(outputPath)

	cmd := exec.CommandContext(ctx, "ffmpeg",
		"-i", audioPath,
		"-filter_complex", "showwavespic=s=1200x200:colors=0x3b82f6[v]",
		"-map", "[v]",
		"-frames:v", "1",
		"-f", "image2",
		"-y", // Overwrite
		outputPath,
	)

	if err := cmd.Run(); err != nil {
		// Waveform generation is optional, don't fail the entire process
		return nil
	}

	// Save waveform as a special thumbnail
	waveformFile, err := os.Open(outputPath)
	if err != nil {
		return nil // Optional feature, don't fail
	}
	defer waveformFile.Close()

	buf := &bytes.Buffer{}
	if _, err := io.Copy(buf, waveformFile); err != nil {
		return nil
	}

	// Save as a special "waveform" thumbnail
	return ap.assetService.SaveNewThumbnail(ctx, repoPath, buf, asset, "waveform")
}

func (ap *AssetProcessor) getAudioInfo(audioPath string) (*AudioInfo, error) {
	// Get audio information using ffprobe
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "stream=codec_name,sample_rate,channels,bit_rate,duration:format=format_name,duration,bit_rate",
		"-of", "csv=p=0",
		"-select_streams", "a:0", // First audio stream
		audioPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	info := &AudioInfo{}

	// Parse stream info (first line)
	if len(lines) > 0 {
		streamParts := strings.Split(lines[0], ",")
		if len(streamParts) >= 4 {
			info.Codec = streamParts[0]
			if sampleRate, err := strconv.Atoi(streamParts[1]); err == nil {
				info.SampleRate = sampleRate
			}
			if channels, err := strconv.Atoi(streamParts[2]); err == nil {
				info.Channels = channels
			}
			if bitrate, err := strconv.Atoi(streamParts[3]); err == nil {
				info.Bitrate = bitrate / 1000 // Convert to kbps
			}
			if len(streamParts) >= 5 {
				if duration, err := strconv.ParseFloat(streamParts[4], 64); err == nil {
					info.Duration = duration
				}
			}
		}
	}

	// Parse format info (second line)
	if len(lines) > 1 {
		formatParts := strings.Split(lines[1], ",")
		if len(formatParts) >= 1 {
			info.Format = formatParts[0]
		}
		if len(formatParts) >= 2 && info.Duration == 0 {
			if duration, err := strconv.ParseFloat(formatParts[1], 64); err == nil {
				info.Duration = duration
			}
		}
		if len(formatParts) >= 3 && info.Bitrate == 0 {
			if bitrate, err := strconv.Atoi(formatParts[2]); err == nil {
				info.Bitrate = bitrate / 1000 // Convert to kbps
			}
		}
	}

	// Fallback for duration if not found
	if info.Duration == 0 {
		if duration, err := ap.getAudioDuration(audioPath); err == nil {
			info.Duration = duration
		}
	}

	return info, nil
}

func (ap *AssetProcessor) getAudioDuration(audioPath string) (float64, error) {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-show_entries", "format=duration",
		"-of", "csv=p=0",
		audioPath,
	)

	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	duration, err := strconv.ParseFloat(strings.TrimSpace(string(output)), 64)
	if err != nil {
		return 0, err
	}

	return duration, nil
}
