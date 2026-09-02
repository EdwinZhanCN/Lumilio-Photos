package execution

import (
	"context"
	"testing"
	"time"
)

// TestBaselinePhotoVideoConvoy locks the 2026-08-30 baseline bug where photo derivatives
// requested VideoCodec and therefore could not start while video transcoding was active,
// even though photo thumbnails only need ImageCodec.
func TestBaselinePhotoVideoConvoy(t *testing.T) {
	// Under ImageCodec >= 1 and VideoCodec = 1
	capacity := Resources{
		CPU:        4,
		DiskIO:     4,
		ImageCodec: 2,
		VideoCodec: 1,
	}
	gov, err := NewGovernor(capacity, 16)
	if err != nil {
		t.Fatal(err)
	}

	// 1. Transcode holds VideoCodec
	transcodeRes := Resources{CPU: 1, DiskIO: 1, VideoCodec: 1}
	releaseTranscode, err := gov.Acquire(context.Background(), ClassBackground, transcodeRes)
	if err != nil {
		t.Fatalf("acquire transcode: %v", err)
	}
	defer releaseTranscode()

	// 2. Today's photo-derivative vector (CPU + Disk + Image + Video)
	todaysPhotoVector := Resources{CPU: 1, DiskIO: 1, ImageCodec: 1, VideoCodec: 1}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = gov.Acquire(ctx, ClassBackground, todaysPhotoVector)
	if err == nil {
		t.Fatal("today's photo-derivative vector unexpectedly started while transcode held VideoCodec")
	}

	// 3. Desired Demand row for photo thumbnail compute (CPU + ImageCodec, NO VideoCodec)
	desiredPhotoCompute := Resources{CPU: 1, ImageCodec: 1}
	releasePhoto, err := gov.Acquire(context.Background(), ClassBackground, desiredPhotoCompute)
	if err != nil {
		t.Fatalf("desired photo compute vector failed to start while transcode held VideoCodec: %v", err)
	}
	releasePhoto()
}
