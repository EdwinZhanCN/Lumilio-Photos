package control

import "desktop/internal/control/dto"

// PresentRuntime and PresentLumen are the single source for labels and status
// dots. Tray and Settings consume the result instead of independently
// interpreting lifecycle fields.
func PresentRuntime(snapshot dto.RuntimeSnapshot) dto.ProcessPresentation {
	if snapshot.Ownership == dto.OwnershipHeld && snapshot.Phase == dto.RuntimeFailed {
		return dto.ProcessPresentation{Color: dto.DotRed, Label: "Cleanup Required"}
	}
	switch snapshot.Phase {
	case dto.RuntimeStarting:
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Starting"}
	case dto.RuntimeStopping:
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Stopping"}
	case dto.RuntimeRestarting:
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Restarting"}
	case dto.RuntimeSavingConfig:
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Saving Configuration"}
	case dto.RuntimeApplyingConfig:
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Applying Configuration"}
	case dto.RuntimeRunning:
		return dto.ProcessPresentation{Color: dto.DotGreen, Label: "Running"}
	}
	if !snapshot.Configured {
		// Action-oriented: the wizard is the only way out of this state.
		return dto.ProcessPresentation{Color: dto.DotGray, Label: "Setup required"}
	}
	if snapshot.DesiredState == dto.DesiredStopped {
		return dto.ProcessPresentation{Color: dto.DotGray, Label: "Stopped"}
	}
	if snapshot.Phase == dto.RuntimeFailed {
		return dto.ProcessPresentation{Color: dto.DotRed, Label: "Failed"}
	}
	return dto.ProcessPresentation{Color: dto.DotGray, Label: "Stopped"}
}

func PresentLumen(snapshot dto.LumenSnapshot) dto.ProcessPresentation {
	if snapshot.Ownership == dto.OwnershipHeld && snapshot.ProcessPhase == dto.LumenFailed {
		return dto.ProcessPresentation{Color: dto.DotRed, Label: "Cleanup Required"}
	}
	if snapshot.InstallPhase == dto.LumenInstalling {
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Installing"}
	}
	switch snapshot.ProcessPhase {
	case dto.LumenStarting:
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Starting"}
	case dto.LumenStopping:
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Stopping"}
	case dto.LumenBackoff:
		return dto.ProcessPresentation{Color: dto.DotYellow, Label: "Retrying"}
	case dto.LumenRunning:
		return dto.ProcessPresentation{Color: dto.DotGreen, Label: "Running"}
	}
	if snapshot.InstallPhase == dto.LumenAbsent {
		return dto.ProcessPresentation{Color: dto.DotGray, Label: "Not Installed"}
	}
	if snapshot.InstallPhase == dto.LumenInstallFailed {
		return dto.ProcessPresentation{Color: dto.DotGray, Label: "Install Failed"}
	}
	if snapshot.DesiredState == dto.DesiredDisabled {
		return dto.ProcessPresentation{Color: dto.DotGray, Label: "Disabled"}
	}
	if snapshot.ProcessPhase == dto.LumenFailed {
		return dto.ProcessPresentation{Color: dto.DotRed, Label: "Failed"}
	}
	return dto.ProcessPresentation{Color: dto.DotGray, Label: "Stopped"}
}

func ProjectCapabilities(snapshot dto.DesktopSnapshot) dto.DesktopSnapshot {
	snapshot.Runtime.Presentation = PresentRuntime(snapshot.Runtime)
	snapshot.Lumen.Presentation = PresentLumen(snapshot.Lumen)
	snapshot.Runtime.Capabilities = runtimeCapabilities(snapshot)
	snapshot.Lumen.Capabilities = lumenCapabilities(snapshot)

	capabilities := dto.Capabilities{
		CanOpenProduct:               snapshot.Runtime.Capabilities.CanOpenProduct,
		CanStartRuntime:              snapshot.Runtime.Capabilities.CanStartRuntime,
		CanStopRuntime:               snapshot.Runtime.Capabilities.CanStopRuntime,
		CanRetryCleanupRuntime:       snapshot.Runtime.Capabilities.CanRetryCleanupRuntime,
		CanRestartRuntime:            snapshot.Runtime.Capabilities.CanRestartRuntime,
		CanStartLumen:                snapshot.Lumen.Capabilities.CanStartLumen,
		CanStopLumen:                 snapshot.Lumen.Capabilities.CanStopLumen,
		CanRetryCleanupLumen:         snapshot.Lumen.Capabilities.CanRetryCleanupLumen,
		CanRestartLumen:              snapshot.Lumen.Capabilities.CanRestartLumen,
		CanResumeAfterFailedShutdown: snapshot.Host.Shutdown.Phase == dto.ShutdownFailed,
		CanRequestQuit:               snapshot.Host.Shutdown.Phase == dto.ShutdownIdle || snapshot.Host.Shutdown.Phase == dto.ShutdownFailed,
		CanApplyUpdate:               snapshot.Update.CanApply,
	}
	snapshot.Runtime.Capabilities.CanOpenProduct = capabilities.CanOpenProduct
	snapshot.Runtime.Capabilities.CanStartRuntime = capabilities.CanStartRuntime
	snapshot.Runtime.Capabilities.CanStopRuntime = capabilities.CanStopRuntime
	snapshot.Runtime.Capabilities.CanRetryCleanupRuntime = capabilities.CanRetryCleanupRuntime
	snapshot.Runtime.Capabilities.CanRestartRuntime = capabilities.CanRestartRuntime
	snapshot.Lumen.Capabilities.CanStartLumen = capabilities.CanStartLumen
	snapshot.Lumen.Capabilities.CanStopLumen = capabilities.CanStopLumen
	snapshot.Lumen.Capabilities.CanRetryCleanupLumen = capabilities.CanRetryCleanupLumen
	snapshot.Lumen.Capabilities.CanRestartLumen = capabilities.CanRestartLumen
	if snapshot.Host.Recovery.Code != "" {
		applyRecoveryOverlay(&capabilities, snapshot)
	}
	applyShutdownOverlay(&capabilities, snapshot.Host.Shutdown.Phase)
	snapshot.Runtime.Capabilities.CanOpenProduct = capabilities.CanOpenProduct
	snapshot.Runtime.Capabilities.CanStartRuntime = capabilities.CanStartRuntime
	snapshot.Runtime.Capabilities.CanStopRuntime = capabilities.CanStopRuntime
	snapshot.Runtime.Capabilities.CanRetryCleanupRuntime = capabilities.CanRetryCleanupRuntime
	snapshot.Runtime.Capabilities.CanRestartRuntime = capabilities.CanRestartRuntime
	snapshot.Lumen.Capabilities.CanStartLumen = capabilities.CanStartLumen
	snapshot.Lumen.Capabilities.CanStopLumen = capabilities.CanStopLumen
	snapshot.Lumen.Capabilities.CanRetryCleanupLumen = capabilities.CanRetryCleanupLumen
	snapshot.Lumen.Capabilities.CanRestartLumen = capabilities.CanRestartLumen
	snapshot.Runtime.Capabilities.CanApplyUpdate = capabilities.CanApplyUpdate
	snapshot.Lumen.Capabilities.CanApplyUpdate = capabilities.CanApplyUpdate
	return snapshot
}

