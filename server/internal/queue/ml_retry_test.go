package queue

import (
	"errors"
	"testing"

	"github.com/riverqueue/river"
)

func TestMaybeSnoozeMLInfraError(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		err        error
		wantSnooze bool
	}{
		{
			name:       "select node failure snoozes",
			err:        errors.New("failed to perform face detection: failed to infer face embedding: failed after 0 retries: failed to select node: no suitable nodes available for service insightface task face_recognition"),
			wantSnooze: true,
		},
		{
			name:       "task unavailable snoozes",
			err:        errors.New("semantic_image_embed task not available"),
			wantSnooze: true,
		},
		{
			name:       "ordinary error does not snooze",
			err:        errors.New("failed to save OCR results: duplicate key value violates unique constraint"),
			wantSnooze: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := maybeSnoozeMLInfraError(tt.err)
			if tt.wantSnooze {
				var snoozeErr *river.JobSnoozeError
				if !errors.As(got, &snoozeErr) {
					t.Fatalf("expected snooze error, got %v", got)
				}
				if snoozeErr.Duration != mlInfraSnoozeDuration {
					t.Fatalf("expected snooze duration %s, got %s", mlInfraSnoozeDuration, snoozeErr.Duration)
				}
				return
			}

			if !errors.Is(got, tt.err) || got.Error() != tt.err.Error() {
				t.Fatalf("expected original error, got %v", got)
			}
		})
	}
}
