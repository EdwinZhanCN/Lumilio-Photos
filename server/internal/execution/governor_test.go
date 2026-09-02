package execution

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

func TestGovernorBoundsAndCancellationRelease(t *testing.T) {
	g, err := NewGovernor(Resources{CPU: 1, MemoryBytes: 100}, 2)
	if err != nil {
		t.Fatal(err)
	}
	release, err := g.Acquire(context.Background(), ClassBackground, Resources{CPU: 1, MemoryBytes: 100})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err = g.Acquire(ctx, ClassBackground, Resources{CPU: 1})
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("Acquire error = %v", err)
	}
	release()
	release()
	snapshot := g.Snapshot()
	if snapshot.InUse.CPU != 0 || snapshot.Waiting != 0 || snapshot.Canceled != 1 {
		t.Fatalf("snapshot = %+v", snapshot)
	}
}

func TestGovernorHonorsAlreadyCanceledContext(t *testing.T) {
	g, err := NewGovernor(Resources{CPU: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	release, err := g.Acquire(ctx, ClassBackground, Resources{CPU: 1})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Acquire error = %v, want context canceled", err)
	}
	if release != nil {
		t.Fatal("Acquire returned a release function after cancellation")
	}
	if snapshot := g.Snapshot(); snapshot.InUse.CPU != 0 || snapshot.Canceled != 1 {
		t.Fatalf("snapshot = %+v, want no reservation and one cancellation", snapshot)
	}
}

func TestEngineReleasesAfterPanic(t *testing.T) {
	g, _ := NewGovernor(Resources{CPU: 1}, 1)
	engine := NewEngine(g)
	err := engine.Run(context.Background(), ClassBackground, Resources{CPU: 1}, func(context.Context) error { panic("boom") })
	if err == nil || g.Snapshot().InUse.CPU != 0 {
		t.Fatalf("err=%v snapshot=%+v", err, g.Snapshot())
	}
}

func TestGovernorBypassesOnlyIndependentBlockedResources(t *testing.T) {
	g, err := NewGovernor(Resources{CPU: 2, DiskIO: 1, VideoCodec: 1}, 4)
	if err != nil {
		t.Fatal(err)
	}
	holdCPU, err := g.Acquire(context.Background(), ClassBackground, Resources{CPU: 1})
	if err != nil {
		t.Fatal(err)
	}

	type acquired struct {
		name    string
		release func()
	}
	results := make(chan acquired, 3)
	var group sync.WaitGroup
	acquire := func(name string, resources Resources) {
		group.Add(1)
		go func() {
			defer group.Done()
			release, acquireErr := g.Acquire(context.Background(), ClassBackground, resources)
			if acquireErr != nil {
				t.Errorf("%s acquire: %v", name, acquireErr)
				return
			}
			results <- acquired{name: name, release: release}
		}()
	}

	// The head needs both CPU slots and therefore cannot run while holdCPU is
	// active. A disk-only follower is independent and should proceed. A later
	// CPU request must not leapfrog the head and starve it.
	acquire("head", Resources{CPU: 2})
	deadline := time.Now().Add(time.Second)
	for g.Snapshot().Waiting != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	acquire("disk", Resources{DiskIO: 1})
	acquire("cpu-follower", Resources{CPU: 1})

	select {
	case got := <-results:
		if got.name != "disk" {
			got.release()
			t.Fatalf("first admitted waiter = %q, want independent disk waiter", got.name)
		}
		got.release()
	case <-time.After(time.Second):
		t.Fatal("independent disk waiter was blocked behind CPU head")
	}

	select {
	case got := <-results:
		got.release()
		t.Fatalf("CPU follower bypassed blocked CPU head: %q", got.name)
	case <-time.After(20 * time.Millisecond):
	}

	holdCPU()
	first := <-results
	if first.name != "head" {
		first.release()
		t.Fatalf("first CPU waiter after release = %q, want head", first.name)
	}
	first.release()
	second := <-results
	if second.name != "cpu-follower" {
		second.release()
		t.Fatalf("second CPU waiter = %q, want follower", second.name)
	}
	second.release()
	group.Wait()
}

func TestGovernorClassPriorityBypass(t *testing.T) {
	// 2 CPU slots total.
	g, err := NewGovernor(Resources{CPU: 2}, 4)
	if err != nil {
		t.Fatal(err)
	}
	// Hold 1 CPU slot.
	holdCPU, err := g.Acquire(context.Background(), ClassBackground, Resources{CPU: 1})
	if err != nil {
		t.Fatal(err)
	}

	type acquired struct {
		name    string
		release func()
	}
	results := make(chan acquired, 2)
	var group sync.WaitGroup
	acquire := func(name string, class Class, resources Resources) {
		group.Add(1)
		go func() {
			defer group.Done()
			release, acquireErr := g.Acquire(context.Background(), class, resources)
			if acquireErr != nil {
				t.Errorf("%s acquire: %v", name, acquireErr)
				return
			}
			results <- acquired{name: name, release: release}
		}()
	}

	// 1. Background head asks for 2 CPU slots (blocked because 1 is held).
	acquire("bg-head", ClassBackground, Resources{CPU: 2})
	deadline := time.Now().Add(time.Second)
	for g.Snapshot().Waiting != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}

	// 2. Interactive waiter asks for 1 CPU slot (which fits in available capacity).
	// Because interactive > background, it should bypass the blocked background head.
	acquire("interactive-follower", ClassInteractive, Resources{CPU: 1})

	select {
	case got := <-results:
		if got.name != "interactive-follower" {
			got.release()
			t.Fatalf("first admitted waiter = %q, want interactive follower", got.name)
		}
		got.release()
	case <-time.After(time.Second):
		t.Fatal("interactive waiter failed to bypass blocked background head")
	}

	// Release held CPU so bg-head can finish.
	holdCPU()
	head := <-results
	if head.name != "bg-head" {
		head.release()
		t.Fatalf("waiter after release = %q, want bg-head", head.name)
	}
	head.release()
	group.Wait()
}

