package service

import "testing"

type stubTaskAvailabilityChecker map[string]bool

func (s stubTaskAvailabilityChecker) IsTaskAvailable(taskName string) bool {
	return s[taskName]
}

func TestIsIndexingTaskRuntimeAvailable(t *testing.T) {
	t.Run("clip requires image embedding runtime task", func(t *testing.T) {
		checker := stubTaskAvailabilityChecker{
			"clip_image_embed": true,
		}

		if !IsIndexingTaskRuntimeAvailable(checker, AssetIndexingTaskClip) {
			t.Fatal("expected clip task to be available when image embedding runtime is present")
		}
	})

	t.Run("single runtime tasks map directly", func(t *testing.T) {
		checker := stubTaskAvailabilityChecker{
			"ocr":              true,
			"bioclip_classify": true,
		}

		if !IsIndexingTaskRuntimeAvailable(checker, AssetIndexingTaskOCR) {
			t.Fatal("expected OCR task to be available")
		}
		if !IsIndexingTaskRuntimeAvailable(checker, AssetIndexingTaskBioCLIP) {
			t.Fatal("expected BioCLIP task to be available")
		}
		if IsIndexingTaskRuntimeAvailable(checker, AssetIndexingTaskFace) {
			t.Fatal("expected face task to be unavailable")
		}
	})

	t.Run("nil checker disables runtime-dependent enqueueing", func(t *testing.T) {
		if IsIndexingTaskRuntimeAvailable(nil, AssetIndexingTaskCaption) {
			t.Fatal("expected nil runtime checker to report unavailable")
		}
	})
}

func TestFilterRuntimeAvailableIndexingTasks(t *testing.T) {
	checker := stubTaskAvailabilityChecker{
		"clip_image_embed":      true,
		"bioclip_classify":      true,
		"face_detect_and_embed": false,
		"ocr":                   true,
	}

	filtered := FilterRuntimeAvailableIndexingTasks([]AssetIndexingTask{
		AssetIndexingTaskClip,
		AssetIndexingTaskBioCLIP,
		AssetIndexingTaskFace,
		AssetIndexingTaskOCR,
	}, checker)

	if len(filtered) != 3 {
		t.Fatalf("expected 3 available tasks, got %d", len(filtered))
	}
	if filtered[0] != AssetIndexingTaskClip || filtered[1] != AssetIndexingTaskBioCLIP || filtered[2] != AssetIndexingTaskOCR {
		t.Fatalf("unexpected filtered task order: %#v", filtered)
	}
}
