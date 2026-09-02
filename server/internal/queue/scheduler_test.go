package queue

import (
	"testing"

	"github.com/google/uuid"
	"server/internal/pipeline"
	"server/internal/queue/jobs"
)

func TestAssetJobArgsAreTypedAndRejectUnknownStages(t *testing.T) {
	assetID, sourceFence := uuid.New(), uuid.New()
	tests := []struct {
		name  string
		stage pipeline.Stage
		want  any
	}{
		{"analyze", pipeline.StageAnalyze, jobs.AnalyzeAssetArgs{}},
		{"derivatives", pipeline.StageDerivatives, jobs.GenerateAssetDerivativesArgs{}},
		{"transcode", pipeline.StageTranscode, jobs.TranscodeMediaArgs{}},
		{"enrich", pipeline.StageEnrich, jobs.EnrichAssetArgs{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := assetJobArgs(assetID, sourceFence, 7, pipeline.AssetPipelineVersion, test.stage)
			if err != nil {
				t.Fatal(err)
			}
			switch test.want.(type) {
			case jobs.AnalyzeAssetArgs:
				if _, ok := got.(jobs.AnalyzeAssetArgs); !ok {
					t.Fatalf("got %T, want analyze args", got)
				}
			case jobs.GenerateAssetDerivativesArgs:
				if _, ok := got.(jobs.GenerateAssetDerivativesArgs); !ok {
					t.Fatalf("got %T, want derivatives args", got)
				}
			case jobs.TranscodeMediaArgs:
				if _, ok := got.(jobs.TranscodeMediaArgs); !ok {
					t.Fatalf("got %T, want transcode args", got)
				}
			case jobs.EnrichAssetArgs:
				if _, ok := got.(jobs.EnrichAssetArgs); !ok {
					t.Fatalf("got %T, want enrich args", got)
				}
			}
		})
	}
	if _, err := assetJobArgs(assetID, sourceFence, 7, pipeline.AssetPipelineVersion, pipeline.Stage("unknown")); err == nil {
		t.Fatal("unknown asset stage was silently converted to a job")
	}
}
