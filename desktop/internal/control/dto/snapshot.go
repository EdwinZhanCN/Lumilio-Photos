package dto

const SnapshotChangedEvent = "desktop:snapshot-changed"

type DesiredState string

const (
	DesiredStopped  DesiredState = "stopped"
	DesiredRunning  DesiredState = "running"
	DesiredDisabled DesiredState = "disabled"
)

type RuntimePhase string

const (
	RuntimeStopped        RuntimePhase = "stopped"
	RuntimeStarting       RuntimePhase = "starting"
	RuntimeRunning        RuntimePhase = "running"
	RuntimeStopping       RuntimePhase = "stopping"
	RuntimeRestarting     RuntimePhase = "restarting"
	RuntimeSavingConfig   RuntimePhase = "saving-config"
	RuntimeApplyingConfig RuntimePhase = "applying-config"
	RuntimeFailed         RuntimePhase = "failed"
)

type LumenInstallPhase string

const (
	LumenAbsent        LumenInstallPhase = "absent"
	LumenInstalling    LumenInstallPhase = "installing"
	LumenInstalled     LumenInstallPhase = "installed"
	LumenInstallFailed LumenInstallPhase = "failed"
)

type LumenProcessPhase string

const (
	LumenStopped  LumenProcessPhase = "stopped"
	LumenStarting LumenProcessPhase = "starting"
	LumenRunning  LumenProcessPhase = "running"
	LumenStopping LumenProcessPhase = "stopping"
	LumenBackoff  LumenProcessPhase = "backoff"
	LumenFailed   LumenProcessPhase = "failed"
)

type LumenControlPhase string

const (
	LumenControlUnspecified LumenControlPhase = "unspecified"
	LumenControlStarting    LumenControlPhase = "starting"
	LumenControlDownloading LumenControlPhase = "downloading"
	LumenControlLoading     LumenControlPhase = "loading"
	LumenControlWarmup      LumenControlPhase = "warmup"
	LumenControlReady       LumenControlPhase = "ready"
	LumenControlFailed      LumenControlPhase = "failed"
	LumenControlStopping    LumenControlPhase = "stopping"
)

type Ownership string

const (
	OwnershipNone Ownership = "none"
	OwnershipHeld Ownership = "held"
)

type ShutdownPhase string

const (
	ShutdownIdle      ShutdownPhase = "idle"
	ShutdownQuiescing ShutdownPhase = "quiescing"
	ShutdownFailed    ShutdownPhase = "failed"
	ShutdownArmed     ShutdownPhase = "armed"
)

type DotColor string

const (
	DotGray   DotColor = "gray"
	DotYellow DotColor = "yellow"
	DotGreen  DotColor = "green"
	DotRed    DotColor = "red"
)

type ProcessPresentation struct {
	Color DotColor `json:"color"`
	Label string   `json:"label"`
}

type Capabilities struct {
	CanOpenProduct               bool `json:"canOpenProduct"`
	CanStartRuntime              bool `json:"canStartRuntime"`
	CanStopRuntime               bool `json:"canStopRuntime"`
	CanRetryCleanupRuntime       bool `json:"canRetryCleanupRuntime"`
	CanRestartRuntime            bool `json:"canRestartRuntime"`
	CanStartLumen                bool `json:"canStartLumen"`
	CanStopLumen                 bool `json:"canStopLumen"`
	CanRetryCleanupLumen         bool `json:"canRetryCleanupLumen"`
	CanRestartLumen              bool `json:"canRestartLumen"`
	CanResumeAfterFailedShutdown bool `json:"canResumeAfterFailedShutdown"`
	CanRequestQuit               bool `json:"canRequestQuit"`
	CanApplyUpdate               bool `json:"canApplyUpdate"`
}

type NavigationIntent struct {
	Sequence uint64 `json:"sequence"`
	Route    string `json:"route"`
}

type ShutdownSnapshot struct {
	Phase       ShutdownPhase `json:"phase"`
	OperationID string        `json:"operationID,omitempty"`
	Error       Error         `json:"error,omitempty"`
}

type HostSnapshot struct {
	BootPhase          string             `json:"bootPhase"`
	SettingsVisible    bool               `json:"settingsVisible"`
	SettingsNavigation NavigationIntent   `json:"settingsNavigation"`
	Preferences        DesktopPreferences `json:"preferences"`
	Shutdown           ShutdownSnapshot   `json:"shutdown"`
	Recovery           Error              `json:"recovery,omitempty"`
}

