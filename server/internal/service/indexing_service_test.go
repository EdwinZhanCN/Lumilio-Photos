package service

import "testing"

func TestNormalizeRequestedIndexingTasks_DefaultExcludesBioCLIP(t *testing.T) {
	tasks := normalizeRequestedIndexingTasks(nil)

	if containsIndexingTask(tasks, AssetIndexingTaskBioCLIP) {
		t.Fatalf("default indexing tasks should not include BioCLIP: %#v", tasks)
	}
}

func TestNormalizeRequestedIndexingTasks_IgnoresBioCLIP(t *testing.T) {
	tasks := normalizeRequestedIndexingTasks([]AssetIndexingTask{
		AssetIndexingTaskBioCLIP,
		AssetIndexingTaskOCR,
	})

	if containsIndexingTask(tasks, AssetIndexingTaskBioCLIP) {
		t.Fatalf("requested indexing tasks should not include BioCLIP: %#v", tasks)
	}
	if len(tasks) != 1 || tasks[0] != AssetIndexingTaskOCR {
		t.Fatalf("expected only OCR task, got %#v", tasks)
	}
}

func TestNormalizeReindexAssetsInput_OffsetClampedToNonNegative(t *testing.T) {
	cases := []struct {
		name   string
		offset int
		want   int
	}{
		{"negative zeroes", -50, 0},
		{"zero stays", 0, 0},
		{"positive preserved", 400, 400},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeReindexAssetsInput(ReindexAssetsInput{Offset: tc.offset})
			if got.Offset != tc.want {
				t.Fatalf("offset = %d, want %d", got.Offset, tc.want)
			}
		})
	}
}

func TestNormalizeReindexAssetsInput_LimitClamped(t *testing.T) {
	cases := []struct {
		name  string
		limit int
		want  int
	}{
		{"zero uses default", 0, defaultIndexingBatchSize},
		{"negative uses default", -1, defaultIndexingBatchSize},
		{"within range preserved", 300, 300},
		{"over max clamped", 9999, maxIndexingBatchSize},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := normalizeReindexAssetsInput(ReindexAssetsInput{Limit: tc.limit})
			if got.Limit != tc.want {
				t.Fatalf("limit = %d, want %d", got.Limit, tc.want)
			}
		})
	}
}

func TestNextReindexPageOffset(t *testing.T) {
	cases := []struct {
		name           string
		missingOnly    bool
		candidateCount int
		limit          int
		currentOffset  int
		wantOffset     int
		wantMore       bool
	}{
		{
			name:           "missing-only never pages regardless of batch size",
			missingOnly:    true,
			candidateCount: 200,
			limit:          200,
			currentOffset:  0,
			wantOffset:     0,
			wantMore:       false,
		},
		{
			name:           "full batch chains to next page",
			missingOnly:    false,
			candidateCount: 500,
			limit:          500,
			currentOffset:  0,
			wantOffset:     500,
			wantMore:       true,
		},
		{
			name:           "partial batch is the last page (no chain)",
			missingOnly:    false,
			candidateCount: 252,
			limit:          500,
			currentOffset:  500,
			wantOffset:     0,
			wantMore:       false,
		},
		{
			name:           "exact multiple still chains; empty next page stops via no_candidates",
			missingOnly:    false,
			candidateCount: 500,
			limit:          500,
			currentOffset:  500,
			wantOffset:     1000,
			wantMore:       true,
		},
		{
			name:           "second page advances offset by limit",
			missingOnly:    false,
			candidateCount: 200,
			limit:          200,
			currentOffset:  200,
			wantOffset:     400,
			wantMore:       true,
		},
		{
			name:           "non-positive limit does not chain",
			missingOnly:    false,
			candidateCount: 200,
			limit:          0,
			currentOffset:  0,
			wantOffset:     0,
			wantMore:       false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gotOffset, gotMore := nextReindexPageOffset(tc.missingOnly, tc.candidateCount, tc.limit, tc.currentOffset)
			if gotOffset != tc.wantOffset || gotMore != tc.wantMore {
				t.Fatalf("nextReindexPageOffset(%v, %d, %d, %d) = (%d, %v), want (%d, %v)",
					tc.missingOnly, tc.candidateCount, tc.limit, tc.currentOffset,
					gotOffset, gotMore, tc.wantOffset, tc.wantMore)
			}
		})
	}
}
