package processors

import "testing"

func TestChooseFrameSamplingStrategy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		duration float64
		long     int
		want     frameSamplingStrategy
	}{
		{name: "very short", duration: 2, long: 300, want: frameStrategyMidpoint},
		{name: "short scene", duration: 30, long: 300, want: frameStrategyScene},
		{name: "long interval", duration: 600, long: 300, want: frameStrategyInterval},
		{name: "boundary short", duration: 4, long: 300, want: frameStrategyScene},
		{name: "boundary long", duration: 300, long: 300, want: frameStrategyInterval},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := chooseFrameSamplingStrategy(tc.duration, tc.long)
			if got != tc.want {
				t.Fatalf("chooseFrameSamplingStrategy(%v, %d) = %v, want %v", tc.duration, tc.long, got, tc.want)
			}
		})
	}
}

func TestSubsampleTimestamps(t *testing.T) {
	t.Parallel()

	in := []int32{0, 1000, 2000, 3000, 4000, 5000, 6000, 7000, 8000, 9000}
	got := subsampleTimestamps(in, 4)
	if len(got) != 4 {
		t.Fatalf("len = %d, want 4: %v", len(got), got)
	}
	if got[0] != 0 || got[len(got)-1] != 9000 {
		t.Fatalf("expected endpoints preserved, got %v", got)
	}

	if got := subsampleTimestamps(in, 20); len(got) != len(in) {
		t.Fatalf("no-op when max >= len: got %v", got)
	}
}

func TestUniformIntervalTimestamps(t *testing.T) {
	t.Parallel()

	got := uniformIntervalTimestamps(80, 8)
	if len(got) != 8 {
		t.Fatalf("len = %d, want 8", len(got))
	}
	if got[0] < 0 || got[len(got)-1] > 80_000 {
		t.Fatalf("timestamps out of range: %v", got)
	}
}

func TestParseShowinfoTimestamps(t *testing.T) {
	t.Parallel()

	stderr := `
[Parsed_showinfo_1 @ 0x] n:0 pts:0 pts_time:1.234 pos:123
[Parsed_showinfo_1 @ 0x] n:1 pts:100 pts_time:12.5 pos:456
`
	got := parseShowinfoTimestamps(stderr)
	if len(got) != 2 || got[0] != 1234 || got[1] != 12500 {
		t.Fatalf("got %v, want [1234 12500]", got)
	}
}
