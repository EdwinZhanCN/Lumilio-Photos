package panel

import (
	"strconv"

	"desktop/internal/control/dto"
	"desktop/internal/locales"
	"desktop/internal/operation"
)

// errorText says a control-plane error in the app's language. An error
// without a stable code keeps its own text.
func errorText(err error) string {
	if err == nil {
		return ""
	}
	if text := codeText(operation.ErrorCodeOf(err)); text != "" {
		return text
	}
	return err.Error()
}

func operationFailure(item dto.OperationSnapshot) string {
	if text := codeText(item.Error.Code); text != "" {
		return text
	}
	if item.Error.Message != "" {
		return item.Error.Message
	}
	return locales.ErrorsUnknown()
}

// codeText maps every stable control-plane code to its copy. The test
// TestEveryErrorCodeHasCopy fails when a code lands without one.
func codeText(code dto.ErrorCode) string {
	switch code {
	case dto.ErrorInvalidArgument:
		return locales.ErrorsInvalidArgument()
	case dto.ErrorStaleVersion:
		return locales.ErrorsStaleVersion()
	case dto.ErrorStaleConfig:
		return locales.ErrorsStaleConfig()
	case dto.ErrorControllerBusy:
		return locales.ErrorsControllerBusy()
	case dto.ErrorOperationConflict:
		return locales.ErrorsOperationConflict()
	case dto.ErrorOperationNotCancellable:
		return locales.ErrorsOperationNotCancellable()
	case dto.ErrorRuntimeNotConfigured:
		return locales.ErrorsRuntimeNotConfigured()
	case dto.ErrorRuntimeNotReady:
		return locales.ErrorsRuntimeNotReady()
	case dto.ErrorRepositoryControlUnavailable:
		return locales.ErrorsRepositoryControlUnavailable()
	case dto.ErrorStorageLocationOffline:
		return locales.ErrorsStorageLocationOffline()
	case dto.ErrorStopTimeout:
		return locales.ErrorsStopTimeout()
	case dto.ErrorReadinessTimeout:
		return locales.ErrorsReadinessTimeout()
	case dto.ErrorSignatureInvalid:
		return locales.ErrorsSignatureInvalid()
	case dto.ErrorShutdownInProgress:
		return locales.ErrorsShutdownInProgress()
	case dto.ErrorLumenNotInstalled:
		return locales.ErrorsLumenNotInstalled()
	case dto.ErrorLumenOwnerBusy:
		return locales.ErrorsLumenOwnerBusy()
	case dto.ErrorRecoveryRequired:
		return locales.ErrorsRecoveryRequired()
	case dto.ErrorSelectionCancelled:
		return locales.ErrorsSelectionCancelled()
	case dto.ErrorCatalogIncompatible:
		return locales.ErrorsCatalogIncompatible()
	case dto.ErrorConfigurationIncompatible:
		return locales.ErrorsConfigurationIncompatible()
	case dto.ErrorStorageMigrationRejected:
		return locales.ErrorsStorageMigrationRejected()
	case dto.ErrorStorageUnavailable:
		return locales.ErrorsStorageUnavailable()
	}
	return ""
}

// StatusLabel names a process status in the app's language. The tray and the
// Settings window share it, so they never disagree.
func StatusLabel(presentation dto.ProcessPresentation) string {
	switch presentation.Key {
	case "running":
		return locales.StatusRunning()
	case "starting":
		return locales.StatusStarting()
	case "stopping":
		return locales.StatusStopping()
	case "restarting":
		return locales.StatusRestarting()
	case "saving-config":
		return locales.StatusSavingConfiguration()
	case "applying-config":
		return locales.StatusApplyingConfiguration()
	case "setup-required":
		return locales.StatusSetupRequired()
	case "stopped":
		return locales.StatusStopped()
	case "cleanup-required":
		return locales.StatusCleanupRequired()
	case "failed":
		return locales.StatusFailed()
	case "catalog-incompatible":
		return locales.StatusCatalogIncompatible()
	case "configuration-incompatible":
		return locales.StatusConfigurationIncompatible()
	case "storage-rejected":
		return locales.StatusStorageRejected()
	case "storage-unavailable":
		return locales.StatusStorageUnavailable()
	case "installing":
		return locales.StatusInstalling()
	case "retrying":
		return locales.StatusRetrying()
	case "not-installed":
		return locales.StatusNotInstalled()
	case "install-failed":
		return locales.StatusInstallFailed()
	case "disabled":
		return locales.StatusDisabled()
	}
	return presentation.Label
}

