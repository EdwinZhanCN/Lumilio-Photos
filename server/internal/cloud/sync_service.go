package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/sourcing"
)

const (
	CredentialStatusConnected  = "connected"
	CredentialStatusPending2FA = "pending_2fa"
	CredentialStatusDisabled   = "disabled"
	CredentialStatusError      = "error"

	ImportRunStatusQueued      = "queued"
	ImportRunStatusRunning     = "running"
	ImportRunStatusCompleted   = "completed"
	ImportRunStatusFailed      = "failed"
	ImportRunStatusInterrupted = "interrupted"
)

// CreateICloudCredentialInput holds the credentials for creating an iCloud connection.
type CreateICloudCredentialInput struct {
	Username        string
	Password        string
	Domain          string
	DisplayName     string
	CreatedByUserID *int32
}

// CreateICloudCredentialResult is returned after attempting iCloud auth.
type CreateICloudCredentialResult struct {
	Credential repo.CloudCredential
	Needs2FA   bool
}

// VerifyICloudCredential2FAInput holds the 2FA verification code.
type VerifyICloudCredential2FAInput struct {
	CredentialID uuid.UUID
	Code         string
}

// StartRepositoryImportInput identifies a repository import request.
type StartRepositoryImportInput struct {
	RepositoryID uuid.UUID
	OwnerID      *int32
}

// BindRepositoryCredentialInput binds a repo to a credential and starts import.
type BindRepositoryCredentialInput struct {
	RepositoryID uuid.UUID
	CredentialID uuid.UUID
	OwnerID      *int32
}

// RepositoryCloudStatus describes a repository's cloud binding and latest run.
type RepositoryCloudStatus struct {
	Binding    *repo.RepositoryCloudBinding
	Credential *repo.CloudCredential
	LatestRun  *repo.CloudImportRun
}

// CloudSyncService manages cloud credentials and repo-scoped imports.
type CloudSyncService interface {
	ListCredentials(ctx context.Context) ([]repo.CloudCredential, error)
	CreateICloudCredential(ctx context.Context, input CreateICloudCredentialInput) (CreateICloudCredentialResult, error)
	VerifyICloudCredential2FA(ctx context.Context, input VerifyICloudCredential2FAInput) error
	DisableCredential(ctx context.Context, credentialID uuid.UUID) error
	BindRepositoryCredentialAndStartImport(ctx context.Context, input BindRepositoryCredentialInput) (uuid.UUID, error)
	StartRepositoryImport(ctx context.Context, input StartRepositoryImportInput) (uuid.UUID, error)
	GetRepositoryCloudStatus(ctx context.Context, repositoryID uuid.UUID) (RepositoryCloudStatus, error)
	GetImportRun(ctx context.Context, runID uuid.UUID) (repo.CloudImportRun, error)
	RecoverInterruptedRuns(ctx context.Context) error
}

type pendingICloudAuth struct {
	provider *ICloudProvider
	signal   *twoFASignal
}

// twoFASignal is a TextGetter implementation that signals when 2FA is needed.
type twoFASignal struct {
	mu        sync.Mutex
	code      string
	triggered bool
}

func (s *twoFASignal) GetText(tip string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.code != "" {
		return s.code, nil
	}
	s.triggered = true
	return "", fmt.Errorf("2FA required")
}

func (s *twoFASignal) setCode(code string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.code = code
}

func (s *twoFASignal) wasTriggered() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.triggered
}

// activeImport tracks an in-flight import run so it can be single-flighted per
// repository and cancelled (e.g. when its credential is disabled).
type activeImport struct {
	repoID       uuid.UUID
	credentialID uuid.UUID
	cancel       context.CancelFunc
}

type cloudSyncService struct {
	queries      *repo.Queries
	materializer *sourcing.SourceMaterializer
	logger       *zap.Logger

	mu            sync.Mutex
	pendingICloud map[uuid.UUID]pendingICloudAuth
	activeImports map[uuid.UUID]*activeImport // keyed by run ID
}

// NewCloudSyncService creates a CloudSyncService.
func NewCloudSyncService(
	queries *repo.Queries,
	materializer *sourcing.SourceMaterializer,
	logger *zap.Logger,
) CloudSyncService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &cloudSyncService{
		queries:       queries,
		materializer:  materializer,
		logger:        logger.With(zap.String("component", "cloud_sync_service")),
		pendingICloud: make(map[uuid.UUID]pendingICloudAuth),
		activeImports: make(map[uuid.UUID]*activeImport),
	}
}

