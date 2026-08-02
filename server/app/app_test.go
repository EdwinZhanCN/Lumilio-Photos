package app

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"server/config"
)

type fakeRiverStopper struct {
	stopErr       error
	forcedErr     error
	stopped       chan struct{}
	stopCalls     int
	forcedCalls   int
	closeOnForced bool
}

func (fake *fakeRiverStopper) Stop(context.Context) error {
	fake.stopCalls++
	return fake.stopErr
}

func (fake *fakeRiverStopper) StopAndCancel(context.Context) error {
	fake.forcedCalls++
	if fake.closeOnForced {
		close(fake.stopped)
		fake.closeOnForced = false
	}
	return fake.forcedErr
}

func (fake *fakeRiverStopper) Stopped() <-chan struct{} {
	return fake.stopped
}

func TestStopRiverQueueUsesForcedCancellationAfterDrainFailure(t *testing.T) {
	fake := &fakeRiverStopper{
		stopErr:       context.DeadlineExceeded,
		stopped:       make(chan struct{}),
		closeOnForced: true,
	}
	if err := stopRiverQueue(fake, time.Millisecond, time.Millisecond); err != nil {
		t.Fatalf("stopRiverQueue: %v", err)
	}
	if fake.stopCalls != 1 || fake.forcedCalls != 1 {
		t.Fatalf("stop calls = %d/%d, want 1/1", fake.stopCalls, fake.forcedCalls)
	}
}

func TestStopRiverQueueRequiresStoppedConfirmation(t *testing.T) {
	fake := &fakeRiverStopper{
		stopped:       make(chan struct{}),
		closeOnForced: true,
	}
	if err := stopRiverQueue(fake, time.Millisecond, time.Millisecond); err != nil {
		t.Fatalf("stopRiverQueue: %v", err)
	}
	if fake.stopCalls != 1 || fake.forcedCalls != 1 {
		t.Fatalf("unconfirmed graceful stop calls = %d/%d, want 1/1", fake.stopCalls, fake.forcedCalls)
	}
}

func TestStopRiverQueueRejectsUnconfirmedStop(t *testing.T) {
	fake := &fakeRiverStopper{
		stopErr:   context.DeadlineExceeded,
		forcedErr: context.DeadlineExceeded,
		stopped:   make(chan struct{}),
	}
	err := stopRiverQueue(fake, time.Millisecond, time.Millisecond)
	if err == nil {
		t.Fatal("stopRiverQueue accepted an unconfirmed stop")
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("stopRiverQueue error = %v", err)
	}
}

func TestPprofHostCanRestartOnSameAddress(t *testing.T) {
	first, err := startPprofHost("127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := first.server.Addr
	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	if err := first.shutdown(shutdownCtx); err != nil {
		cancel()
		t.Fatal(err)
	}
	cancel()

	second, err := startPprofHost(addr)
	if err != nil {
		t.Fatalf("restart pprof host on %s: %v", addr, err)
	}
	shutdownCtx, cancel = context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := second.shutdown(shutdownCtx); err != nil {
		t.Fatal(err)
	}
}

func TestRunRejectsStructLiteralConfig(t *testing.T) {
	err := Run(context.Background(), config.AppConfig{}, OperatorControls{})
	if err == nil || !strings.Contains(err.Error(), "strict manifest loader") {
		t.Fatalf("expected unvalidated config rejection, got %v", err)
	}
}

func TestProductURLUsesLoopbackForDesktopListeners(t *testing.T) {
	for _, test := range []struct {
		listen string
		want   string
	}{
		{listen: ":6680", want: "http://127.0.0.1:6680"},
		{listen: "0.0.0.0:6680", want: "http://127.0.0.1:6680"},
		{listen: "127.0.0.1:6680", want: "http://127.0.0.1:6680"},
	} {
		if got := productURL(test.listen); got != test.want {
			t.Fatalf("productURL(%q) = %q, want %q", test.listen, got, test.want)
		}
	}
}
