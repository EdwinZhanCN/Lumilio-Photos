package processors

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
)

// PreparedTranscode carries immutable source identity and temporary encoder
// output across admissions. Repository leases are stage-local and never held
// while the execution governor admits a later stage.
type PreparedTranscode struct {
	assetID         uuid.UUID
	sourceContentID uuid.UUID
	assetType       dbtypes.AssetType
	videoInfo       *VideoInfo
	audioInfo       *AudioInfo
	pipelineVersion string
	outputPath      string
	copySource      bool
}

func (work *PreparedTranscode) Close() error {
	if work == nil {
		return nil
	}
	if work.outputPath != "" {
		_ = os.Remove(work.outputPath)
		work.outputPath = ""
	}
	return nil
}

// LoadTranscodeTask resolves and validates the source and performs ffprobe
// while the runtime holds the transcode-load DiskIO reservation.
func (ap *AssetProcessor) LoadTranscodeTask(ctx context.Context, args TranscodeArgs) (*PreparedTranscode, error) {
	source, err := ap.resolveCurrentAssetSource(ctx, args.AssetID, args.ExpectedContentID)
	if err != nil {
		if errors.Is(err, ErrAssetSourceStale) {
			return nil, nil
		}
		return nil, err
	}
	defer source.Close()
	work := &PreparedTranscode{
		assetID: source.asset.AssetID, sourceContentID: source.asset.ContentID,
		assetType: dbtypes.AssetType(source.asset.Type), pipelineVersion: args.PipelineVersion,
	}
	switch work.assetType {
	case dbtypes.AssetTypeVideo:
		work.videoInfo, err = ap.getVideoInfo(source.localPath)
	case dbtypes.AssetTypeAudio:
		work.audioInfo, err = ap.getAudioInfo(source.localPath)
	case dbtypes.AssetTypePhoto:
		return work, nil
	default:
		err = fmt.Errorf("unsupported asset type for transcode: %s", work.assetType)
	}
	if err != nil {
		return nil, err
	}
	return work, nil
}

// ComputeTranscodeTask performs only codec work. It never publishes an
// artifact, so the VideoCodec slot is released before repository writes.
func (ap *AssetProcessor) ComputeTranscodeTask(ctx context.Context, work *PreparedTranscode) error {
	if work == nil || work.assetType == dbtypes.AssetTypePhoto {
		return nil
	}
	source, err := ap.resolveCurrentAssetSource(ctx, work.assetID, work.sourceContentID)
	if err != nil {
		if errors.Is(err, ErrAssetSourceStale) {
			return nil
		}
		return err
	}
	defer source.Close()
	switch work.assetType {
	case dbtypes.AssetTypeAudio:
		if strings.EqualFold(work.audioInfo.Format, "mp3") && work.audioInfo.Bitrate >= 128000 && work.audioInfo.Bitrate <= 320000 {
			work.copySource = true
			return nil
		}
		path, err := ap.transcodeAudioToMP3(ctx, source.localPath, work.audioInfo)
		work.outputPath = path
		return err
	case dbtypes.AssetTypeVideo:
		info := work.videoInfo
		longSide := info.Width
		if info.Height > longSide {
			longSide = info.Height
		}
		if longSide <= 1080 && info.Width >= info.Height && strings.EqualFold(info.Format, "mp4") && strings.Contains(strings.ToLower(info.Codec), "h264") {
			work.copySource = true
			return nil
		}
		width, height := info.Width, info.Height
		filter := buildScaleFilter(info.Width, info.Height, info.Width, info.Height)
		if longSide > 1080 {
			if info.Width >= info.Height {
				filter, width, height = "scale=-2:1080", int(float64(1080)*float64(info.Width)/float64(info.Height)), 1080
			} else {
				filter, width, height = "scale=1080:-2", 1080, int(float64(1080)*float64(info.Height)/float64(info.Width))
			}
		}
		path, err := ap.transcodeVideoToMP4(ctx, source.localPath, filter, width, height, ap.transcodeConfig)
		work.outputPath = path
		return err
	default:
		return fmt.Errorf("unsupported asset type for transcode: %s", work.assetType)
	}
}

// PublishTranscodeTask performs the source/output read and immutable artifact
// write while the runtime holds DiskIO and no codec reservation.
func (ap *AssetProcessor) PublishTranscodeTask(ctx context.Context, work *PreparedTranscode) error {
	if work == nil || work.assetType == dbtypes.AssetTypePhoto {
		return nil
	}
	source, err := ap.resolveCurrentAssetSource(ctx, work.assetID, work.sourceContentID)
	if err != nil {
		if errors.Is(err, ErrAssetSourceStale) {
			return nil
		}
		return err
	}
	defer source.Close()
	if work.copySource {
		if work.assetType == dbtypes.AssetTypeAudio {
			return copyAudioForWeb(ctx, source.files, source.path, source.asset, work.pipelineVersion, "web")
		}
		return copyVideoAsWebVersion(ctx, source.files, source.path, source.asset, work.pipelineVersion, "web")
	}
	if work.outputPath == "" {
		return errors.New("transcode compute produced no output")
	}
	if work.assetType == dbtypes.AssetTypeAudio {
		return ap.saveTranscodedAudio(ctx, source.files, source.asset, work.outputPath, work.pipelineVersion, "web")
	}
	return ap.saveTranscodedVideo(ctx, source.files, source.asset, work.outputPath, work.pipelineVersion, "web")
}