// DesktopPreferences contains host-owned settings. These values are persisted
// in settings.v1.json and are deliberately separate from the Server manifest.
type DesktopPreferences struct {
	Version             uint64 `json:"version"`
	Locale              string `json:"locale"`
	Region              string `json:"region"`
	UpdateChannel       string `json:"updateChannel"`
	Theme               string `json:"theme"`
	OpenProductOnLaunch bool   `json:"openProductOnLaunch"`
}

type RuntimeSnapshot struct {
	Version                 uint64              `json:"version"`
	DesiredState            DesiredState        `json:"desiredState"`
	Phase                   RuntimePhase        `json:"phase"`
	Ownership               Ownership           `json:"ownership"`
	RecoveryCause           ErrorCode           `json:"recoveryCause,omitempty"`
	PendingConfigValidation bool                `json:"pendingConfigValidation"`
	Configured              bool                `json:"configured"`
	ManifestSHA256          string              `json:"manifestSHA256,omitempty"`
	ProductURL              string              `json:"productURL,omitempty"`
	Presentation            ProcessPresentation `json:"presentation"`
	Capabilities            Capabilities        `json:"capabilities"`
}

type LumenSnapshot struct {
	Version            uint64              `json:"version"`
	InstallPhase       LumenInstallPhase   `json:"installPhase"`
	DesiredState       DesiredState        `json:"desiredState"`
	ProcessPhase       LumenProcessPhase   `json:"processPhase"`
	Ownership          Ownership           `json:"ownership"`
	RecoveryCause      ErrorCode           `json:"recoveryCause,omitempty"`
	Profile            string              `json:"profile,omitempty"`
	Preset             string              `json:"preset"`
	CacheDir           string              `json:"cacheDir"`
	AvailableProfiles  []string            `json:"availableProfiles"`
	AvailablePresets   []string            `json:"availablePresets"`
	Control            LumenControlStatus  `json:"control"`
	InstallerAvailable bool                `json:"installerAvailable"`
	ProcessAvailable   bool                `json:"processAvailable"`
	Presentation       ProcessPresentation `json:"presentation"`
	Capabilities       Capabilities        `json:"capabilities"`
}

type LumenDownloadProgress struct {
	Model      string `json:"model"`
	File       string `json:"file"`
	BytesDone  uint64 `json:"bytesDone"`
	BytesTotal uint64 `json:"bytesTotal"`
	FilesDone  uint32 `json:"filesDone"`
	FilesTotal uint32 `json:"filesTotal"`
}

type LumenServiceStatus struct {
	Service string            `json:"service"`
	Phase   LumenControlPhase `json:"phase"`
	Error   *Error            `json:"error,omitempty"`
}

type LumenControlStatus struct {
	Connected       bool                   `json:"connected"`
	InferenceReady  bool                   `json:"inferenceReady"`
	Phase           LumenControlPhase      `json:"phase"`
	Version         string                 `json:"version,omitempty"`
	Backend         string                 `json:"backend,omitempty"`
	StartedAtUnixMS int64                  `json:"startedAtUnixMS,omitempty"`
	Download        *LumenDownloadProgress `json:"download,omitempty"`
	Services        []LumenServiceStatus   `json:"services"`
	Error           *Error                 `json:"error,omitempty"`
	Sequence        uint64                 `json:"sequence"`
}

type LumenLogEntry struct {
	TimeUnixMS int64             `json:"timeUnixMS"`
	Level      string            `json:"level"`
	Target     string            `json:"target"`
	Message    string            `json:"message"`
	Fields     map[string]string `json:"fields"`
}

type StorageShortcut struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Path    string `json:"path,omitempty"`
	Kind    string `json:"kind,omitempty"`
	Status  string `json:"status"`
	CanOpen bool   `json:"canOpen"`
}

type StorageSummary struct {
	Version uint64 `json:"version"`
	Count   int    `json:"count"`
}

type UpdateSnapshot struct {
	Version           uint64 `json:"version"`
	Phase             string `json:"phase"`
	Channel           string `json:"channel"`
	CurrentVersion    string `json:"currentVersion"`
	AvailableVersion  string `json:"availableVersion,omitempty"`
	ProviderAvailable bool   `json:"providerAvailable"`
	Error             Error  `json:"error,omitempty"`
	CanApply          bool   `json:"canApply"`
}

type OperationSnapshot struct {
	OperationID string `json:"operationID"`
	RequestID   string `json:"requestID"`
	Aggregate   string `json:"aggregate"`
	State       string `json:"state"`
	Cancellable bool   `json:"cancellable"`
	Error       Error  `json:"error,omitempty"`
}

