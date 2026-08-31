package execution

import (
	"testing"
)

func TestDemandCatalogVectors(t *testing.T) {
	catalog := NewDemandCatalog(ToolSession{
		Threads:        3,
		SoftwarePreset: "faster",
	})

	// 1. Photo vs Video thumbnail compute separation
	photoThumb := catalog.Demand(StepDerivativesComputeThumb, MediaPhoto)
	if photoThumb.ImageCodec != 1 || photoThumb.VideoCodec != 0 {
		t.Fatalf("photo thumb compute demand should require ImageCodec=1, VideoCodec=0: got %+v", photoThumb)
	}

	videoThumb := catalog.Demand(StepDerivativesComputeVideoFrame, MediaVideo)
	if videoThumb.VideoCodec != 1 || videoThumb.ImageCodec != 0 {
		t.Fatalf("video thumb compute demand should require VideoCodec=1, ImageCodec=0: got %+v", videoThumb)
	}
	audioWaveform := catalog.Demand(StepDerivativesComputeVideoFrame, MediaAudio)
	if audioWaveform.VideoCodec != 1 || audioWaveform.ImageCodec != 0 {
		t.Fatalf("audio waveform demand should require VideoCodec=1, ImageCodec=0: got %+v", audioWaveform)
	}

	// 2. Transcode compute CPU matches ToolSession.Threads
	transcodeCompute := catalog.Demand(StepTranscodeCompute, MediaVideo)
	if transcodeCompute.CPU != 3 || transcodeCompute.VideoCodec != 1 {
		t.Fatalf("transcode compute demand should match ToolSession.Threads=3, VideoCodec=1: got %+v", transcodeCompute)
	}
	hardwareCatalog := NewDemandCatalog(ToolSession{Threads: 3, SoftwarePreset: "faster", HardwareAccel: "vaapi"})
	if got := hardwareCatalog.Demand(StepTranscodeCompute, MediaAudio).CPU; got != 3 {
		t.Fatalf("hardware video session reduced software audio CPU demand to %d, want 3", got)
	}
	videoSemantic := catalog.Demand(StepEnrichComputeSemantic, MediaVideo)
	if videoSemantic.VideoCodec != 1 || videoSemantic.Inference != 1 || videoSemantic.ImageCodec != 0 {
		t.Fatalf("video semantic demand must reserve extraction and inference resources: %+v", videoSemantic)
	}

	// 3. Publishing steps hold DiskIO, not Codecs
	derivPublish := catalog.Demand(StepDerivativesPublish, MediaPhoto)
	if derivPublish.DiskIO != 1 || derivPublish.ImageCodec != 0 || derivPublish.VideoCodec != 0 {
		t.Fatalf("deriv publish should hold DiskIO=1 and zero codecs: got %+v", derivPublish)
	}

	transcodePublish := catalog.Demand(StepTranscodePublish, MediaVideo)
	if transcodePublish.DiskIO != 1 || transcodePublish.VideoCodec != 0 {
		t.Fatalf("transcode publish should hold DiskIO=1 and zero codecs: got %+v", transcodePublish)
	}

	// 4. Projection demands
	ocr := catalog.DemandForProjection("ocr")
	if ocr.DiskIO != 1 || ocr.CPU != 1 {
		t.Fatalf("ocr projection demand unexpected: %+v", ocr)
	}

	loc := catalog.DemandForProjection("location_resolution")
	if loc.DiskIO != 0 || loc.CPU != 1 {
		t.Fatalf("location projection demand unexpected: %+v", loc)
	}
}
