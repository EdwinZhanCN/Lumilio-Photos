// Package runtimeconfig stores the complete, schema-versioned Server intent
// and its crash-recoverable current/LKG pointers. It never reads secrets or
// fills missing manifest fields; strict validation belongs to server/config.
package runtimeconfig

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"desktop/internal/control/dto"
	"desktop/internal/platform"

	"github.com/google/uuid"
	"github.com/pelletier/go-toml/v2"
	"gopkg.in/yaml.v3"
	"server/config"
)

const PointerSchemaVersion = 1

type JournalPhase string

const (
	PhasePrepared          JournalPhase = "prepared"
	PhaseStoppingPrevious  JournalPhase = "stopping_previous"
	PhasePreviousStopped   JournalPhase = "previous_stopped"
	PhaseCandidateSelected JournalPhase = "candidate_selected"
	PhaseStoppingCandidate JournalPhase = "stopping_candidate"
	PhaseRollbackSelected  JournalPhase = "rollback_selected"
	PhaseCommitting        JournalPhase = "committing"
)

type Pointer struct {
	SchemaVersion int    `json:"schemaVersion"`
	Fingerprint   string `json:"fingerprint"`
}

type Journal struct {
	SchemaVersion        int          `json:"schemaVersion"`
	OperationID          string       `json:"operationID"`
	Mode                 string       `json:"mode"`
	Phase                JournalPhase `json:"phase"`
	PreviousFingerprint  string       `json:"previousFingerprint"`
	CandidateFingerprint string       `json:"candidateFingerprint"`
}

type Validation struct {
	Config      config.AppConfig
	Canonical   []byte
	Fingerprint string
}

type ReconcileResult struct {
	Journal       Journal
	Current       Pointer
	LastKnownGood Pointer
	NeedsResume   bool
	NeedsRollback bool
}

type Store struct {
	paths platform.Paths
}

func NewStore(paths platform.Paths) *Store { return &Store{paths: paths} }

func (s *Store) Paths() platform.Paths { return s.paths }

func Fingerprint(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

func Canonicalize(data []byte) ([]byte, error) {
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse runtime intent TOML: %w", err)
	}
	canonical, err := toml.Marshal(document)
	if err != nil {
		return nil, fmt.Errorf("canonicalize runtime intent TOML: %w", err)
	}
	return canonical, nil
}

func (s *Store) Validate(path string, data []byte) (Validation, error) {
	data, err := s.projectHostMediaPaths(data)
	if err != nil {
		return Validation{}, err
	}
	canonical, err := Canonicalize(data)
	if err != nil {
		return Validation{}, err
	}
	cfg, err := config.LoadAppConfigBytes(path, canonical)
	if err != nil {
		return Validation{}, err
	}
	return Validation{Config: cfg, Canonical: canonical, Fingerprint: Fingerprint(canonical)}, nil
}

// projectHostMediaPaths projects host-specific media paths before strict
// validation/loading. Existing manifests may still contain bare development
// command names; the host projection repairs those at validation/startup.
// Missing tools remain empty and are rejected by the strict Server loader
// instead of falling back to an ambient package-manager PATH at runtime.
func (s *Store) projectHostMediaPaths(data []byte) ([]byte, error) {
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, fmt.Errorf("parse runtime intent TOML: %w", err)
	}
	tools, err := requireTable(document, "tools")
	if err != nil {
		return nil, err
	}
	server, err := requireTable(document, "server")
	if err != nil {
		return nil, err
	}
	paths, err := platform.ResolveMediaToolPaths()
	if err != nil {
		return nil, err
	}
	for key, value := range map[string]string{
		"exiftool_path": paths.ExifTool,
		"ffmpeg_path":   paths.FFmpeg,
		"ffprobe_path":  paths.FFprobe,
	} {
		tools[key] = value
	}
	server["web_root"] = s.paths.WebRoot
	return toml.Marshal(document)
}

