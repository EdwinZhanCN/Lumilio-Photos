package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"strings"
	"time"

	"github.com/pelletier/go-toml/v2"
)

type runtimeApplyPhase string

const (
	applyCandidateStaged   runtimeApplyPhase = "candidate_staged"
	applyCandidatePromoted runtimeApplyPhase = "candidate_promoted"
	applyRollingBack       runtimeApplyPhase = "rolling_back"
)

type runtimeApplyJournal struct {
	Phase                runtimeApplyPhase `json:"phase"`
	BaseFingerprint      string            `json:"base_fingerprint"`
	CandidateFingerprint string            `json:"candidate_fingerprint"`
	StartedAt            time.Time         `json:"started_at"`
	CandidateError       string            `json:"candidate_error,omitempty"`
	RollbackError        string            `json:"rollback_error,omitempty"`
}

type RuntimeConfigValidationError struct {
	Issues []ConfigIssue
}

func (e *RuntimeConfigValidationError) Error() string {
	messages := make([]string, 0, len(e.Issues))
	for _, issue := range e.Issues {
		messages = append(messages, issue.Message)
	}
	if len(messages) == 0 {
		return "runtime candidate is invalid"
	}
	return strings.Join(messages, "; ")
}

func (s *Supervisor) writeApplyJournal(journal runtimeApplyJournal) error {
	data, err := json.MarshalIndent(journal, "", "  ")
	if err != nil {
		return fmt.Errorf("encode runtime apply journal: %w", err)
	}
	if err := writeAtomicPrivate(s.paths.RuntimeApplyJournalFile(), data); err != nil {
		return fmt.Errorf("write runtime apply journal: %w", err)
	}
	return nil
}

func (s *Supervisor) cleanupRuntimeApply() error {
	var cleanupErr error
	for _, path := range []string{
		s.paths.RuntimeCandidateFile(),
		s.paths.RuntimeApplyJournalFile(),
	} {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			cleanupErr = errors.Join(cleanupErr, err)
		}
	}
	return cleanupErr
}

func (s *Supervisor) reconcileRuntimeApply() error {
	data, err := os.ReadFile(s.paths.RuntimeApplyJournalFile())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read runtime apply journal: %w", err)
	}
	var journal runtimeApplyJournal
	if err := json.Unmarshal(data, &journal); err != nil {
		return fmt.Errorf("decode runtime apply journal: %w", err)
	}
	switch journal.Phase {
	case applyCandidateStaged:
		return s.cleanupRuntimeApply()
	case applyCandidatePromoted, applyRollingBack:
		lkg, err := os.ReadFile(s.paths.RuntimeLastKnownGoodFile())
		if err != nil {
			return fmt.Errorf("reconcile runtime apply from last-known-good: %w", err)
		}
		if err := writeAtomicPrivate(s.paths.RuntimeConfigFile(), lkg); err != nil {
			return fmt.Errorf("restore runtime intent during reconciliation: %w", err)
		}
		return s.cleanupRuntimeApply()
	default:
		return fmt.Errorf("unsupported runtime apply journal phase %q", journal.Phase)
	}
}

