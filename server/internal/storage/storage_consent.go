package storage

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"server/internal/db/dbtypes"
	"server/internal/db/repo"
	"server/internal/storage/marker"

	"github.com/google/uuid"
)

// StorageIntent names the command a create-target assessment was made for.
// Consent is bound to the intent: acknowledging risks for one command never
// carries over to another.
type StorageIntent string

const (
	IntentCreateRepository StorageIntent = "create_repository"
	IntentSetupPrimary     StorageIntent = "setup_primary"
)

// Typed refusals derived from the assessment classification. Handlers map
// each to its registered Problem type; none carries a host path.
var (
	ErrStorageAssessmentStale         = errors.New("storage assessment no longer matches the target")
	ErrStorageUnsupportedFilesystem   = errors.New("storage target is on an unsupported filesystem")
	ErrStorageObservationUnknown      = errors.New("storage target could not be observed")
	ErrStoragePermissionDenied        = errors.New("storage target access was denied")
	ErrStorageMaterializationRequired = errors.New("storage target holds files that must be downloaded first")
)

// StorageRiskConfirmationError is the risk gate's refusal. It carries the
// risks the administrator must acknowledge and the assessment token those
// acknowledgements are bound to, so the client can review and retry the same
// command without a second round trip.
type StorageRiskConfirmationError struct {
	Risks []StorageRisk
	Token string
}

func (e *StorageRiskConfirmationError) Error() string {
	return fmt.Sprintf("%v: %s", ErrRepositoryRiskConfirmationRequired, joinRisks(e.Risks))
}

func (e *StorageRiskConfirmationError) Unwrap() error { return ErrRepositoryRiskConfirmationRequired }

// AssessCreateTargetRequest selects the target of a create-intent assessment
// by registered Storage Location and direct-child folder, never by host path.
type AssessCreateTargetRequest struct {
	Intent            StorageIntent
	StorageLocationID string
	DirectoryName     string
}

// CreateTargetAssessment is the Server's review of one create target: the
// P1 observation and classification plus the consent token that binds an
// acknowledgement to exactly these facts.
type CreateTargetAssessment struct {
	Intent              StorageIntent
	StorageLocationID   uuid.UUID
	StorageLocationKind dbtypes.StorageLocationKind
	DirectoryName       string
	// Path is the resolved target. It never leaves the Server.
	Path       string
	Assessment StorageAssessment
	// Risks are the acknowledgements the create command requires.
	Risks []StorageRisk
	Token string
}

// Problem is the classification's refusal code, empty when the target can be
// reviewed for creation.
func (a *CreateTargetAssessment) Problem() StorageProblemCode {
	return a.Assessment.Classification.Problem
}

// RefusalError maps the classification to the typed error the create command
// returns before any consent is considered.
func (a *CreateTargetAssessment) RefusalError() error {
	switch a.Problem() {
	case "":
		return nil
	case ProblemUnsupportedFilesystem:
		return ErrStorageUnsupportedFilesystem
	case ProblemNotMountedVolume:
		return ErrRepositoryExistingTargetNotMountPoint
	case ProblemPermissionDenied:
		return ErrStoragePermissionDenied
	case ProblemMaterializationRequired:
		return ErrStorageMaterializationRequired
	default:
		return ErrStorageObservationUnknown
	}
}

// ExistingRepositoryID is the identity of a valid, unregistered or registered
// Repository marker already at the target, or empty.
func (a *CreateTargetAssessment) ExistingRepositoryID() string {
	if a.Assessment.Observation.RepositoryMarker.State != marker.Valid {
		return ""
	}
	return a.Assessment.Observation.RepositoryMarker.UUID
}

// AssessCreateTarget resolves the create target of a command and samples it
// through the installed observer. It performs no mutation and no probe.
func (rm *DefaultRepositoryManager) AssessCreateTarget(ctx context.Context, request AssessCreateTargetRequest) (*CreateTargetAssessment, error) {
	role, directoryName, err := createIntentRole(request.Intent, request.DirectoryName)
	if err != nil {
		return nil, err
	}
	location, err := rm.resolveStorageLocationForCreate(ctx, request.StorageLocationID, role)
	if err != nil {
		return nil, err
	}
	repoPath, err := resolveRepositoryCreatePath(location.Path, directoryName, role)
	if err != nil {
		return nil, err
	}
	return rm.assessResolvedCreateTarget(ctx, request.Intent, location, directoryName, repoPath), nil
}

func createIntentRole(intent StorageIntent, directoryName string) (dbtypes.RepoRole, string, error) {
	switch intent {
	case IntentSetupPrimary:
		return dbtypes.RepoRolePrimary, "primary", nil
	case IntentCreateRepository:
		if err := ValidateRepositoryDirectoryName(directoryName); err != nil {
			return "", "", err
		}
		return dbtypes.RepoRoleRegular, directoryName, nil
	default:
		return "", "", fmt.Errorf("%w: unknown storage intent %q", ErrPathNotAllowed, intent)
	}
}