// RecoverInterruptedRuns flags any import run left in queued/running (e.g. by a
// crash or restart) as interrupted, so repositories are not stuck with a
// permanently "running" import that blocks new imports in the UI.
func (s *cloudSyncService) RecoverInterruptedRuns(ctx context.Context) error {
	return s.queries.MarkStaleCloudImportRunsInterrupted(ctx)
}

func (s *cloudSyncService) ListCredentials(ctx context.Context) ([]repo.CloudCredential, error) {
	return s.queries.ListCloudCredentials(ctx)
}

func (s *cloudSyncService) CreateICloudCredential(ctx context.Context, input CreateICloudCredentialInput) (CreateICloudCredentialResult, error) {
	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password == "" {
		return CreateICloudCredentialResult{}, fmt.Errorf("username and password are required")
	}
	domain := normalizeICloudDomain(input.Domain)
	accountHash := accountIdentifierHash(username)

	credentialID := uuid.New()
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = maskAccount(username)
	}

	cookieDir := credentialCookieDir(credentialID)
	existing, err := s.queries.GetCloudCredentialByAccount(ctx, repo.GetCloudCredentialByAccountParams{
		Provider:              string(ProviderICloud),
		AccountIdentifierHash: accountHash,
		Domain:                domain,
	})
	if err == nil {
		credentialID = uuid.UUID(existing.CredentialID.Bytes)
		cookieDir = existing.CookieDir
		displayName = existing.DisplayName
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CreateICloudCredentialResult{}, err
	}

	if err := ensurePrivateDir(cookieDir); err != nil {
		return CreateICloudCredentialResult{}, err
	}

	signal := &twoFASignal{}
	provider := NewICloudProvider(ICloudConfig{
		Username:  username,
		Password:  password,
		Domain:    domain,
		CookieDir: cookieDir,
	})
	provider.SetTwoFACodeGetter(signal)

	if err := provider.ForceAuth(ctx); err != nil {
		if signal.wasTriggered() {
			credential, saveErr := s.upsertCredentialForAuth(ctx, credentialID, displayName, accountHash, maskAccount(username), domain, cookieDir, CredentialStatusPending2FA, input.CreatedByUserID)
			if saveErr != nil {
				return CreateICloudCredentialResult{}, saveErr
			}
			s.mu.Lock()
			s.pendingICloud[credentialID] = pendingICloudAuth{provider: provider, signal: signal}
			s.mu.Unlock()
			return CreateICloudCredentialResult{Credential: credential, Needs2FA: true}, nil
		}
		return CreateICloudCredentialResult{}, fmt.Errorf("icloud authentication failed: %w", err)
	}

	credential, err := s.upsertCredentialForAuth(ctx, credentialID, displayName, accountHash, maskAccount(username), domain, cookieDir, CredentialStatusConnected, input.CreatedByUserID)
	if err != nil {
		return CreateICloudCredentialResult{}, err
	}
	return CreateICloudCredentialResult{Credential: credential}, nil
}

func (s *cloudSyncService) VerifyICloudCredential2FA(ctx context.Context, input VerifyICloudCredential2FAInput) error {
	code := strings.TrimSpace(input.Code)
	if code == "" {
		return fmt.Errorf("2FA code is required")
	}

	s.mu.Lock()
	pending, ok := s.pendingICloud[input.CredentialID]
	s.mu.Unlock()
	if !ok {
		return fmt.Errorf("no pending iCloud 2FA session for credential")
	}

	pending.signal.setCode(code)
	if err := pending.provider.ForceAuth(ctx); err != nil {
		_, _ = s.queries.UpdateCloudCredentialStatus(ctx, repo.UpdateCloudCredentialStatusParams{
			CredentialID: toPGUUID(input.CredentialID),
			Status:       CredentialStatusError,
		})
		return fmt.Errorf("icloud 2FA verification failed: %w", err)
	}

	if _, err := s.queries.UpdateCloudCredentialStatus(ctx, repo.UpdateCloudCredentialStatusParams{
		CredentialID: toPGUUID(input.CredentialID),
		Status:       CredentialStatusConnected,
	}); err != nil {
		return err
	}

	s.mu.Lock()
	delete(s.pendingICloud, input.CredentialID)
	s.mu.Unlock()
	return nil
}