func applyRecoveryOverlay(capabilities *dto.Capabilities, snapshot dto.DesktopSnapshot) {
	capabilities.CanOpenProduct = false
	capabilities.CanStartRuntime = false
	capabilities.CanRestartRuntime = false
	capabilities.CanRestartLumen = false
	capabilities.CanApplyUpdate = false
	// Recovery must not strand an already-owned process. It removes ordinary
	// lifecycle actions, but retains the one safe cleanup action for each
	// aggregate so the user can release ownership before repairing state.
	capabilities.CanStopRuntime = snapshot.Runtime.Ownership == dto.OwnershipHeld && snapshot.Runtime.Phase == dto.RuntimeRunning
	capabilities.CanRetryCleanupRuntime = snapshot.Runtime.Ownership == dto.OwnershipHeld && snapshot.Runtime.Phase == dto.RuntimeFailed
	capabilities.CanStartLumen = false
	capabilities.CanStopLumen = snapshot.Lumen.Ownership == dto.OwnershipHeld && snapshot.Lumen.ProcessPhase == dto.LumenRunning
	capabilities.CanRetryCleanupLumen = snapshot.Lumen.Ownership == dto.OwnershipHeld && snapshot.Lumen.ProcessPhase == dto.LumenFailed
}

func applyShutdownOverlay(capabilities *dto.Capabilities, phase dto.ShutdownPhase) {
	switch phase {
	case dto.ShutdownQuiescing:
		capabilities.CanOpenProduct = false
		capabilities.CanStartRuntime = false
		capabilities.CanStopRuntime = false
		capabilities.CanRetryCleanupRuntime = false
		capabilities.CanRestartRuntime = false
		capabilities.CanStartLumen = false
		capabilities.CanStopLumen = false
		capabilities.CanRetryCleanupLumen = false
		capabilities.CanRestartLumen = false
		capabilities.CanRequestQuit = false
		capabilities.CanApplyUpdate = false
	case dto.ShutdownFailed:
		capabilities.CanOpenProduct = false
		capabilities.CanStartRuntime = false
		capabilities.CanRestartRuntime = false
		capabilities.CanStartLumen = false
		capabilities.CanRestartLumen = false
		capabilities.CanRequestQuit = true
		capabilities.CanApplyUpdate = false
	case dto.ShutdownArmed:
		*capabilities = dto.Capabilities{}
	}
}

func runtimeCapabilities(snapshot dto.DesktopSnapshot) dto.Capabilities {
	runtime := snapshot.Runtime
	capabilities := dto.Capabilities{}
	if runtime.Ownership == dto.OwnershipHeld && runtime.Phase == dto.RuntimeFailed {
		capabilities.CanRetryCleanupRuntime = true
	} else if runtime.Phase == dto.RuntimeRunning {
		capabilities.CanStopRuntime = true
		capabilities.CanRestartRuntime = runtime.DesiredState == dto.DesiredRunning
	} else if runtime.Configured && (runtime.Phase == dto.RuntimeStopped || (runtime.Phase == dto.RuntimeFailed && runtime.Ownership == dto.OwnershipNone)) {
		capabilities.CanStartRuntime = true
	}
	capabilities.CanOpenProduct = runtime.Phase == dto.RuntimeRunning && runtime.ProductURL != ""
	return capabilities
}

func lumenCapabilities(snapshot dto.DesktopSnapshot) dto.Capabilities {
	lumen := snapshot.Lumen
	capabilities := dto.Capabilities{}
	if lumen.Ownership == dto.OwnershipHeld && lumen.ProcessPhase == dto.LumenFailed {
		capabilities.CanRetryCleanupLumen = true
	} else if lumen.ProcessPhase == dto.LumenRunning {
		capabilities.CanStopLumen = true
		capabilities.CanRestartLumen = lumen.DesiredState == dto.DesiredRunning
	} else if lumen.ProcessAvailable && lumen.InstallPhase == dto.LumenInstalled && (lumen.ProcessPhase == dto.LumenStopped || (lumen.ProcessPhase == dto.LumenFailed && lumen.Ownership == dto.OwnershipNone)) {
		capabilities.CanStartLumen = true
	}
	return capabilities
}
