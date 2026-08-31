package processors

import (
	"github.com/google/uuid"
)

// AssetStageArgs is the read-only input shared by local asset stage
// implementations. It is intentionally owned by processors rather than the
// River adapter so compute code cannot depend on queue payloads.
type AssetStageArgs struct {
	AssetID           uuid.UUID
	ExpectedContentID uuid.UUID
	PipelineVersion   string
}

// DerivedArtifact is an immutable filesystem publication awaiting catalog
// activation. The coordinator records it only after the source fence is
// revalidated.
type DerivedArtifact struct {
	AssetID      uuid.UUID
	SourceFence  uuid.UUID
	RepositoryID uuid.UUID
	Size         string
	StoragePath  string
	MimeType     string
}

type DerivativeResult struct {
	AssetID, SourceContentID uuid.UUID
	Artifacts                []DerivedArtifact
}

type VideoFrameEmbedding struct {
	FrameTsMs int32
	Vector    []float32
}

type VideoFramesResult struct {
	AssetID, SourceContentID uuid.UUID
	ModelID                  string
	Frames                   []VideoFrameEmbedding
}

// VideoFramesArgs adds the model preprocessing contract used by video frame
// extraction. The macro supplies the source fence; no River identity is
// carried into the processor.
type VideoFramesArgs struct {
	AssetID           uuid.UUID
	ExpectedContentID uuid.UUID
	PreprocessVersion string
}

type MetadataArgs = AssetStageArgs
type ThumbnailArgs = AssetStageArgs
type TranscodeArgs = AssetStageArgs
