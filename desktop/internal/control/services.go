package control

import (
	"context"
	"strings"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/state"
)

type HostActions interface {
	ShowSettings(route string) error
	OpenProduct() error
	RequestQuit(requestID string, expectedVersion uint64) (dto.OperationReceipt, error)
	ResumeAfterFailedShutdown(requestID string, expectedVersion uint64) (dto.OperationReceipt, error)
	ForceQuit(requestID string, expectedVersion uint64, confirmation string) (dto.OperationReceipt, error)
}

// DesktopService is the narrow host-facing binding surface. It never exposes
// process handles, paths, or generic command dispatch.
type DesktopService struct {
	Store                     *state.Store
	Host                      HostActions
	Preferences               DesktopPreferencesAdapter
	OpenRuntimeManifestAction func() error
}

type DesktopPreferencesAdapter interface {
	Save(dto.DesktopPreferences) (dto.DesktopPreferences, error)
}

func (s *DesktopService) GetSnapshot() dto.DesktopSnapshot {
	if s == nil || s.Store == nil {
		return dto.DesktopSnapshot{}
	}
	return s.Store.Get()
}

func (s *DesktopService) ShowSettings(route string) error {
	if !validRoute(route) {
		return operation.NewError(dto.ErrorInvalidArgument, "unknown Settings route")
	}
	if s.Host == nil {
		return operation.NewError(dto.ErrorRuntimeNotReady, "Desktop host is unavailable")
	}
	return s.Host.ShowSettings(route)
}

func (s *DesktopService) OpenProduct() error {
	if s.Host == nil {
		return operation.NewError(dto.ErrorRuntimeNotReady, "Desktop host is unavailable")
	}
	return s.Host.OpenProduct()
}

func (s *DesktopService) SavePreferences(preferences dto.DesktopPreferences) (dto.DesktopPreferences, error) {
	if s == nil || s.Preferences == nil {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorRuntimeNotReady, "Desktop preferences are unavailable")
	}
	return s.Preferences.Save(preferences)
}

func (s *DesktopService) OpenRuntimeManifest() error {
	if s == nil || s.OpenRuntimeManifestAction == nil {
		return operation.NewError(dto.ErrorRuntimeNotReady, "runtime manifest location is unavailable")
	}
	return s.OpenRuntimeManifestAction()
}

func (s *DesktopService) RequestQuit(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if s.Host == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Desktop host is unavailable")
	}
	return s.Host.RequestQuit(requestID, expectedVersion)
}

func (s *DesktopService) ResumeAfterFailedShutdown(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if s.Host == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Desktop host is unavailable")
	}
	return s.Host.ResumeAfterFailedShutdown(requestID, expectedVersion)
}

func (s *DesktopService) ForceQuit(requestID string, expectedVersion uint64, confirmation string) (dto.OperationReceipt, error) {
	if strings.TrimSpace(confirmation) != "FORCE QUIT" {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "force quit confirmation is required")
	}
	if s.Host == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Desktop host is unavailable")
	}
	return s.Host.ForceQuit(requestID, expectedVersion, confirmation)
}

type RuntimeController interface {
	Snapshot() dto.DesktopSnapshot
	Start(string, uint64) (dto.OperationReceipt, error)
	Stop(string, uint64) (dto.OperationReceipt, error)
	Restart(string, uint64) (dto.OperationReceipt, error)
	RetryCleanup(string, uint64) (dto.OperationReceipt, error)
}

type RuntimeConfigAdapter interface {
	ReadDraft() (dto.RuntimeConfigDraft, error)
	PatchDraft(string, dto.RuntimeConfigSettings) (dto.RuntimeConfigDraft, error)
	Validate(string) (dto.ConfigValidation, error)
	Save(string, uint64, string, string) (dto.OperationReceipt, error)
	Apply(string, uint64, string, string) (dto.OperationReceipt, error)
	RestoreLastKnownGood(string, uint64) (dto.OperationReceipt, error)
}

type RuntimeService struct {
	Controller RuntimeController
	Config     RuntimeConfigAdapter
}

func (s *RuntimeService) GetSnapshot() dto.RuntimeSnapshot {
	if s == nil || s.Controller == nil {
		return dto.RuntimeSnapshot{}
	}
	return s.Controller.Snapshot().Runtime
}

