package processors

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/google/uuid"

	"server/internal/db/dbtypes"
	"server/internal/utils/imagesource"
	"server/internal/utils/imaging"
)

// PreparedThumbnail carries only immutable source identity and in-memory codec
// output across admissions. Repository leases never cross an admission
// boundary; every stage revalidates the source fence and closes its own lease.
type PreparedThumbnail struct {
	assetID         uuid.UUID
	sourceContentID uuid.UUID
	assetType       dbtypes.AssetType
	videoInfo       *VideoInfo
	pipelineVersion string
	frame           []byte
	outputs         map[string][]byte
}

func (work *PreparedThumbnail) MediaType() dbtypes.AssetType {
	if work == nil {
		return ""
	}
	return work.assetType
}

// LoadThumbnailTask resolves the source and performs video probing under the
// load-stage DiskIO reservation.
func (ap *AssetProcessor) LoadThumbnailTask(ctx context.Context, args ThumbnailArgs) (*PreparedThumbnail, error) {
	source, err := ap.resolveCurrentAssetSource(ctx, args.AssetID, args.ExpectedContentID)
	if err != nil {
		if errors.Is(err, ErrAssetSourceStale) {
			return nil, nil
		}
		return nil, err
	}
	defer source.Close()
	work := &PreparedThumbnail{
		assetID: source.asset.AssetID, sourceContentID: source.asset.ContentID,
		assetType: dbtypes.AssetType(source.asset.Type), pipelineVersion: args.PipelineVersion,
	}
	if work.assetType == dbtypes.AssetTypeVideo {
		work.videoInfo, err = ap.getVideoInfo(source.localPath)
		if err != nil {
			return nil, err
		}
	}
	return work, nil
}

// ComputeThumbnailCodec performs the media-specific decode/extract operation.
// Video scaling is intentionally deferred to ComputeThumbnailScale.
func (ap *AssetProcessor) ComputeThumbnailCodec(ctx context.Context, work *PreparedThumbnail) error {
	if work == nil {
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
	case dbtypes.AssetTypePhoto:
		reader, err := imagesource.OpenPhoto(ctx, source.localPath, source.asset.OriginalFilename)
		if err != nil {
			return fmt.Errorf("open photo source: %w", err)
		}
		defer reader.Close()
		work.outputs, err = thumbnailBuffers(reader)
		return err
	case dbtypes.AssetTypeVideo:
		framePath, err := ap.extractVideoThumbnailFrame(ctx, source.asset, source.localPath, work.videoInfo)
		if err != nil {
			return err
		}
		defer os.Remove(framePath)
		work.frame, err = os.ReadFile(framePath)
		return err
	case dbtypes.AssetTypeAudio:
		work.frame, err = ap.computeWaveform(ctx, source.asset, source.localPath)
		return err
	default:
		return fmt.Errorf("unsupported asset type for thumbnails: %s", work.assetType)
	}
}

// ComputeThumbnailScale runs libvips for the extracted video frame under its
// own ImageCodec reservation. Audio waveform and photo outputs are complete.
func (ap *AssetProcessor) ComputeThumbnailScale(_ context.Context, work *PreparedThumbnail) error {
	if work == nil || work.assetType != dbtypes.AssetTypeVideo {
		return nil
	}
	outputs, err := thumbnailBuffers(bytes.NewReader(work.frame))
	if err != nil {
		return err
	}
	work.outputs = outputs
	work.frame = nil
	return nil
}

func thumbnailBuffers(reader io.Reader) (map[string][]byte, error) {
	writers := make(map[string]io.Writer, len(thumbnailSizes))
	buffers := make(map[string]*bytes.Buffer, len(thumbnailSizes))
	for name := range thumbnailSizes {
		buf := &bytes.Buffer{}
		buffers[name], writers[name] = buf, buf
	}
	if err := imaging.StreamThumbnails(reader, thumbnailSizes, writers); err != nil {
		return nil, fmt.Errorf("generate thumbnails: %w", err)
	}
	outputs := make(map[string][]byte, len(buffers))
	for name, buf := range buffers {
		if buf.Len() > 0 {
			outputs[name] = bytes.Clone(buf.Bytes())
		}
	}
	return outputs, nil
}

// PublishThumbnailTask is the only derivative filesystem publication path.
func (ap *AssetProcessor) PublishThumbnailTask(ctx context.Context, work *PreparedThumbnail) (DerivativeResult, error) {
	if work == nil {
		return DerivativeResult{}, nil
	}
	source, err := ap.resolveCurrentAssetSource(ctx, work.assetID, work.sourceContentID)
	if err != nil {
		if errors.Is(err, ErrAssetSourceStale) {
			return DerivativeResult{}, nil
		}
		return DerivativeResult{}, err
	}
	defer source.Close()
	if work.assetType == dbtypes.AssetTypeAudio && len(work.frame) > 0 {
		work.outputs = map[string][]byte{"waveform": work.frame}
	}
	artifacts := make([]DerivedArtifact, 0, len(work.outputs))
	for name, output := range work.outputs {
		artifact, err := ap.saveThumbnail(ctx, source.files, bytes.NewReader(output), source.asset, work.pipelineVersion, name)
		if err != nil {
			return DerivativeResult{}, fmt.Errorf("publish thumbnail %s: %w", name, err)
		}
		artifacts = append(artifacts, artifact)
	}
	return DerivativeResult{AssetID: source.asset.AssetID, SourceContentID: source.asset.ContentID, Artifacts: artifacts}, nil
}
