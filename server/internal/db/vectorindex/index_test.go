package vectorindex

import "testing"

func TestANNBucketsRespectTrainingPopulation(t *testing.T) {
	tests := []struct {
		rows    int64
		samples int64
		want    int
	}{
		{rows: 5_000, samples: 5_000, want: 70},
		{rows: 1_000_000, samples: 100_000, want: 1_000},
		{rows: 16, samples: 16, want: 4},
		{rows: 100_000_000, samples: 1_000, want: 250},
	}
	for _, test := range tests {
		if got := annBuckets(test.rows, test.samples); got != test.want {
			t.Errorf(
				"annBuckets(%d, %d) = %d, want %d",
				test.rows,
				test.samples,
				got,
				test.want,
			)
		}
	}
}
