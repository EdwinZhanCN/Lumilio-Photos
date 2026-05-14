package queue

import "testing"

func TestClampWorkers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value int
		min   int
		max   int
		want  int
	}{
		{name: "below minimum", value: 1, min: 2, max: 8, want: 2},
		{name: "within range", value: 6, min: 2, max: 8, want: 6},
		{name: "above maximum", value: 10, min: 2, max: 8, want: 8},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := clampWorkers(tt.value, tt.min, tt.max); got != tt.want {
				t.Fatalf("clampWorkers(%d, %d, %d) = %d, want %d", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}

func TestQueueWorkerCountsForCPU(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		cpuCount      int
		wantIngest    int
		wantThumbnail int
		wantPHash     int
	}{
		{name: "single cpu", cpuCount: 1, wantIngest: 2, wantThumbnail: 4, wantPHash: 1},
		{name: "four cpu", cpuCount: 4, wantIngest: 2, wantThumbnail: 4, wantPHash: 1},
		{name: "eight cpu", cpuCount: 8, wantIngest: 4, wantThumbnail: 8, wantPHash: 2},
		{name: "many cpu", cpuCount: 32, wantIngest: 8, wantThumbnail: 12, wantPHash: 4},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ingestWorkers, thumbnailWorkers, phashWorkers := queueWorkerCountsForCPU(tt.cpuCount)
			if ingestWorkers != tt.wantIngest {
				t.Fatalf("ingestWorkers = %d, want %d", ingestWorkers, tt.wantIngest)
			}
			if thumbnailWorkers != tt.wantThumbnail {
				t.Fatalf("thumbnailWorkers = %d, want %d", thumbnailWorkers, tt.wantThumbnail)
			}
			if phashWorkers != tt.wantPHash {
				t.Fatalf("phashWorkers = %d, want %d", phashWorkers, tt.wantPHash)
			}
			if thumbnailWorkers < ingestWorkers {
				t.Fatalf("thumbnailWorkers = %d, want >= ingestWorkers = %d", thumbnailWorkers, ingestWorkers)
			}
		})
	}
}
