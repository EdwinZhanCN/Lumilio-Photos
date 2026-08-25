package outbox

import "testing"

func TestOutboxHasMoreKeepsOneJobForFullOrRetryingPage(t *testing.T) {
	for _, test := range []struct {
		name      string
		claimed   int
		batchSize int64
		retrying  int
		want      bool
	}{
		{name: "short drained page", claimed: 3, batchSize: 32, want: false},
		{name: "full page", claimed: 32, batchSize: 32, want: true},
		{name: "short page with retry", claimed: 3, batchSize: 32, retrying: 1, want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := outboxHasMore(test.claimed, test.batchSize, test.retrying); got != test.want {
				t.Fatalf("outboxHasMore(%d, %d, %d) = %t, want %t", test.claimed, test.batchSize, test.retrying, got, test.want)
			}
		})
	}
}