// ReadDraft returns the current intent, or a complete Desktop-local candidate
// on first run. The strict Server loader still receives every manifest field;
// defaults are an explicit one-shot Desktop onboarding choice, not a runtime
// fallback or a second configuration source.
func (s *Store) ReadDraft() (dto.RuntimeConfigDraft, error) {
	pointer, err := s.CurrentPointer()
	if err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	data := []byte(nil)
	source := "current"
	if pointer.Fingerprint == "" {
		data, err = s.defaultIntent()
		source = "default"
	} else {
		data, err = s.LoadIntent(pointer.Fingerprint)
	}
	if err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	return s.draftFromBytes(data, pointer.Fingerprint, source)
}

func (s *Store) PatchDraft(candidate string, settings dto.RuntimeConfigSettings) (dto.RuntimeConfigDraft, error) {
	data := []byte(candidate)
	if strings.TrimSpace(candidate) == "" {
		var err error
		data, err = s.defaultIntent()
		if err != nil {
			return dto.RuntimeConfigDraft{}, err
		}
	}
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return dto.RuntimeConfigDraft{}, fmt.Errorf("parse runtime intent TOML: %w", err)
	}
	serverTable, err := requireTable(document, "server")
	if err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	proxyTable, err := requireTable(serverTable, "proxy")
	if err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	switch settings.NetworkMode {
	case "local":
		serverTable["listen"] = "127.0.0.1:6680"
		proxyTable["trusted_cidrs"] = []string{"127.0.0.1/32", "::1/128"}
	case "lan":
		serverTable["listen"] = "0.0.0.0:6680"
		proxyTable["trusted_cidrs"] = []string{}
	case "custom":
		if strings.TrimSpace(settings.Listen) == "" {
			return dto.RuntimeConfigDraft{}, errors.New("custom listener is required")
		}
		serverTable["listen"] = strings.TrimSpace(settings.Listen)
	default:
		return dto.RuntimeConfigDraft{}, fmt.Errorf("unsupported Desktop network mode %q", settings.NetworkMode)
	}
	if err := s.validateStoragePathChange(settings.StoragePath); err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	if err := setTableValue(document, "storage", "path", strings.TrimSpace(settings.StoragePath)); err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	if err := setTableValue(document, "logging", "level", strings.TrimSpace(settings.LoggingLevel)); err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	if err := setTableValue(document, "repository_scan", "enabled", settings.RepositoryScanEnabled); err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	if err := setTableValue(document, "transcode", "hardware_accel", strings.TrimSpace(settings.HardwareAcceleration)); err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	patched, err := toml.Marshal(document)
	if err != nil {
		return dto.RuntimeConfigDraft{}, fmt.Errorf("encode runtime intent TOML: %w", err)
	}
	pointer, err := s.CurrentPointer()
	if err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	return s.draftFromBytes(patched, pointer.Fingerprint, "candidate")
}

func (s *Store) draftFromBytes(data []byte, baseFingerprint, source string) (dto.RuntimeConfigDraft, error) {
	validation, err := s.Validate(filepath.Join(s.paths.RuntimeIntents, "candidate.toml"), data)
	if err != nil {
		return dto.RuntimeConfigDraft{}, err
	}
	settings := dto.RuntimeConfigSettings{
		NetworkMode:           desktopNetworkMode(validation.Config.ServerConfig.Listen),
		Listen:                validation.Config.ServerConfig.Listen,
		StoragePath:           validation.Config.StorageConfig.Path,
		LoggingLevel:          validation.Config.LoggingConfig.Level,
		RepositoryScanEnabled: validation.Config.RepositoryScan.Enabled,
		HardwareAcceleration:  validation.Config.Transcode.HardwareAccel,
	}
	return dto.RuntimeConfigDraft{
		TOML: string(validation.Canonical), BaseFingerprint: baseFingerprint,
		CandidateFingerprint: validation.Fingerprint, Source: source, Settings: settings,
		Validation: dto.ConfigValidation{Valid: true, Fingerprint: validation.Fingerprint},
	}, nil
}