func (s *cloudSyncService) DisableCredential(ctx context.Context, credentialID uuid.UUID) error {
	_, err := s.queries.UpdateCloudCredentialStatus(ctx, repo.UpdateCloudCredentialStatusParams{
		CredentialID: toPGUUID(credentialID),
		Status:       CredentialStatusDisabled,
	})
	if err == nil {
		s.mu.Lock()
		delete(s.pendingICloud, credentialID)
		// Cancel any in-flight imports using this credential; their goroutines
		// clean up their own registry entries on exit.
		for _, imp := range s.activeImports {
			if imp.credentialID == credentialID && imp.cancel != nil {
				imp.cancel()
			}
		}
		s.mu.Unlock()
	}
	return err
}

func (s *cloudSyncService) BindRepositoryCredentialAndStartImport(ctx context.Context, input BindRepositoryCredentialInput) (uuid.UUID, error) {
	credential, err := s.queries.GetCloudCredential(ctx, toPGUUID(input.CredentialID))
	if err != nil {
		return uuid.Nil, err
	}
	if credential.Status != CredentialStatusConnected {
		return uuid.Nil, fmt.Errorf("cloud credential is not connected")
	}

	if _, err := s.queries.UpsertRepositoryCloudBinding(ctx, repo.UpsertRepositoryCloudBindingParams{
		RepositoryID: toPGUUID(input.RepositoryID),
		CredentialID: toPGUUID(input.CredentialID),
		Provider:     string(ProviderICloud),
	}); err != nil {
		return uuid.Nil, err
	}

	return s.StartRepositoryImport(ctx, StartRepositoryImportInput{
		RepositoryID: input.RepositoryID,
		OwnerID:      input.OwnerID,
	})
}

