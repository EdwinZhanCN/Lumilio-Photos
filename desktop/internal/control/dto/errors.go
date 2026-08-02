package dto

// ErrorCode is a stable control-plane error identifier. The Settings client
// must map these values exhaustively; display text is deliberately separate.
type ErrorCode string

const (
	ErrorInvalidArgument              ErrorCode = "invalid_argument"
	ErrorStaleVersion                 ErrorCode = "stale_version"
	ErrorStaleConfig                  ErrorCode = "stale_config"
	ErrorControllerBusy               ErrorCode = "controller_busy"
	ErrorOperationConflict            ErrorCode = "operation_conflict"
	ErrorOperationNotCancellable      ErrorCode = "operation_not_cancellable"
	ErrorRuntimeNotConfigured         ErrorCode = "runtime_not_configured"
	ErrorRuntimeNotReady              ErrorCode = "runtime_not_ready"
	ErrorRepositoryControlUnavailable ErrorCode = "repository_control_unavailable"
	ErrorStorageLocationOffline       ErrorCode = "storage_location_offline"
	ErrorStopTimeout                  ErrorCode = "stop_timeout"
	ErrorReadinessTimeout             ErrorCode = "readiness_timeout"
	ErrorSignatureInvalid             ErrorCode = "signature_invalid"
	ErrorShutdownInProgress           ErrorCode = "shutdown_in_progress"
	ErrorLumenNotInstalled            ErrorCode = "lumen_not_installed"
	ErrorLumenOwnerBusy               ErrorCode = "lumen_owner_busy"
	ErrorRecoveryRequired             ErrorCode = "recovery_required"
)

// Error is the only error shape exposed through Wails bindings and events.
// Internal errors must be redacted before they reach this type.
type Error struct {
	Code        ErrorCode `json:"code"`
	Message     string    `json:"message"`
	OperationID string    `json:"operationID,omitempty"`
}

func (e Error) Empty() bool {
	return e.Code == "" && e.Message == "" && e.OperationID == ""
}