func (s *Store) validateStoragePathChange(candidate string) error {
	pointer, err := s.CurrentPointer()
	if err != nil {
		return err
	}
	if pointer.Fingerprint == "" {
		return nil
	}
	current, err := s.LoadCurrentConfig()
	if err != nil {
		return err
	}
	currentPath := filepath.Clean(strings.TrimSpace(current.StorageConfig.Path))
	candidatePath := filepath.Clean(strings.TrimSpace(candidate))
	if currentPath == candidatePath {
		return nil
	}
	newRootID, err := loadPortableMarkerID(filepath.Join(candidatePath, ".lumilioroot"))
	if err != nil {
		return fmt.Errorf("selected default storage location has no valid .lumilioroot marker: %w", err)
	}
	newPrimaryID, err := loadPortableMarkerID(filepath.Join(candidatePath, "primary", ".lumiliorepo"))
	if err != nil {
		return fmt.Errorf("selected default storage location has no valid primary/.lumiliorepo marker: %w", err)
	}
	// If the old path is still online, the Desktop can prove immediately that
	// the selected directory is the same portable root and primary repository.
	// A true move normally leaves the old path offline; the Server then performs
	// the authoritative comparison against the catalog during controlled restart.
	if oldRootID, loadErr := loadPortableMarkerID(filepath.Join(currentPath, ".lumilioroot")); loadErr == nil {
		if oldRootID != newRootID {
			return errors.New("selected default storage location has a different .lumilioroot identity")
		}
		oldPrimaryID, primaryErr := loadPortableMarkerID(filepath.Join(currentPath, "primary", ".lumiliorepo"))
		if primaryErr != nil {
			return fmt.Errorf("current default storage location primary marker is invalid: %w", primaryErr)
		}
		if oldPrimaryID != newPrimaryID {
			return errors.New("selected default storage location has a different primary repository identity")
		}
	}
	return nil
}

func loadPortableMarkerID(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var marker struct {
		Version string `yaml:"version"`
		ID      string `yaml:"id"`
	}
	if err := yaml.Unmarshal(data, &marker); err != nil {
		return "", err
	}
	if marker.Version != "1.0" {
		return "", fmt.Errorf("unsupported marker version %q", marker.Version)
	}
	parsed, err := uuid.Parse(strings.TrimSpace(marker.ID))
	if err != nil {
		return "", err
	}
	return parsed.String(), nil
}

func (s *Store) defaultIntent() ([]byte, error) {
	profile, ok := config.ProfileByName(config.ProfileDesktopLocal)
	if !ok {
		return nil, errors.New("Desktop-local config profile is unavailable")
	}
	data, err := config.EncodeProfile(profile, config.ProfileInputs{}, "")
	if err != nil {
		return nil, err
	}
	var document map[string]any
	if err := toml.Unmarshal(data, &document); err != nil {
		return nil, err
	}
	stateDir := filepath.Join(s.paths.Root, "state")
	storagePath := filepath.Join(s.paths.Root, "media")
	if home, homeErr := os.UserHomeDir(); homeErr == nil && strings.TrimSpace(home) != "" {
		storagePath = filepath.Join(home, "Pictures", "Lumilio")
	}
	values := []struct {
		table string
		key   string
		value any
	}{
		{"database", "path", filepath.Join(stateDir, "library.sqlite3")},
		{"logging", "dir", s.paths.LogsDir},
		{"storage", "path", storagePath},
		{"storage", "cloud_state_path", filepath.Join(stateDir, "cloud")},
		{"storage", "backups_path", filepath.Join(stateDir, "backups")},
		{"auth", "secret_key_file", filepath.Join(s.paths.SecretsDir, "lumilio_secret_key")},
	}
	for _, value := range values {
		if err := setTableValue(document, value.table, value.key, value.value); err != nil {
			return nil, err
		}
	}
	serverTable, err := requireTable(document, "server")
	if err != nil {
		return nil, err
	}
	// The product SPA ships in the packaged bundle (Resources/web on macOS,
	// resources/web in the Windows portable layout). Point the in-process
	// server at it so the published ProductURL serves the full app; when no
	// bundle is present (dev runs) the empty root keeps the runtime API-only.
	serverTable["web_root"] = s.paths.WebRoot
	return toml.Marshal(document)
}

