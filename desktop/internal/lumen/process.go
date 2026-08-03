package lumen

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
)

// ReadinessProbe performs the Hub's protocol-level version/profile handshake.
// A TCP listener alone is not sufficient evidence of ownership or readiness.
type ReadinessProbe func(context.Context, string, string) error

// ExecFactory is the production process boundary. The installed artifact is
// selected by the signed/current pointer supplied by the host; PATH lookup is
// deliberately impossible. The Hub must implement the parent-liveness and
// launch-token contract described by the Desktop plan.
type ExecFactory struct {
	Binary       string
	Args         []string
	WorkDir      string
	OwnerLock    string
	Profile      string
	Endpoint     string
	Probe        ReadinessProbe
	ProbeTimeout time.Duration
}

func (f ExecFactory) Start(parent context.Context, id uint64, profile string) (Process, error) {
	if strings.TrimSpace(f.Binary) == "" {
		return Process{}, operationError(dto.ErrorLumenNotInstalled, "Lumen executable is not installed")
	}
	if f.Probe == nil {
		return Process{}, operationError(dto.ErrorRuntimeNotReady, "Lumen readiness handshake is unavailable")
	}
	owner, err := AcquireOwnerLock(f.OwnerLock)
	if err != nil {
		if errors.Is(err, ErrOwnerBusy) {
			return Process{}, operationError(dto.ErrorLumenOwnerBusy, "another Lumen supervisor owns this installation")
		}
		return Process{}, err
	}

	ctx, cancel := context.WithCancel(parent)
	args := append([]string(nil), f.Args...)
	cmd := exec.Command(f.Binary, args...)
	cmd.Dir = f.WorkDir
	cmd.Env = append(os.Environ(),
		"LUMILIO_LAUNCH_TOKEN="+owner.Token(),
		"LUMILIO_PROFILE="+profile,
		"LUMILIO_PARENT_LIVENESS=required",
		// The Desktop Hub binds loopback. Advertising that same address keeps
		// existing Desktop manifests (which predate the static node) working
		// through mDNS without exposing inference on a LAN interface.
		"ADVERTISE_IP=127.0.0.1",
	)
	configureProcessGroup(cmd)
	closeChildLiveness, closeParentLiveness, err := attachParentLiveness(cmd)
	if err != nil {
		cancel()
		_ = owner.Close()
		return Process{}, err
	}
	if err := cmd.Start(); err != nil {
		_ = closeChildLiveness()
		_ = closeParentLiveness()
		cancel()
		_ = owner.Close()
		return Process{}, err
	}
	releaseProcessGroup, err := attachProcessGroup(cmd)
	if err != nil {
		terminateProcessTree(cmd)
		_ = closeChildLiveness()
		_ = closeParentLiveness()
		cancel()
		_ = owner.Close()
		return Process{}, err
	}

	done := make(chan error, 1)
	ready := make(chan ReadyInfo, 1)
	var waitOnce sync.Once
	finish := func() {
		waitOnce.Do(func() {
			err := cmd.Wait()
			cancel()
			_ = releaseProcessGroup()
			_ = closeParentLiveness()
			_ = owner.Close()
			done <- err
			close(done)
		})
	}
	_ = closeChildLiveness()
	// Wait immediately so an unexpected child exit is observable through Done.
	// Cancellation paths also call finish; waitOnce keeps the single Wait owner.
	go finish()
	stop := func() {
		cancel()
		terminateProcessTree(cmd)
	}
	go func() {
		<-ctx.Done()
		terminateProcessTree(cmd)
		finish()
	}()
	go func() {
		probeCtx := ctx
		if f.ProbeTimeout > 0 {
			var probeCancel context.CancelFunc
			probeCtx, probeCancel = context.WithTimeout(ctx, f.ProbeTimeout)
			defer probeCancel()
		}
		if err := f.Probe(probeCtx, profile, owner.Token()); err != nil {
			cancel()
			terminateProcessTree(cmd)
			return
		}
		select {
		case ready <- ReadyInfo{Endpoint: f.Endpoint}:
		case <-ctx.Done():
		}
	}()
	_ = id // the process identity is tracked by Controller's Process.ID.
	return Process{ID: id, Cancel: stop, Lifetime: ctx, Done: done, Ready: ready, Profile: profile}, nil
}

func operationError(code dto.ErrorCode, message string) error {
	return operation.NewError(code, message)
}

var _ Factory = ExecFactory{}