func (s *RuntimeService) ReadConfigDraft() (dto.RuntimeConfigDraft, error) {
	if s == nil || s.Config == nil {
		return dto.RuntimeConfigDraft{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime config controller is unavailable")
	}
	return s.Config.ReadDraft()
}

func (s *RuntimeService) PatchConfigDraft(candidate string, settings dto.RuntimeConfigSettings) (dto.RuntimeConfigDraft, error) {
	if s == nil || s.Config == nil {
		return dto.RuntimeConfigDraft{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime config controller is unavailable")
	}
	return s.Config.PatchDraft(candidate, settings)
}

func (s *RuntimeService) Start(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return s.mutate(func() (dto.OperationReceipt, error) { return s.Controller.Start(requestID, expectedVersion) })
}

func (s *RuntimeService) Stop(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return s.mutate(func() (dto.OperationReceipt, error) { return s.Controller.Stop(requestID, expectedVersion) })
}

func (s *RuntimeService) Restart(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return s.mutate(func() (dto.OperationReceipt, error) { return s.Controller.Restart(requestID, expectedVersion) })
}

func (s *RuntimeService) RetryCleanup(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return s.mutate(func() (dto.OperationReceipt, error) { return s.Controller.RetryCleanup(requestID, expectedVersion) })
}

func (s *RuntimeService) ValidateConfig(candidate string) (dto.ConfigValidation, error) {
	if s == nil || s.Config == nil {
		return dto.ConfigValidation{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime config controller is unavailable")
	}
	return s.Config.Validate(candidate)
}

func (s *RuntimeService) SaveConfig(requestID string, expectedVersion uint64, baseFingerprint, candidate string) (dto.OperationReceipt, error) {
	if s == nil || s.Config == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime config controller is unavailable")
	}
	return s.Config.Save(requestID, expectedVersion, baseFingerprint, candidate)
}

func (s *RuntimeService) ApplyConfig(requestID string, expectedVersion uint64, baseFingerprint, candidate string) (dto.OperationReceipt, error) {
	if s == nil || s.Config == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime config controller is unavailable")
	}
	return s.Config.Apply(requestID, expectedVersion, baseFingerprint, candidate)
}

func (s *RuntimeService) RestoreLastKnownGood(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if s == nil || s.Config == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime config controller is unavailable")
	}
	return s.Config.RestoreLastKnownGood(requestID, expectedVersion)
}

func (s *RuntimeService) mutate(fn func() (dto.OperationReceipt, error)) (dto.OperationReceipt, error) {
	if s == nil || s.Controller == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "runtime controller is unavailable")
	}
	return fn()
}

type StorageAdapter interface {
	ListShortcuts(context.Context) ([]dto.StorageShortcut, error)
	PickLocation(string) (string, error)
	OpenLocation(context.Context, string) error
	AddLocation(string, uint64, string, string) (dto.OperationReceipt, error)
	AttachRepository(string, uint64, string) (dto.OperationReceipt, error)
}

type StorageService struct{ Adapter StorageAdapter }

func (s *StorageService) ListShortcuts() ([]dto.StorageShortcut, error) {
	if s == nil || s.Adapter == nil {
		return nil, operation.NewError(dto.ErrorRepositoryControlUnavailable, "storage control is unavailable")
	}
	return s.Adapter.ListShortcuts(context.Background())
}

func (s *StorageService) PickLocation(title string) (string, error) {
	if s == nil || s.Adapter == nil {
		return "", operation.NewError(dto.ErrorRepositoryControlUnavailable, "storage control is unavailable")
	}
	return s.Adapter.PickLocation(title)
}

func (s *StorageService) OpenLocation(id string) error {
	if strings.TrimSpace(id) == "" {
		return operation.NewError(dto.ErrorInvalidArgument, "storage location ID is required")
	}
	if s == nil || s.Adapter == nil {
		return operation.NewError(dto.ErrorRepositoryControlUnavailable, "storage control is unavailable")
	}
	return s.Adapter.OpenLocation(context.Background(), id)
}

func (s *StorageService) AddLocation(requestID string, expectedVersion uint64, path, name string) (dto.OperationReceipt, error) {
	if s == nil || s.Adapter == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRepositoryControlUnavailable, "storage control is unavailable")
	}
	return s.Adapter.AddLocation(requestID, expectedVersion, path, name)
}

func (s *StorageService) AttachRepository(requestID string, expectedVersion uint64, path string) (dto.OperationReceipt, error) {
	if s == nil || s.Adapter == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRepositoryControlUnavailable, "storage control is unavailable")
	}
	return s.Adapter.AttachRepository(requestID, expectedVersion, path)
}