func requireTable(document map[string]any, key string) (map[string]any, error) {
	value, ok := document[key]
	if !ok {
		return nil, fmt.Errorf("runtime configuration is missing [%s]", key)
	}
	table, ok := value.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("runtime configuration [%s] is not a table", key)
	}
	return table, nil
}

func setTableValue(document map[string]any, table, key string, value any) error {
	target, err := requireTable(document, table)
	if err != nil {
		return err
	}
	target[key] = value
	return nil
}

func desktopNetworkMode(listen string) string {
	switch strings.TrimSpace(listen) {
	case "127.0.0.1:6680", "localhost:6680", "[::1]:6680":
		return "local"
	case "0.0.0.0:6680", ":6680", "[::]:6680":
		return "lan"
	default:
		return "custom"
	}
}

func (s *Store) LoadCurrentConfig() (config.AppConfig, error) {
	pointer, err := s.CurrentPointer()
	if err != nil {
		return config.AppConfig{}, err
	}
	if pointer.Fingerprint == "" {
		return config.AppConfig{}, fmt.Errorf("runtime is not configured")
	}
	data, err := s.LoadIntent(pointer.Fingerprint)
	if err != nil {
		return config.AppConfig{}, err
	}
	data, err = s.projectHostMediaPaths(data)
	if err != nil {
		return config.AppConfig{}, err
	}
	return config.LoadAppConfigBytes(filepath.Join(s.paths.RuntimeIntents, pointer.Fingerprint[len("sha256:"):]+".toml"), data)
}

func (s *Store) LoadPointer(path string) (Pointer, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Pointer{}, nil
	}
	if err != nil {
		return Pointer{}, err
	}
	var pointer Pointer
	if err := json.Unmarshal(data, &pointer); err != nil {
		return Pointer{}, fmt.Errorf("decode runtime pointer: %w", err)
	}
	if pointer.SchemaVersion != PointerSchemaVersion || !validFingerprint(pointer.Fingerprint) {
		return Pointer{}, fmt.Errorf("invalid runtime pointer %q", path)
	}
	return pointer, nil
}

func (s *Store) CurrentPointer() (Pointer, error) { return s.LoadPointer(s.paths.RuntimeCurrent) }

func (s *Store) LastKnownGoodPointer() (Pointer, error) { return s.LoadPointer(s.paths.RuntimeLKG) }

func (s *Store) LoadIntent(fingerprint string) ([]byte, error) {
	if !validFingerprint(fingerprint) {
		return nil, fmt.Errorf("invalid runtime fingerprint")
	}
	path := filepath.Join(s.paths.RuntimeIntents, fingerprint[len("sha256:"):]+".toml")
	return os.ReadFile(path)
}

func (s *Store) WriteIntent(validation Validation) error {
	if !validFingerprint(validation.Fingerprint) {
		return fmt.Errorf("invalid validation fingerprint")
	}
	path := filepath.Join(s.paths.RuntimeIntents, validation.Fingerprint[len("sha256:"):]+".toml")
	return platform.WriteAtomic(path, validation.Canonical, 0o600)
}

func (s *Store) WritePointer(path, fingerprint string) error {
	if !validFingerprint(fingerprint) {
		return fmt.Errorf("invalid runtime fingerprint")
	}
	data, err := json.MarshalIndent(Pointer{SchemaVersion: PointerSchemaVersion, Fingerprint: fingerprint}, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteAtomic(path, append(data, '\n'), 0o600)
}

func (s *Store) WriteJournal(journal Journal) error {
	if journal.SchemaVersion == 0 {
		journal.SchemaVersion = PointerSchemaVersion
	}
	if journal.SchemaVersion != PointerSchemaVersion || journal.OperationID == "" || !validFingerprint(journal.CandidateFingerprint) {
		return fmt.Errorf("invalid runtime apply journal")
	}
	if journal.PreviousFingerprint != "" && !validFingerprint(journal.PreviousFingerprint) {
		return fmt.Errorf("invalid runtime previous fingerprint")
	}
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return err
	}
	return platform.WriteAtomic(s.paths.RuntimeApply, append(data, '\n'), 0o600)
}