// ApplyRuntimeConfigAsync validates and stages a candidate before returning.
// The claimed operation gate remains held by the asynchronous stop/promote/
// readiness/rollback sequence, so another mutation receives a deterministic
// ErrOperationInProgress.
func (s *Supervisor) ApplyRuntimeConfigAsync(
	ctx context.Context,
	baseFingerprint string,
	candidate string,
	acceptLANWarning bool,
) (RuntimeConfigValidation, error) {
	if !s.operationMu.TryLock() {
		return RuntimeConfigValidation{}, ErrOperationInProgress
	}
	validation, err := s.ValidateRuntimeConfig(baseFingerprint, candidate)
	if err != nil {
		s.operationMu.Unlock()
		return RuntimeConfigValidation{}, err
	}
	if !validation.Valid {
		s.operationMu.Unlock()
		return validation, &RuntimeConfigValidationError{Issues: validation.Issues}
	}
	if validation.Network.Mode == NetworkLANHTTP {
		settings, settingsErr := LoadSettings(s.paths.DesktopSettingsFile())
		if settingsErr != nil {
			s.operationMu.Unlock()
			return RuntimeConfigValidation{}, settingsErr
		}
		if !acceptLANWarning &&
			settings.LANHTTPWarningAcceptedVersion < lanHTTPWarningCurrentVersion {
			issue := ConfigIssue{
				Field: "server.listen", Code: "lan_warning_required",
				Message: "LAN HTTP requires acknowledgement of the unencrypted-network warning",
			}
			validation.Valid = false
			validation.Issues = append(validation.Issues, issue)
			s.operationMu.Unlock()
			return validation, &RuntimeConfigValidationError{Issues: validation.Issues}
		}
		if acceptLANWarning {
			settings.LANHTTPWarningAcceptedVersion = lanHTTPWarningCurrentVersion
			if err := SaveSettings(s.paths.DesktopSettingsFile(), settings); err != nil {
				s.operationMu.Unlock()
				return RuntimeConfigValidation{}, err
			}
		}
	}
	candidateData := []byte(validation.CandidateTOML)
	if err := writeAtomicPrivate(s.paths.RuntimeCandidateFile(), candidateData); err != nil {
		s.operationMu.Unlock()
		return RuntimeConfigValidation{}, fmt.Errorf("stage runtime candidate: %w", err)
	}
	journal := runtimeApplyJournal{
		Phase:                applyCandidateStaged,
		BaseFingerprint:      baseFingerprint,
		CandidateFingerprint: runtimeFingerprint(candidateData),
		StartedAt:            time.Now().UTC(),
	}
	if err := s.writeApplyJournal(journal); err != nil {
		_ = s.cleanupRuntimeApply()
		s.operationMu.Unlock()
		return RuntimeConfigValidation{}, err
	}
	go func() {
		defer s.operationMu.Unlock()
		s.applyStagedRuntimeLocked(ctx, journal)
	}()
	return validation, nil
}