type DesktopSnapshot struct {
	InstanceID string              `json:"instanceID"`
	Revision   uint64              `json:"revision"`
	Host       HostSnapshot        `json:"host"`
	Runtime    RuntimeSnapshot     `json:"runtime"`
	Storage    StorageSummary      `json:"storage"`
	Lumen      LumenSnapshot       `json:"lumen"`
	Update     UpdateSnapshot      `json:"update"`
	Operations []OperationSnapshot `json:"operations"`
}

type SnapshotChanged struct {
	InstanceID string `json:"instanceID"`
	Revision   uint64 `json:"revision"`
}

type OperationReceipt struct {
	OperationID     string `json:"operationID"`
	RequestID       string `json:"requestID"`
	Aggregate       string `json:"aggregate"`
	AcceptedVersion uint64 `json:"acceptedVersion"`
}

type ErrorResponse struct {
	Error Error `json:"error"`
}

type ConfigIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
}

type ConfigValidation struct {
	Valid       bool          `json:"valid"`
	Fingerprint string        `json:"fingerprint,omitempty"`
	Issues      []ConfigIssue `json:"issues,omitempty"`
}

// RuntimeConfigSettings is the small, user-facing projection of the complete
// Server manifest. The Desktop frontend edits this structure; Go patches the
// authoritative TOML candidate and runs the real strict loader.
type RuntimeConfigSettings struct {
	NetworkMode           string `json:"networkMode"`
	Listen                string `json:"listen"`
	StoragePath           string `json:"storagePath"`
	LoggingLevel          string `json:"loggingLevel"`
	RepositoryScanEnabled bool   `json:"repositoryScanEnabled"`
	HardwareAcceleration  string `json:"hardwareAcceleration"`
}

type RuntimeConfigDraft struct {
	TOML                 string                `json:"toml"`
	BaseFingerprint      string                `json:"baseFingerprint"`
	CandidateFingerprint string                `json:"candidateFingerprint"`
	Source               string                `json:"source"`
	Settings             RuntimeConfigSettings `json:"settings"`
	Validation           ConfigValidation      `json:"validation"`
}

func InitialSnapshot(instanceID string) DesktopSnapshot {
	return DesktopSnapshot{
		InstanceID: instanceID,
		Revision:   1,
		Host: HostSnapshot{
			BootPhase: "created",
			Preferences: DesktopPreferences{
				Version:       1,
				Locale:        "en",
				Region:        "global",
				UpdateChannel: "stable",
				Theme:         "system",
			},
			Shutdown: ShutdownSnapshot{Phase: ShutdownIdle},
		},
		Runtime: RuntimeSnapshot{
			Version:      1,
			DesiredState: DesiredStopped,
			Phase:        RuntimeStopped,
			Ownership:    OwnershipNone,
			Presentation: ProcessPresentation{Color: DotGray, Label: "Setup required"},
		},
		Lumen: LumenSnapshot{
			Version:      1,
			InstallPhase: LumenAbsent,
			DesiredState: DesiredDisabled,
			ProcessPhase: LumenStopped,
			Ownership:    OwnershipNone,
			Preset:       "basic",
			Control:      LumenControlStatus{Phase: LumenControlUnspecified},
			Presentation: ProcessPresentation{Color: DotGray, Label: "Not Installed"},
		},
		Update: UpdateSnapshot{Version: 1, Phase: "idle", Channel: "stable"},
	}
}

func (s DesktopSnapshot) Clone() DesktopSnapshot {
	clone := s
	clone.Operations = append([]OperationSnapshot(nil), s.Operations...)
	clone.Lumen.AvailableProfiles = append([]string(nil), s.Lumen.AvailableProfiles...)
	clone.Lumen.AvailablePresets = append([]string(nil), s.Lumen.AvailablePresets...)
	clone.Lumen.Control.Services = append([]LumenServiceStatus(nil), s.Lumen.Control.Services...)
	if s.Lumen.Control.Download != nil {
		download := *s.Lumen.Control.Download
		clone.Lumen.Control.Download = &download
	}
	if s.Lumen.Control.Error != nil {
		controlError := *s.Lumen.Control.Error
		clone.Lumen.Control.Error = &controlError
	}
	for index := range clone.Lumen.Control.Services {
		if s.Lumen.Control.Services[index].Error != nil {
			serviceError := *s.Lumen.Control.Services[index].Error
			clone.Lumen.Control.Services[index].Error = &serviceError
		}
	}
	return clone
}