func networkLabel(mode string) string {
	switch mode {
	case "lan":
		return locales.NetworkLan()
	case "custom":
		return locales.NetworkCustom()
	default:
		return locales.NetworkLocal()
	}
}

// storageRiskLabel is the Desktop's copy for the Server's closed storage
// risk vocabulary, one literal entry per code the shared classifier can
// report on a native task. The architecture check compares these keys with
// the Server's constants, so a new code lands with its copy here as well as
// in the Web. An unknown code is shown as its identifier.
func storageRiskLabel(code string) string {
	switch code {
	case "removable_storage":
		return locales.StorageRiskRemovable()
	case "non_default_location":
		return locales.StorageRiskNonDefault()
	case "cloud_sync_directory":
		return locales.StorageRiskCloudSync()
	case "network_filesystem":
		return locales.StorageRiskNetwork()
	case "mount_fingerprint_changed":
		return locales.StorageRiskMountChanged()
	}
	return code
}

func hostActionLabel(kind string) string {
	switch kind {
	case "authorize_storage_location":
		return locales.StorageHostActionAddLocation()
	case "open_repository":
		return locales.StorageHostActionOpenRepository()
	case "locate_storage_location":
		return locales.StorageHostActionLocateLocation()
	case "locate_repository":
		return locales.StorageHostActionLocateRepository()
	}
	return locales.StorageHostAction()
}

func controlPhaseLabel(phase dto.LumenControlPhase) string {
	switch phase {
	case dto.LumenControlStarting:
		return locales.LumenPhaseStarting()
	case dto.LumenControlDownloading:
		return locales.LumenPhaseDownloading()
	case dto.LumenControlLoading:
		return locales.LumenPhaseLoading()
	case dto.LumenControlWarmup:
		return locales.LumenPhaseWarmup()
	case dto.LumenControlReady:
		return locales.LumenPhaseReady()
	case dto.LumenControlFailed:
		return locales.LumenPhaseFailed()
	case dto.LumenControlStopping:
		return locales.LumenPhaseStopping()
	}
	return locales.LumenPhaseUnavailable()
}

func controlPhaseColor(phase dto.LumenControlPhase) dto.DotColor {
	switch phase {
	case dto.LumenControlReady:
		return dto.DotGreen
	case dto.LumenControlFailed:
		return dto.DotRed
	case dto.LumenControlUnspecified, dto.LumenControlStopping, "":
		return dto.DotGray
	}
	return dto.DotYellow
}

func presetLabel(preset string) string {
	switch preset {
	case "minimal":
		return locales.LumenPresetNameMinimal()
	case "brave":
		return locales.LumenPresetNameBrave()
	case "basic":
		return locales.LumenPresetNameBasic()
	}
	return preset
}

func firstIssue(validation *dto.ConfigValidation) string {
	if validation != nil && len(validation.Issues) > 0 && validation.Issues[0].Message != "" {
		return validation.Issues[0].Message
	}
	return locales.ServerSettingsInvalid()
}

func formatBytes(bytes uint64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value, unit := float64(bytes), 0
	for value >= 1024 && unit < len(units)-1 {
		value /= 1024
		unit++
	}
	precision := 0
	if unit > 1 {
		precision = 1
	}
	return strconv.FormatFloat(value, 'f', precision, 64) + " " + units[unit]
}
