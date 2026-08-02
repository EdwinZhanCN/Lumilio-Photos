package control

import (
	"testing"

	"desktop/internal/control/dto"
)

func TestRuntimePresentationUsesFirstMatchOrder(t *testing.T) {
	cases := []struct {
		name  string
		input dto.RuntimeSnapshot
		color dto.DotColor
		label string
	}{
		{"retained failure", dto.RuntimeSnapshot{Configured: true, Phase: dto.RuntimeFailed, Ownership: dto.OwnershipHeld}, dto.DotRed, "Cleanup Required"},
		{"transition", dto.RuntimeSnapshot{Configured: true, Phase: dto.RuntimeStarting}, dto.DotYellow, "Starting"},
		{"running", dto.RuntimeSnapshot{Configured: true, Phase: dto.RuntimeRunning}, dto.DotGreen, "Running"},
		{"not configured", dto.RuntimeSnapshot{Phase: dto.RuntimeStopped}, dto.DotGray, "Setup required"},
		{"stopped", dto.RuntimeSnapshot{Configured: true, DesiredState: dto.DesiredStopped, Phase: dto.RuntimeStopped}, dto.DotGray, "Stopped"},
		{"failed", dto.RuntimeSnapshot{Configured: true, DesiredState: dto.DesiredRunning, Phase: dto.RuntimeFailed}, dto.DotRed, "Failed"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			got := PresentRuntime(test.input)
			if got.Color != test.color || got.Label != test.label {
				t.Fatalf("got %+v, want %s/%s", got, test.color, test.label)
			}
		})
	}
}

func TestCapabilitiesApplyShutdownOverlay(t *testing.T) {
	snapshot := dto.InitialSnapshot("instance")
	snapshot.Runtime.Configured = true
	snapshot.Runtime.Phase = dto.RuntimeRunning
	snapshot.Runtime.DesiredState = dto.DesiredRunning
	snapshot.Runtime.ProductURL = "http://localhost:6680"
	snapshot.Host.Shutdown.Phase = dto.ShutdownQuiescing
	projected := ProjectCapabilities(snapshot)
	if projected.Runtime.Capabilities.CanStopRuntime || projected.Runtime.Capabilities.CanOpenProduct {
		t.Fatalf("quiescing runtime capabilities were not disabled: %+v", projected.Runtime.Capabilities)
	}
	if projected.Host.Shutdown.Phase != dto.ShutdownQuiescing {
		t.Fatal("projection changed shutdown state")
	}
}

func TestRecoveryOverlayRetainsSafeCleanup(t *testing.T) {
	snapshot := dto.InitialSnapshot("instance")
	snapshot.Runtime.Configured = true
	snapshot.Runtime.Phase = dto.RuntimeFailed
	snapshot.Runtime.Ownership = dto.OwnershipHeld
	snapshot.Lumen.InstallPhase = dto.LumenInstalled
	snapshot.Lumen.ProcessPhase = dto.LumenFailed
	snapshot.Lumen.Ownership = dto.OwnershipHeld
	snapshot.Host.Recovery = dto.Error{Code: dto.ErrorRecoveryRequired, Message: "repair required"}

	projected := ProjectCapabilities(snapshot)
	if !projected.Runtime.Capabilities.CanRetryCleanupRuntime || !projected.Lumen.Capabilities.CanRetryCleanupLumen {
		t.Fatalf("recovery removed retained-owner cleanup: runtime=%+v lumen=%+v", projected.Runtime.Capabilities, projected.Lumen.Capabilities)
	}
	if projected.Runtime.Capabilities.CanStartRuntime || projected.Runtime.Capabilities.CanRestartRuntime || projected.Lumen.Capabilities.CanStartLumen || projected.Lumen.Capabilities.CanRestartLumen {
		t.Fatalf("recovery left ordinary lifecycle actions enabled: runtime=%+v lumen=%+v", projected.Runtime.Capabilities, projected.Lumen.Capabilities)
	}
}