func (s *cloudSyncService) StartRepositoryImport(ctx context.Context, input StartRepositoryImportInput) (uuid.UUID, error) {
	binding, err := s.queries.GetRepositoryCloudBinding(ctx, repo.GetRepositoryCloudBindingParams{
		RepositoryID: toPGUUID(input.RepositoryID),
		Provider:     string(ProviderICloud),
	})
	if err != nil {
		return uuid.Nil, err
	}
	if !binding.Enabled {
		return uuid.Nil, fmt.Errorf("repository cloud binding is disabled")
	}

	credential, err := s.queries.GetCloudCredential(ctx, binding.CredentialID)
	if err != nil {
		return uuid.Nil, err
	}
	if credential.Status != CredentialStatusConnected {
		return uuid.Nil, fmt.Errorf("cloud credential is not connected")
	}

	runID := uuid.New()

	// Single-flight per repository: refuse to launch a second concurrent import
	// for the same repository so runs cannot overlap (double download/count).
	s.mu.Lock()
	for _, imp := range s.activeImports {
		if imp.repoID == input.RepositoryID {
			s.mu.Unlock()
			return uuid.Nil, fmt.Errorf("an import is already running for this repository")
		}
	}
	entry := &activeImport{repoID: input.RepositoryID, credentialID: uuid.UUID(binding.CredentialID.Bytes)}
	s.activeImports[runID] = entry
	s.mu.Unlock()

	run, err := s.queries.CreateCloudImportRun(ctx, repo.CreateCloudImportRunParams{
		RunID:        toPGUUID(runID),
		RepositoryID: toPGUUID(input.RepositoryID),
		CredentialID: binding.CredentialID,
		Provider:     string(ProviderICloud),
		Status:       ImportRunStatusQueued,
	})
	if err != nil {
		s.finishActiveImport(runID)
		return uuid.Nil, err
	}
	if _, err := s.queries.UpdateRepositoryCloudBindingLastRun(ctx, repo.UpdateRepositoryCloudBindingLastRunParams{
		RepositoryID: toPGUUID(input.RepositoryID),
		Provider:     string(ProviderICloud),
		LastImportRunID: pgtype.UUID{
			Bytes: uuid.UUID(run.RunID.Bytes),
			Valid: true,
		},
	}); err != nil {
		s.finishActiveImport(runID)
		return uuid.Nil, err
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.mu.Lock()
	entry.cancel = cancel
	s.mu.Unlock()

	go s.runImport(runCtx, run, credential, input.OwnerID)
	return uuid.UUID(run.RunID.Bytes), nil
}

// finishActiveImport removes a run from the active registry and releases its
// cancel function. Safe to call from the launching path or the run goroutine.
func (s *cloudSyncService) finishActiveImport(runID uuid.UUID) {
	s.mu.Lock()
	entry, ok := s.activeImports[runID]
	if ok {
		delete(s.activeImports, runID)
	}
	s.mu.Unlock()
	if ok && entry.cancel != nil {
		entry.cancel()
	}
}

func (s *cloudSyncService) GetRepositoryCloudStatus(ctx context.Context, repositoryID uuid.UUID) (RepositoryCloudStatus, error) {
	binding, err := s.queries.GetRepositoryCloudBinding(ctx, repo.GetRepositoryCloudBindingParams{
		RepositoryID: toPGUUID(repositoryID),
		Provider:     string(ProviderICloud),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return RepositoryCloudStatus{}, nil
		}
		return RepositoryCloudStatus{}, err
	}

	credential, err := s.queries.GetCloudCredential(ctx, binding.CredentialID)
	if err != nil {
		return RepositoryCloudStatus{}, err
	}

	status := RepositoryCloudStatus{
		Binding:    &binding,
		Credential: &credential,
	}
	if binding.LastImportRunID.Valid {
		run, err := s.queries.GetCloudImportRun(ctx, binding.LastImportRunID)
		if err == nil {
			status.LatestRun = &run
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return RepositoryCloudStatus{}, err
		}
	}
	return status, nil
}

func (s *cloudSyncService) GetImportRun(ctx context.Context, runID uuid.UUID) (repo.CloudImportRun, error) {
	return s.queries.GetCloudImportRun(ctx, toPGUUID(runID))
}

func (s *cloudSyncService) runImport(ctx context.Context, run repo.CloudImportRun, credential repo.CloudCredential, ownerID *int32) {
	runID := uuid.UUID(run.RunID.Bytes)
	repositoryID := uuid.UUID(run.RepositoryID.Bytes)
	credentialID := uuid.UUID(run.CredentialID.Bytes)

	// Release the single-flight slot (and cancel func) when the run ends.
	defer s.finishActiveImport(runID)

	// Run bookkeeping (status + counts) must persist even when ctx is cancelled
	// mid-import, so it uses a context detached from the cancellable import ctx.
	bookkeeping := context.Background()

	if _, err := s.queries.MarkCloudImportRunStarted(bookkeeping, toPGUUID(runID)); err != nil {
		s.logger.Error("failed to mark cloud import started", zap.String("run_id", runID.String()), zap.Error(err))
		return
	}

	finish := func(status string, err error) {
		var errorText *string
		if err != nil {
			text := err.Error()
			errorText = &text
		}
		if _, finishErr := s.queries.FinishCloudImportRun(bookkeeping, repo.FinishCloudImportRunParams{
			RunID:  toPGUUID(runID),
			Status: status,
			Error:  errorText,
		}); finishErr != nil {
			s.logger.Error("failed to finish cloud import run", zap.String("run_id", runID.String()), zap.Error(finishErr))
		}
	}

	if strings.TrimSpace(credential.CookieDir) == "" {
		err := fmt.Errorf("cloud credential has no cookie directory")
		finish(ImportRunStatusFailed, err)
		return
	}

	provider := NewICloudProvider(ICloudConfig{
		Domain:    credential.Domain,
		CookieDir: credential.CookieDir,
	})
	if err := provider.ForceAuth(ctx); err != nil {
		_, _ = s.queries.UpdateCloudCredentialStatus(bookkeeping, repo.UpdateCloudCredentialStatusParams{
			CredentialID: toPGUUID(credentialID),
			Status:       CredentialStatusError,
		})
		finish(ImportRunStatusFailed, fmt.Errorf("icloud session is not valid; reconnect the credential"))
		return
	}

	// Coalesce progress: the producer (discovery) and consumer (materialize)
	// goroutines both report per-file deltas. Accumulate them and flush on a
	// timer into a single counts UPDATE instead of one UPDATE per delta.
	var pmu sync.Mutex
	var acc ImportProgressDelta
	progress := func(delta ImportProgressDelta) {
		pmu.Lock()
		acc.TotalSeen += delta.TotalSeen
		acc.Downloaded += delta.Downloaded
		acc.Imported += delta.Imported
		acc.Skipped += delta.Skipped
		acc.Failed += delta.Failed
		pmu.Unlock()
	}
	flush := func() {
		pmu.Lock()
		d := acc
		acc = ImportProgressDelta{}
		pmu.Unlock()
		if d == (ImportProgressDelta{}) {
			return
		}
		if _, err := s.queries.IncrementCloudImportRunCounts(bookkeeping, repo.IncrementCloudImportRunCountsParams{
			RunID:           toPGUUID(runID),
			TotalSeen:       d.TotalSeen,
			DownloadedCount: d.Downloaded,
			ImportedCount:   d.Imported,
			SkippedCount:    d.Skipped,
			FailedCount:     d.Failed,
		}); err != nil {
			s.logger.Warn("failed to update cloud import progress", zap.String("run_id", runID.String()), zap.Error(err))
		}
	}

	flusherDone := make(chan struct{})
	go func() {
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-flusherDone:
				return
			case <-ticker.C:
				flush()
			}
		}
	}()

	stateStore := NewPGSyncStateStore(s.queries, credentialID)
	source := NewCloudImportSource(CloudImportSourceConfig{
		Provider:   provider,
		State:      stateStore,
		StagingDir: defaultStagingDir(),
		RepoID:     repositoryID,
		OwnerID:    ownerID,
		OnProgress: progress,
		Logger:     s.logger,
	})
	consumer := NewCloudSyncConsumer(source, s.materializer, stateStore, progress, s.logger)

	s.logger.Info("cloud import started",
		zap.String("run_id", runID.String()),
		zap.String("repository_id", repositoryID.String()),
		zap.String("credential_id", credentialID.String()),
	)

	runErr := consumer.Run(ctx)
	close(flusherDone)
	flush() // persist any remaining counts before flipping status

	switch {
	case ctx.Err() != nil:
		// Cancelled (e.g. credential disabled or shutdown): not a failure.
		finish(ImportRunStatusInterrupted, nil)
	case runErr != nil:
		finish(ImportRunStatusFailed, runErr)
	default:
		finish(ImportRunStatusCompleted, nil)
	}
}