func (s *Supervisor) applyStagedRuntimeLocked(ctx context.Context, journal runtimeApplyJournal) {
	candidate, err := os.ReadFile(s.paths.RuntimeCandidateFile())
	if err != nil {
		s.failRuntime(fmt.Errorf("read staged runtime candidate: %w", err))
		return
	}
	wasRunning := s.generation != nil
	snapshot := s.RuntimeSnapshot()
	snapshot.Phase = RuntimeRestarting
	snapshot.ErrorCode = ""
	snapshot.ErrorMessage = ""
	snapshot.OperationActive = true
	s.setSnapshot(snapshot)

	if err := s.stopGenerationLocked(); err != nil {
		_ = s.cleanupRuntimeApply()
		s.failRuntime(fmt.Errorf("stop runtime before candidate promotion: %w", err))
		return
	}
	if err := writeAtomicPrivate(s.paths.RuntimeConfigFile(), candidate); err != nil {
		_ = s.cleanupRuntimeApply()
		if wasRunning {
			if restartErr := s.startRuntimeLocked(ctx, RuntimeRestarting); restartErr != nil {
				err = errors.Join(err, fmt.Errorf("restart current runtime after promotion failure: %w", restartErr))
			}
		}
		s.failRuntime(fmt.Errorf("promote runtime candidate: %w", err))
		return
	}
	journal.Phase = applyCandidatePromoted
	if err := s.writeApplyJournal(journal); err != nil {
		s.failRuntime(err)
		return
	}
	if candidateErr := s.startRuntimeLockedWithOperation(ctx, RuntimeRestarting, true); candidateErr == nil {
		if err := writeAtomicPrivate(s.paths.RuntimeLastKnownGoodFile(), candidate); err != nil {
			s.failRuntime(fmt.Errorf("update last-known-good runtime: %w", err))
			return
		}
		if err := s.cleanupRuntimeApply(); err != nil {
			s.logf("cleanup successful runtime apply: %v", err)
		}
		s.updateSnapshot(func(snapshot *RuntimeSnapshot) {
			snapshot.LastKnownGoodAvailable = true
			snapshot.OperationActive = false
		})
		return
	} else {
		s.updateSnapshot(func(snapshot *RuntimeSnapshot) {
			snapshot.Phase = RuntimeRestarting
			snapshot.ErrorCode = ""
			snapshot.ErrorMessage = ""
			snapshot.OperationActive = true
		})
		journal.Phase = applyRollingBack
		journal.CandidateError = runtimeErrorMessage(candidateErr)
		if err := s.writeApplyJournal(journal); err != nil {
			s.failRuntime(errors.Join(candidateErr, err))
			return
		}
		lkg, err := os.ReadFile(s.paths.RuntimeLastKnownGoodFile())
		if err != nil {
			journal.RollbackError = err.Error()
			_ = s.writeApplyJournal(journal)
			s.failRuntime(errors.Join(candidateErr, fmt.Errorf("read last-known-good runtime: %w", err)))
			return
		}
		if err := writeAtomicPrivate(s.paths.RuntimeConfigFile(), lkg); err != nil {
			journal.RollbackError = err.Error()
			_ = s.writeApplyJournal(journal)
			s.failRuntime(errors.Join(candidateErr, fmt.Errorf("restore last-known-good runtime: %w", err)))
			return
		}
		if rollbackErr := s.startRuntimeLockedWithOperation(ctx, RuntimeRestarting, true); rollbackErr != nil {
			journal.RollbackError = runtimeErrorMessage(rollbackErr)
			_ = s.writeApplyJournal(journal)
			s.failRuntime(errors.Join(
				fmt.Errorf("candidate runtime rejected: %w", candidateErr),
				fmt.Errorf("last-known-good runtime failed: %w", rollbackErr),
			))
			return
		}
		if err := s.cleanupRuntimeApply(); err != nil {
			s.logf("cleanup rolled-back runtime apply: %v", err)
		}
		s.updateSnapshot(func(snapshot *RuntimeSnapshot) {
			snapshot.ErrorCode = "candidate_rolled_back"
			snapshot.ErrorMessage = "Candidate runtime failed readiness and was rolled back: " +
				runtimeErrorMessage(candidateErr)
			snapshot.LastKnownGoodAvailable = true
			snapshot.OperationActive = false
		})
	}
}

func (s *Supervisor) RestoreLastKnownGoodAsync(ctx context.Context) (RuntimeConfigValidation, error) {
	if err := s.ensurePaths(); err != nil {
		return RuntimeConfigValidation{}, err
	}
	view, err := s.ReadRuntimeConfig()
	if err != nil {
		return RuntimeConfigValidation{}, err
	}
	lkg, err := os.ReadFile(s.paths.RuntimeLastKnownGoodFile())
	if err != nil {
		return RuntimeConfigValidation{}, fmt.Errorf("read last-known-good runtime: %w", err)
	}
	candidate, err := candidateWithCurrentHostFields([]byte(view.CurrentTOML), lkg)
	if err != nil {
		return RuntimeConfigValidation{}, err
	}
	return s.ApplyRuntimeConfigAsync(ctx, view.BaseFingerprint, string(candidate), true)
}

func candidateWithCurrentHostFields(current, candidate []byte) ([]byte, error) {
	currentDocument, err := parseRuntimeDocument(current)
	if err != nil {
		return nil, err
	}
	candidateDocument, err := parseRuntimeDocument(candidate)
	if err != nil {
		return nil, err
	}
	for _, path := range hostManagedRuntimePaths {
		value, ok := runtimePathValue(currentDocument, path)
		if !ok {
			return nil, fmt.Errorf("current runtime is missing host-managed field %s", path)
		}
		setRuntimePath(candidateDocument, path, value)
	}
	data, err := toml.Marshal(candidateDocument)
	if err != nil {
		return nil, fmt.Errorf("encode last-known-good candidate: %w", err)
	}
	return data, nil
}
