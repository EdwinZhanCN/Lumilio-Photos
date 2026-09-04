package queue

import (
	"server/internal/queue/jobs"
	"testing"
)

func TestRuntimeQueueIsSingleBoundedMacroAdmissionLane(t *testing.T) {
	counts := RuntimeMacroWorkerCounts(11)
	if len(counts) != 1 || counts[jobs.QueueMacro] != 11 {
		t.Fatalf("counts=%v", counts)
	}
	counts[jobs.QueueMacro] = 99
	if RuntimeMacroWorkerCounts(11)[jobs.QueueMacro] == 99 {
		t.Fatal("runtime macro worker map is shared")
	}
}
