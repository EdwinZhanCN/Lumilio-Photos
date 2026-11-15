package execution

import (
	"testing"
)

func TestBudgetValidationAndDerivations(t *testing.T) {
	// 1. Minimum compute validation
	bZeroCompute := Budget{CPU: 0, MemoryBytes: 128 << 20}
	if err := bZeroCompute.Validate(); err == nil {
		t.Fatal("budget with 0 compute slots unexpectedly passed validation")
	}

	// 2. Minimum memory validation (at least 64 MiB)
	bLowMem := Budget{CPU: 2, MemoryBytes: 32 << 20}
	if err := bLowMem.Validate(); err == nil {
		t.Fatal("budget with <64 MiB memory unexpectedly passed validation")
	}

	// 3. Threads exceeding CPU
	bHighThreads := Budget{
		CPU:         2,
		MemoryBytes: 128 << 20,
		ToolSession: ToolSession{Threads: 4},
	}
	if err := bHighThreads.Validate(); err == nil {
		t.Fatal("budget with threads > CPU unexpectedly passed validation")
	}

	// 4. Macro worker derivation
	// CPU: 2, ImageCodec: 1, VideoCodec: 1, Inference: 1 => 2 * (2+1+1+1) = 10
	bDerive := Budget{
		CPU:         2,
		ImageCodec:  1,
		VideoCodec:  1,
		Inference:   1,
		MemoryBytes: 512 << 20,
		ToolSession: ToolSession{Threads: 2, SoftwarePreset: "veryfast"},
	}
	if got := bDerive.DerivedMacroWorkers(); got != 10 {
		t.Fatalf("DerivedMacroWorkers() = %d, want 10", got)
	}

	// Boundary caps: minimum 2, maximum 32
	if got := DeriveMacroWorkers(0, 0, 0, 0); got != 2 {
		t.Fatalf("DeriveMacroWorkers(0,0,0,0) = %d, want min 2", got)
	}
	if got := DeriveMacroWorkers(32, 16, 8, 4); got != 32 {
		t.Fatalf("DeriveMacroWorkers high capacities = %d, want max 32", got)
	}

	// 5. Governor creation
	gov, err := bDerive.Governor()
	if err != nil {
		t.Fatalf("Governor() failed: %v", err)
	}
	snap := gov.Snapshot()
	if snap.Capacity.CPU != 2 || snap.Capacity.ImageCodec != 1 || snap.QueueCapacity < 256 {
		t.Fatalf("Governor snapshot mismatch: %+v", snap)
	}
}