func (s *cloudSyncService) upsertCredentialForAuth(
	ctx context.Context,
	credentialID uuid.UUID,
	displayName string,
	accountHash string,
	maskedAccount string,
	domain string,
	cookieDir string,
	status string,
	createdByUserID *int32,
) (repo.CloudCredential, error) {
	existing, err := s.queries.GetCloudCredentialByAccount(ctx, repo.GetCloudCredentialByAccountParams{
		Provider:              string(ProviderICloud),
		AccountIdentifierHash: accountHash,
		Domain:                domain,
	})
	if err == nil {
		return s.queries.UpdateCloudCredentialStatus(ctx, repo.UpdateCloudCredentialStatusParams{
			CredentialID: existing.CredentialID,
			Status:       status,
		})
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return repo.CloudCredential{}, err
	}

	var userID *int32
	if createdByUserID != nil {
		v := *createdByUserID
		userID = &v
	}
	return s.queries.CreateCloudCredential(ctx, repo.CreateCloudCredentialParams{
		CredentialID:          toPGUUID(credentialID),
		Provider:              string(ProviderICloud),
		DisplayName:           displayName,
		AccountIdentifierHash: accountHash,
		MaskedAccount:         maskedAccount,
		Domain:                domain,
		Status:                status,
		CookieDir:             cookieDir,
		CreatedByUserID:       userID,
	})
}

func normalizeICloudDomain(domain string) string {
	switch strings.ToLower(strings.TrimSpace(domain)) {
	case "cn":
		return "cn"
	default:
		return "com"
	}
}

func accountIdentifierHash(username string) string {
	sum := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(username))))
	return hex.EncodeToString(sum[:])
}

func maskAccount(username string) string {
	username = strings.TrimSpace(username)
	if username == "" {
		return ""
	}
	at := strings.Index(username, "@")
	if at < 0 {
		// Not an email-shaped identifier; mask the whole token.
		return maskToken(username)
	}
	return maskToken(username[:at]) + username[at:]
}

// maskToken obscures a token while keeping a hint of its first/last character.
func maskToken(s string) string {
	switch len(s) {
	case 0:
		return ""
	case 1:
		return "*"
	case 2:
		return s[:1] + "*"
	default:
		return s[:1] + strings.Repeat("*", len(s)-2) + s[len(s)-1:]
	}
}

func credentialCookieDir(credentialID uuid.UUID) string {
	return filepath.Join(defaultICloudCookieDir(), credentialID.String())
}

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create credential cookie dir: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure credential cookie dir: %w", err)
	}
	return nil
}

func defaultStagingDir() string {
	storagePath := strings.TrimSpace(os.Getenv("STORAGE_PATH"))
	if storagePath != "" {
		normalized := filepath.Clean(storagePath)
		if strings.EqualFold(filepath.Base(normalized), "primary") {
			normalized = filepath.Dir(normalized)
		}
		return filepath.Join(normalized, ".cloud-staging")
	}
	return filepath.Join("data", "storage", ".cloud-staging")
}