func TestGovernorSnapshotReportsBoundedResourceWaitPercentiles(t *testing.T) {
	g, err := NewGovernor(Resources{CPU: 1, DiskIO: 1, MemoryBytes: 64}, 2)
	if err != nil {
		t.Fatal(err)
	}
	hold, err := g.Acquire(context.Background(), ClassBackground, Resources{CPU: 1, MemoryBytes: 64})
	if err != nil {
		t.Fatal(err)
	}

	acquired := make(chan func(), 1)
	go func() {
		release, acquireErr := g.Acquire(context.Background(), ClassBackground, Resources{CPU: 1, MemoryBytes: 32})
		if acquireErr != nil {
			t.Errorf("queued acquire: %v", acquireErr)
			return
		}
		acquired <- release
	}()
	deadline := time.Now().Add(time.Second)
	for g.Snapshot().Waiting != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	queued := g.Snapshot()
	if queued.QueueCapacity != 2 || queued.Waiting != 1 || queued.PeakWaiting != 1 {
		hold()
		t.Fatalf("queued snapshot = %+v", queued)
	}
	if queued.WaitingResources.CPU != 1 || queued.WaitingResources.MemoryBytes != 32 {
		hold()
		t.Fatalf("waiting resources = %+v", queued.WaitingResources)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	_, err = g.Acquire(ctx, ClassBackground, Resources{CPU: 1})
	cancel()
	if !errors.Is(err, context.DeadlineExceeded) {
		hold()
		t.Fatalf("canceled acquire error = %v", err)
	}
	hold()
	release := <-acquired
	release()

	snapshot := g.Snapshot()
	if snapshot.Wait.Count != 2 || snapshot.WaitByResource.CPU.Count != 2 || snapshot.WaitByResource.Memory.Count != 1 {
		t.Fatalf("resource wait snapshot = %+v", snapshot)
	}
	if snapshot.Wait.P50 > snapshot.Wait.P95 || snapshot.Wait.P95 > snapshot.Wait.P99 || snapshot.Wait.P99 > snapshot.Wait.Max {
		t.Fatalf("invalid resource wait percentiles = %+v", snapshot.Wait)
	}
	if snapshot.Waiting != 0 || snapshot.InUse != (Resources{}) || snapshot.Canceled != 1 || snapshot.Rejected != 0 {
		t.Fatalf("drained governor snapshot = %+v", snapshot)
	}
}

func TestGovernorRejectsAboveBoundWithoutRetainingWaiter(t *testing.T) {
	g, err := NewGovernor(Resources{CPU: 1}, 1)
	if err != nil {
		t.Fatal(err)
	}
	hold, err := g.Acquire(context.Background(), ClassBackground, Resources{CPU: 1})
	if err != nil {
		t.Fatal(err)
	}
	defer hold()

	waiting := make(chan error, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, acquireErr := g.Acquire(ctx, ClassBackground, Resources{CPU: 1})
		waiting <- acquireErr
	}()
	deadline := time.Now().Add(time.Second)
	for g.Snapshot().Waiting != 1 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	for range 128 {
		if _, acquireErr := g.Acquire(context.Background(), ClassBackground, Resources{CPU: 1}); !errors.Is(acquireErr, ErrWaitQueueFull) {
			t.Fatalf("full queue acquire error = %v", acquireErr)
		}
	}
	snapshot := g.Snapshot()
	if snapshot.Waiting != 1 || snapshot.PeakWaiting != 1 || snapshot.Rejected != 128 {
		t.Fatalf("bounded queue snapshot = %+v", snapshot)
	}
	cancel()
	if err := <-waiting; !errors.Is(err, context.Canceled) {
		t.Fatalf("queued acquire error = %v", err)
	}
}