func (s *Store) ClearJournal() error {
	err := os.Remove(s.paths.RuntimeApply)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

// PromoteCurrentToLastKnownGood records a generation that has passed the
// Server readiness boundary. It is deliberately a separate step from writing
// current.json: a candidate is not a rollback target until the owned runtime
// has actually become ready.
func (s *Store) PromoteCurrentToLastKnownGood() error {
	current, err := s.CurrentPointer()
	if err != nil {
		return err
	}
	if current.Fingerprint == "" {
		return errors.New("runtime current pointer is empty")
	}
	if _, err := s.LoadIntent(current.Fingerprint); err != nil {
		return err
	}
	if err := s.WritePointer(s.paths.RuntimeLKG, current.Fingerprint); err != nil {
		return err
	}
	return s.ClearJournal()
}

// Reconcile validates the journal/pointer relationship. It does not infer a
// previous or candidate from mtimes and never starts a process. The caller can
// resume only the phase explicitly recorded here.
func (s *Store) Reconcile() (ReconcileResult, error) {
	data, err := os.ReadFile(s.paths.RuntimeApply)
	if errors.Is(err, fs.ErrNotExist) {
		current, currentErr := s.CurrentPointer()
		if currentErr != nil {
			return ReconcileResult{}, currentErr
		}
		lkg, lkgErr := s.LastKnownGoodPointer()
		if lkgErr != nil {
			return ReconcileResult{}, lkgErr
		}
		return ReconcileResult{Current: current, LastKnownGood: lkg}, nil
	}
	if err != nil {
		return ReconcileResult{}, fmt.Errorf("read runtime apply journal: %w", err)
	}
	var journal Journal
	if err := json.Unmarshal(data, &journal); err != nil {
		return ReconcileResult{}, fmt.Errorf("decode runtime apply journal: %w", err)
	}
	if journal.SchemaVersion != PointerSchemaVersion || !validFingerprint(journal.CandidateFingerprint) {
		return ReconcileResult{}, fmt.Errorf("unsupported or incomplete runtime apply journal")
	}
	current, err := s.CurrentPointer()
	if err != nil {
		return ReconcileResult{}, err
	}
	lkg, err := s.LastKnownGoodPointer()
	if err != nil {
		return ReconcileResult{}, err
	}
	if !fingerprintAllowed(current.Fingerprint, journal.PreviousFingerprint, journal.CandidateFingerprint) {
		return ReconcileResult{}, fmt.Errorf("runtime current pointer is outside journal candidates")
	}
	result := ReconcileResult{Journal: journal, Current: current, LastKnownGood: lkg}
	switch journal.Phase {
	case PhasePrepared, PhaseStoppingPrevious:
		if current.Fingerprint != journal.PreviousFingerprint {
			return ReconcileResult{}, fmt.Errorf("phase %q requires current pointer to remain previous", journal.Phase)
		}
		result.NeedsResume = true
	case PhasePreviousStopped:
		if current.Fingerprint != journal.PreviousFingerprint && current.Fingerprint != journal.CandidateFingerprint {
			return ReconcileResult{}, fmt.Errorf("phase %q has an unknown current pointer", journal.Phase)
		}
		result.NeedsResume = true
	case PhaseCandidateSelected, PhaseStoppingCandidate:
		result.NeedsRollback = true
	case PhaseRollbackSelected, PhaseCommitting:
		result.NeedsResume = true
	default:
		return ReconcileResult{}, fmt.Errorf("unsupported runtime apply phase %q", journal.Phase)
	}
	return result, nil
}

func validFingerprint(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	return fingerprintPattern.MatchString(strings.TrimPrefix(value, "sha256:"))
}

func fingerprintAllowed(value string, candidates ...string) bool {
	if value == "" {
		return false
	}
	for _, candidate := range candidates {
		if candidate != "" && value == candidate {
			return true
		}
	}
	return false
}

var fingerprintPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