type LumenAdapter interface {
	Snapshot() dto.LumenSnapshot
	Logs(uint32, string) ([]dto.LumenLogEntry, error)
	PickCacheDirectory(string) (string, error)
	Install(string, uint64, string, string, string) (dto.OperationReceipt, error)
	Start(string, uint64) (dto.OperationReceipt, error)
	Stop(string, uint64) (dto.OperationReceipt, error)
	Restart(string, uint64) (dto.OperationReceipt, error)
	RetryCleanup(string, uint64) (dto.OperationReceipt, error)
}

type LumenService struct{ Controller LumenAdapter }

func (s *LumenService) GetSnapshot() dto.LumenSnapshot {
	if s == nil || s.Controller == nil {
		return dto.LumenSnapshot{}
	}
	return s.Controller.Snapshot()
}

func (s *LumenService) GetLogs(backlog uint32, minLevel string) ([]dto.LumenLogEntry, error) {
	if s == nil || s.Controller == nil {
		return nil, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen controller is unavailable")
	}
	return s.Controller.Logs(backlog, minLevel)
}

func (s *LumenService) PickCacheDirectory(title string) (string, error) {
	if s == nil || s.Controller == nil {
		return "", operation.NewError(dto.ErrorRuntimeNotReady, "Lumen controller is unavailable")
	}
	return s.Controller.PickCacheDirectory(title)
}

func (s *LumenService) Install(requestID string, version uint64, profile, preset, cacheDir string) (dto.OperationReceipt, error) {
	if s == nil || s.Controller == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen controller is unavailable")
	}
	return s.Controller.Install(requestID, version, profile, preset, cacheDir)
}

func (s *LumenService) Start(requestID string, version uint64) (dto.OperationReceipt, error) {
	if s == nil || s.Controller == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen controller is unavailable")
	}
	return s.Controller.Start(requestID, version)
}

func (s *LumenService) Stop(requestID string, version uint64) (dto.OperationReceipt, error) {
	if s == nil || s.Controller == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen controller is unavailable")
	}
	return s.Controller.Stop(requestID, version)
}

func (s *LumenService) Restart(requestID string, version uint64) (dto.OperationReceipt, error) {
	if s == nil || s.Controller == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen controller is unavailable")
	}
	return s.Controller.Restart(requestID, version)
}

func (s *LumenService) RetryCleanup(requestID string, version uint64) (dto.OperationReceipt, error) {
	if s == nil || s.Controller == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "Lumen controller is unavailable")
	}
	return s.Controller.RetryCleanup(requestID, version)
}

type UpdateAdapter interface {
	Snapshot() dto.UpdateSnapshot
	Check(string, uint64) (dto.OperationReceipt, error)
	Download(string, uint64) (dto.OperationReceipt, error)
	RestartAndApply(string, uint64) (dto.OperationReceipt, error)
}

type UpdateService struct {
	Store   *state.Store
	Adapter UpdateAdapter
}

func (s *UpdateService) GetSnapshot() dto.UpdateSnapshot {
	if s != nil && s.Adapter != nil {
		return s.Adapter.Snapshot()
	}
	if s == nil || s.Store == nil {
		return dto.UpdateSnapshot{}
	}
	return s.Store.Get().Update
}

func (s *UpdateService) Check(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if strings.TrimSpace(requestID) == "" {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "requestID is required")
	}
	if s == nil || s.Adapter == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "update controller is not configured")
	}
	return s.Adapter.Check(requestID, expectedVersion)
}

func (s *UpdateService) Download(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if s == nil || s.Adapter == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "update controller is not configured")
	}
	return s.Adapter.Download(requestID, expectedVersion)
}

func (s *UpdateService) RestartAndApply(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	if s == nil || s.Adapter == nil {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorRuntimeNotReady, "update controller is not configured")
	}
	return s.Adapter.RestartAndApply(requestID, expectedVersion)
}

type DiagnosticsService struct{ Store *state.Store }

func (s *DiagnosticsService) GetSnapshot() dto.DesktopSnapshot {
	if s == nil || s.Store == nil {
		return dto.DesktopSnapshot{}
	}
	return s.Store.Get()
}

func validRoute(route string) bool {
	switch route {
	case "/onboarding", "/general", "/server", "/storage", "/lumen", "/updates", "/diagnostics", "/recovery", "/overview", "/runtime", "/settings":
		return true
	default:
		return false
	}
}
