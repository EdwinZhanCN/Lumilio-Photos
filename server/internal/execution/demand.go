package execution

type Step int

const (
	StepIngestLoad Step = iota + 1
	StepIngestCompute
	StepIngestPublish
	StepAnalyzeLoad
	StepAnalyzeCompute
	StepAnalyzePublish
	StepDerivativesLoad
	StepDerivativesComputeThumb
	StepDerivativesComputeVideoFrame
	StepDerivativesComputeScale
	StepDerivativesPublish
	StepTranscodeLoad
	StepTranscodeCompute
	StepTranscodePublish
	StepEnrichLoad
	StepEnrichComputePHash
	StepEnrichComputeSemantic
	StepEnrichComputeFace
	StepEnrichComputeBioClip
	StepEnrichComputeOCR
	StepEnrichPublish
	StepScanRepository
	StepScanRepositoryTurn
	StepScanRepositoryHash
	StepScanRepositoryEpoch
	StepRebuildProjection
	StepRebuildProjectionOCR
	StepRebuildProjectionLocation
	StepRebuildProjectionReindex
	StepBackupCatalog
)

type MediaType int

const (
	MediaUnknown MediaType = iota
	MediaPhoto
	MediaVideo
	MediaAudio
)

// DemandCatalog is the closed production catalog. It is instantiated from the
// immutable startup Budget, so CPU accounting and tool argv share one session.
type DemandCatalog struct{ session ToolSession }

func NewDemandCatalog(session ToolSession) DemandCatalog {
	if session.Threads < 1 {
		panic("execution demand catalog requires a positive tool-session thread count")
	}
	return DemandCatalog{session: session}
}

// Demand returns the static resource reservation required for a pipeline step and media type.
// All production execution.Resources composite literals are defined here.
func (c DemandCatalog) Demand(step Step, mediaType MediaType) Resources {
	if !validDemandKey(step, mediaType) {
		panic("unknown execution demand key")
	}

	switch step {
	case StepIngestLoad, StepIngestCompute, StepIngestPublish:
		return Resources{CPU: 1, DiskIO: 1, MemoryBytes: 64 << 20}

	case StepAnalyzeLoad, StepAnalyzeCompute, StepAnalyzePublish:
		return Resources{CPU: 1, DiskIO: 1, MemoryBytes: 64 << 20}

	case StepDerivativesLoad:
		return Resources{CPU: 1, DiskIO: 1, MemoryBytes: 64 << 20}

	case StepDerivativesComputeThumb, StepDerivativesComputeScale:
		// Photo thumbnail generation requires ImageCodec, zero VideoCodec.
		return Resources{CPU: 1, ImageCodec: 1, MemoryBytes: 128 << 20}

	case StepDerivativesComputeVideoFrame:
		// Extracting a representative video frame is separate from vips scaling.
		return Resources{CPU: 1, VideoCodec: 1, MemoryBytes: 128 << 20}

	case StepDerivativesPublish:
		return Resources{DiskIO: 1, MemoryBytes: 64 << 20}

	case StepTranscodeLoad:
		return Resources{DiskIO: 1, MemoryBytes: 64 << 20}

	case StepTranscodeCompute:
		// CPU reservation matches ffmpeg -threads exactly.
		cpu := int64(c.session.Threads)
		// Hardware acceleration applies only to video. Audio still uses the
		// software libmp3lame encoder and consumes all configured threads.
		if mediaType == MediaVideo && c.session.HardwareAccel != "" && c.session.HardwareAccel != "none" {
			cpu = 1
		}
		return Resources{CPU: cpu, VideoCodec: 1, MemoryBytes: 256 << 20}

	case StepTranscodePublish:
		return Resources{DiskIO: 1, MemoryBytes: 64 << 20}

	case StepEnrichLoad:
		return Resources{CPU: 1, DiskIO: 1, MemoryBytes: 64 << 20}

	case StepEnrichComputePHash:
		return Resources{CPU: 1, DiskIO: 1, ImageCodec: 1, MemoryBytes: 64 << 20}

	case StepEnrichComputeSemantic:
		if mediaType == MediaVideo {
			return Resources{CPU: 1, DiskIO: 1, VideoCodec: 1, Inference: 1, MemoryBytes: 256 << 20}
		}
		return Resources{CPU: 1, DiskIO: 1, ImageCodec: 1, Inference: 1, MemoryBytes: 256 << 20}

	case StepEnrichComputeFace, StepEnrichComputeBioClip:
		return Resources{CPU: 1, DiskIO: 1, ImageCodec: 1, Inference: 1, MemoryBytes: 256 << 20}

	case StepEnrichComputeOCR:
		return Resources{CPU: 1, Inference: 1, MemoryBytes: 64 << 20}

	case StepEnrichPublish:
		return Resources{CPU: 1, MemoryBytes: 16 << 20}

	case StepScanRepository, StepScanRepositoryTurn, StepScanRepositoryHash:
		return Resources{CPU: 1, DiskIO: 1, MemoryBytes: 64 << 20}

	case StepScanRepositoryEpoch:
		return Resources{CPU: 1, MemoryBytes: 1 << 20}

	case StepRebuildProjection:
		return Resources{CPU: 1, MemoryBytes: 128 << 20}

	case StepRebuildProjectionOCR:
		return Resources{CPU: 1, DiskIO: 1, MemoryBytes: 128 << 20}

	case StepRebuildProjectionLocation:
		return Resources{CPU: 1, MemoryBytes: 64 << 20}

	case StepRebuildProjectionReindex:
		return Resources{CPU: 1, DiskIO: 1, MemoryBytes: 128 << 20}

	case StepBackupCatalog:
		return Resources{CPU: 1, DiskIO: 1, MemoryBytes: 64 << 20}

	default:
		panic("execution demand catalog is missing a declared key")
	}
}

func validDemandKey(step Step, media MediaType) bool {
	switch step {
	case StepDerivativesComputeThumb:
		return media == MediaPhoto
	case StepDerivativesComputeVideoFrame:
		return media == MediaVideo || media == MediaAudio
	case StepDerivativesComputeScale:
		return media == MediaPhoto || media == MediaVideo
	case StepTranscodeCompute:
		return media == MediaVideo || media == MediaAudio
	case StepDerivativesPublish:
		return media == MediaPhoto || media == MediaVideo || media == MediaAudio
	case StepTranscodeLoad, StepTranscodePublish:
		return media == MediaVideo || media == MediaAudio
	case StepEnrichComputePHash, StepEnrichComputeOCR:
		return media == MediaPhoto
	case StepEnrichComputeSemantic, StepEnrichComputeFace, StepEnrichComputeBioClip:
		return media == MediaPhoto || media == MediaVideo
	default:
		return media == MediaUnknown
	}
}

// DemandForProjection maps projection kind names to their static resource demands.
func (c DemandCatalog) DemandForProjection(kind string) Resources {
	switch kind {
	case "ocr":
		return c.Demand(StepRebuildProjectionOCR, MediaUnknown)
	case "location_resolution":
		return c.Demand(StepRebuildProjectionLocation, MediaUnknown)
	case "asset_reindex":
		return c.Demand(StepRebuildProjectionReindex, MediaUnknown)
	default:
		return c.Demand(StepRebuildProjection, MediaUnknown)
	}
}