func createIntentForRole(role dbtypes.RepoRole) StorageIntent {
	if normalizeRepoRole(role) == dbtypes.RepoRolePrimary {
		return IntentSetupPrimary
	}
	return IntentCreateRepository
}

func (rm *DefaultRepositoryManager) observerOrDefault() StorageObserver {
	if rm.observer != nil {
		return rm.observer
	}
	return OSStorageObserver{}
}

// assessResolvedCreateTarget is the shared half of review and command: both
// sample the actual target, so a token minted by one matches the other.
func (rm *DefaultRepositoryManager) assessResolvedCreateTarget(ctx context.Context, intent StorageIntent, location *repo.StorageLocation, directoryName, repoPath string) *CreateTargetAssessment {
	result := &CreateTargetAssessment{Intent: intent, DirectoryName: directoryName, Path: repoPath}
	nonDefault := false
	if location != nil {
		result.StorageLocationID = location.StorageLocationID
		result.StorageLocationKind = location.Kind
		nonDefault = location.Kind != dbtypes.StorageLocationKindDefault
	}
	result.Assessment = AssessStorageTarget(ctx, rm.observerOrDefault(), repoPath, nonDefault, "")
	result.Risks = append([]StorageRisk(nil), result.Assessment.Classification.Risks...)
	result.Token = rm.consentToken(createTargetFingerprint(result))
	return result
}

// createTargetFingerprint is the fact set an acknowledgement is bound to:
// target identity, intent, the risk vocabulary, and the marker and mount facts
// that decide the classification. Capacity figures and volatile inode or
// grouping facts are excluded on purpose, so free-space fluctuation never
// invalidates a review while a changed mount or marker always does.
func createTargetFingerprint(a *CreateTargetAssessment) string {
	risks := make([]string, 0, len(a.Risks))
	for _, risk := range a.Risks {
		risks = append(risks, string(risk))
	}
	sort.Strings(risks)
	observed := a.Assessment.Observation
	fields := []string{
		"lumilio.storage.consent.v1",
		string(a.Intent),
		a.StorageLocationID.String(),
		string(a.StorageLocationKind),
		a.DirectoryName,
		filepath.Clean(a.Path),
		string(a.Assessment.Classification.Support),
		string(a.Assessment.Classification.Problem),
		strings.Join(risks, ","),
		string(observed.Access.State),
		fmt.Sprintf("empty=%t", observed.Empty),
		fmt.Sprintf("placeholder=%t", observed.Placeholder),
		observed.Volume.Filesystem,
		observed.Mount.Platform,
		observed.Mount.MountID,
		observed.Mount.MountSource,
		observed.Mount.MountPath,
		observed.Mount.Device,
		fmt.Sprintf("remote=%t", observed.Mount.Remote),
		fmt.Sprintf("removable=%t", observed.Mount.Removable),
		observed.Mount.CloudProvider,
		string(observed.RootMarker.State),
		observed.RootMarker.UUID,
		string(observed.RepositoryMarker.State),
		observed.RepositoryMarker.UUID,
	}
	return strings.Join(fields, "\x00")
}

const consentTokenVersion = "v1."

// consentToken keys the fingerprint with a per-process secret, so a token is
// opaque to clients, cannot be forged for another target, and expires with
// the Server process: a restarted Server asks for a fresh review.
func (rm *DefaultRepositoryManager) consentToken(fingerprint string) string {
	rm.consentOnce.Do(func() {
		key := make([]byte, 32)
		if _, err := rand.Read(key); err != nil {
			panic("generate storage consent key: " + err.Error())
		}
		rm.consentKey = key
	})
	mac := hmac.New(sha256.New, rm.consentKey)
	mac.Write([]byte(fingerprint))
	return consentTokenVersion + hex.EncodeToString(mac.Sum(nil))
}

// acknowledgesRisks reports whether every required risk was acknowledged by
// its exact code. Extra or unknown codes are ignored.
func acknowledgesRisks(acknowledged []string, required []StorageRisk) bool {
	seen := make(map[string]struct{}, len(acknowledged))
	for _, code := range acknowledged {
		seen[strings.TrimSpace(code)] = struct{}{}
	}
	for _, risk := range required {
		if _, ok := seen[string(risk)]; !ok {
			return false
		}
	}
	return true
}

func joinRisks(risks []StorageRisk) string {
	codes := make([]string, 0, len(risks))
	for _, risk := range risks {
		codes = append(codes, string(risk))
	}
	return strings.Join(codes, ", ")
}

// RiskCodes renders risks as their stable wire codes.
func RiskCodes(risks []StorageRisk) []string {
	if len(risks) == 0 {
		return nil
	}
	codes := make([]string, 0, len(risks))
	for _, risk := range risks {
		codes = append(codes, string(risk))
	}
	return codes
}
