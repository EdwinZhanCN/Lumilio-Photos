package cloud

import (
	"context"
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

	"server/internal/cloud/icloud"
	"server/internal/db/repo"
	"server/internal/secretbox"
	"server/internal/sourcing"
)

const (
	CredentialStatusConnected        = "connected"
	CredentialStatusPendingChallenge = "pending_challenge"
	CredentialStatusDisabled         = "disabled"
	CredentialStatusError            = "error"

	ImportRunStatusQueued      = "queued"
	ImportRunStatusRunning     = "running"
	ImportRunStatusCompleted   = "completed"
	ImportRunStatusFailed      = "failed"
	ImportRunStatusInterrupted = "interrupted"
)

// CreateCloudCredentialInput holds provider-neutral credential inputs.
type CreateCloudCredentialInput struct {
	Provider        ProviderKind
	DisplayName     string
	Inputs          map[string]string
	CreatedByUserID *int32
}

// CreateCloudCredentialResult is returned after attempting provider auth.
type CreateCloudCredentialResult struct {
	Credential repo.CloudCredential
	AuthStatus string
	Challenge  *AuthChallenge
}

// VerifyCredentialChallengeInput holds provider challenge inputs.
type VerifyCredentialChallengeInput struct {
	CredentialID uuid.UUID
	Inputs       map[string]string
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

// CloudSyncService manages cloud providers, credentials, and repo-scoped imports.
type CloudSyncService interface {
	ListProviders(ctx context.Context) ([]ProviderDescriptor, error)
	ListCredentials(ctx context.Context) ([]repo.CloudCredential, error)
	CreateCredential(ctx context.Context, input CreateCloudCredentialInput) (CreateCloudCredentialResult, error)
	VerifyCredentialChallenge(ctx context.Context, input VerifyCredentialChallengeInput) (CreateCloudCredentialResult, error)
	DisconnectCredential(ctx context.Context, credentialID uuid.UUID) error
	ReconnectCredential(ctx context.Context, input ReconnectCredentialInput) (CreateCloudCredentialResult, error)
	RemoveCredential(ctx context.Context, credentialID uuid.UUID) error
	BindRepositoryCredentialAndStartImport(ctx context.Context, input BindRepositoryCredentialInput) (uuid.UUID, error)
	StartRepositoryImport(ctx context.Context, input StartRepositoryImportInput) (uuid.UUID, error)
	GetRepositoryCloudStatus(ctx context.Context, repositoryID uuid.UUID) (RepositoryCloudStatus, error)
	GetImportRun(ctx context.Context, runID uuid.UUID) (repo.CloudImportRun, error)
	RecoverInterruptedRuns(ctx context.Context) error
	ProviderTitle(provider ProviderKind) string
}

// ReconnectCredentialInput holds inputs for reconnecting a disabled/error credential.
type ReconnectCredentialInput struct {
	CredentialID uuid.UUID
	Inputs       map[string]string // optional; if empty, tries existing session
}

type pendingICloudAuth struct {
	client    *icloud.Client
	phoneID   int
	phoneMode string
}

type pendingCredentialAuth struct {
	provider ProviderKind
	state    any
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
	registry     *ProviderRegistry
	secretBox    *secretbox.Box

	mu            sync.Mutex
	pendingAuth   map[uuid.UUID]pendingCredentialAuth
	activeImports map[uuid.UUID]*activeImport // keyed by run ID
}

// NewCloudSyncService creates a CloudSyncService.
func NewCloudSyncService(
	queries *repo.Queries,
	materializer *sourcing.SourceMaterializer,
	secretKeyPath string,
	storageRoot string,
	logger *zap.Logger,
) CloudSyncService {
	if logger == nil {
		logger = zap.NewNop()
	}
	scopedLogger := logger.With(zap.String("component", "cloud_sync_service"))
	box, err := secretbox.New(strings.TrimSpace(secretKeyPath), "cloud.credentials.encryption.v1")
	if err != nil {
		scopedLogger.Warn("cloud credential secret box unavailable", zap.Error(err))
	}
	return &cloudSyncService{
		queries:       queries,
		materializer:  materializer,
		logger:        scopedLogger,
		registry:      NewDefaultProviderRegistry(storageRoot),
		secretBox:     box,
		pendingAuth:   make(map[uuid.UUID]pendingCredentialAuth),
		activeImports: make(map[uuid.UUID]*activeImport),
	}
}

// RecoverInterruptedRuns flags any import run left in queued/running (e.g. by a
// crash or restart) as interrupted.
func (s *cloudSyncService) RecoverInterruptedRuns(ctx context.Context) error {
	return s.queries.MarkStaleCloudImportRunsInterrupted(ctx)
}

func (s *cloudSyncService) ListProviders(ctx context.Context) ([]ProviderDescriptor, error) {
	_ = ctx
	providers := s.registry.List()
	descriptors := make([]ProviderDescriptor, 0, len(providers))
	for _, provider := range providers {
		descriptors = append(descriptors, provider.Descriptor())
	}
	return descriptors, nil
}

func (s *cloudSyncService) ListCredentials(ctx context.Context) ([]repo.CloudCredential, error) {
	return s.queries.ListCloudCredentials(ctx)
}

func (s *cloudSyncService) ProviderTitle(provider ProviderKind) string {
	credentialProvider, err := s.registry.Get(provider)
	if err != nil {
		return string(provider)
	}
	return credentialProvider.Descriptor().Title
}

func (s *cloudSyncService) CreateCredential(ctx context.Context, input CreateCloudCredentialInput) (CreateCloudCredentialResult, error) {
	provider, err := s.registry.Get(input.Provider)
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}

	identity, err := provider.Identity(input.Inputs)
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}

	credentialID := uuid.New()
	displayName := strings.TrimSpace(input.DisplayName)
	if displayName == "" {
		displayName = identity.DefaultDisplayName
	}
	artifactDir := provider.DefaultArtifactDir(credentialID)

	existing, err := s.queries.GetCloudCredentialByIdentity(ctx, repo.GetCloudCredentialByIdentityParams{
		Provider:     string(input.Provider),
		IdentityHash: identity.IdentityHash,
	})
	if err == nil {
		credentialID = uuid.UUID(existing.CredentialID.Bytes)
		if strings.TrimSpace(existing.DisplayName) != "" {
			displayName = existing.DisplayName
		}
		if existing.ArtifactDir != nil && strings.TrimSpace(*existing.ArtifactDir) != "" {
			artifactDir = *existing.ArtifactDir
		}
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return CreateCloudCredentialResult{}, err
	}

	authResult, err := provider.Authenticate(ctx, CredentialAuthInput{
		CredentialID: credentialID,
		DisplayName:  displayName,
		Inputs:       input.Inputs,
		ArtifactDir:  artifactDir,
		Identity:     identity,
	})
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}

	credential, err := s.upsertCredentialForAuth(ctx, credentialID, input.Provider, displayName, identity, authResult, input.CreatedByUserID)
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}
	if authResult.PendingState != nil {
		s.mu.Lock()
		s.pendingAuth[credentialID] = pendingCredentialAuth{provider: input.Provider, state: authResult.PendingState}
		s.mu.Unlock()
	}
	return CreateCloudCredentialResult{
		Credential: credential,
		AuthStatus: authResult.AuthStatus,
		Challenge:  authResult.Challenge,
	}, nil
}

func (s *cloudSyncService) VerifyCredentialChallenge(ctx context.Context, input VerifyCredentialChallengeInput) (CreateCloudCredentialResult, error) {
	s.mu.Lock()
	pending, ok := s.pendingAuth[input.CredentialID]
	s.mu.Unlock()
	if !ok {
		return CreateCloudCredentialResult{}, fmt.Errorf("no pending authentication challenge for credential")
	}

	credential, err := s.queries.GetCloudCredential(ctx, toPGUUID(input.CredentialID))
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}
	provider, err := s.registry.Get(pending.provider)
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}

	authResult, err := provider.VerifyChallenge(ctx, CredentialChallengeInput{
		Credential:   credential,
		Inputs:       input.Inputs,
		PendingState: pending.state,
	})
	if err != nil {
		_, _ = s.queries.UpdateCloudCredentialStatus(ctx, repo.UpdateCloudCredentialStatusParams{
			CredentialID: toPGUUID(input.CredentialID),
			Status:       CredentialStatusError,
		})
		return CreateCloudCredentialResult{}, err
	}

	updated, err := s.updateCredentialAuthState(ctx, credential, authResult)
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}
	s.mu.Lock()
	delete(s.pendingAuth, input.CredentialID)
	s.mu.Unlock()
	return CreateCloudCredentialResult{Credential: updated, AuthStatus: authResult.AuthStatus}, nil
}

func (s *cloudSyncService) DisconnectCredential(ctx context.Context, credentialID uuid.UUID) error {
	_, err := s.queries.UpdateCloudCredentialStatus(ctx, repo.UpdateCloudCredentialStatusParams{
		CredentialID: toPGUUID(credentialID),
		Status:       CredentialStatusDisabled,
	})
	if err == nil {
		s.cancelCredentialWork(credentialID)
	}
	return err
}

func (s *cloudSyncService) ReconnectCredential(ctx context.Context, input ReconnectCredentialInput) (CreateCloudCredentialResult, error) {
	credential, err := s.queries.GetCloudCredential(ctx, toPGUUID(input.CredentialID))
	if err != nil {
		return CreateCloudCredentialResult{}, fmt.Errorf("credential not found: %w", err)
	}
	if credential.Status == CredentialStatusConnected && len(input.Inputs) == 0 {
		return CreateCloudCredentialResult{Credential: credential, AuthStatus: AuthStatusConnected}, nil
	}

	provider, err := s.registry.Get(ProviderKind(credential.Provider))
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}

	artifactDir := stringPtrValue(credential.ArtifactDir)
	password := strings.TrimSpace(input.Inputs["password"])

	if password == "" {
		// Try existing session
		importer, err := provider.NewImporter(ctx, credential)
		if err != nil {
			return CreateCloudCredentialResult{
				Credential: credential,
				AuthStatus: AuthStatusPasswordRequired,
			}, nil
		}
		if authenticator, ok := importer.(interface{ ForceAuth(context.Context) error }); ok {
			if err := authenticator.ForceAuth(ctx); err != nil {
				return CreateCloudCredentialResult{
					Credential: credential,
					AuthStatus: AuthStatusPasswordRequired,
				}, nil
			}
		}
		// Session is valid, restore connected status
		updated, err := s.queries.UpdateCloudCredentialStatus(ctx, repo.UpdateCloudCredentialStatusParams{
			CredentialID: toPGUUID(input.CredentialID),
			Status:       CredentialStatusConnected,
		})
		if err != nil {
			return CreateCloudCredentialResult{}, err
		}
		return CreateCloudCredentialResult{Credential: updated, AuthStatus: AuthStatusConnected}, nil
	}

	// Full re-authentication with password
	identity, _ := provider.Identity(map[string]string{
		"username": credential.MaskedIdentity,
		"domain":   unmarshalPublicConfig(credential.PublicConfig)["domain"],
	})

	authResult, err := provider.Authenticate(ctx, CredentialAuthInput{
		CredentialID: input.CredentialID,
		DisplayName:  credential.DisplayName,
		Inputs:       input.Inputs,
		ArtifactDir:  artifactDir,
		Identity:     identity,
	})
	if err != nil {
		return CreateCloudCredentialResult{}, fmt.Errorf("reconnect authentication failed: %w", err)
	}

	updated, err := s.updateCredentialAuthState(ctx, credential, authResult)
	if err != nil {
		return CreateCloudCredentialResult{}, err
	}
	if authResult.PendingState != nil {
		s.mu.Lock()
		s.pendingAuth[input.CredentialID] = pendingCredentialAuth{provider: ProviderKind(credential.Provider), state: authResult.PendingState}
		s.mu.Unlock()
	}
	return CreateCloudCredentialResult{
		Credential: updated,
		AuthStatus: authResult.AuthStatus,
		Challenge:  authResult.Challenge,
	}, nil
}

func (s *cloudSyncService) RemoveCredential(ctx context.Context, credentialID uuid.UUID) error {
	credential, err := s.queries.GetCloudCredential(ctx, toPGUUID(credentialID))
	if err != nil {
		return fmt.Errorf("credential not found: %w", err)
	}

	s.cancelCredentialWork(credentialID)

	if err := s.queries.DisableRepositoryCloudBindingsByCredential(ctx, toPGUUID(credentialID)); err != nil {
		s.logger.Warn("failed to disable bindings on credential removal", zap.Error(err))
	}

	if err := s.queries.DeleteCloudCredential(ctx, toPGUUID(credentialID)); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}

	if dir := stringPtrValue(credential.ArtifactDir); dir != "" {
		if err := os.RemoveAll(dir); err != nil {
			s.logger.Warn("failed to remove credential artifact directory", zap.String("dir", dir), zap.Error(err))
		}
	}

	return nil
}

func (s *cloudSyncService) cancelCredentialWork(credentialID uuid.UUID) {
	s.mu.Lock()
	delete(s.pendingAuth, credentialID)
	for _, imp := range s.activeImports {
		if imp.credentialID == credentialID && imp.cancel != nil {
			imp.cancel()
		}
	}
	s.mu.Unlock()
}

func (s *cloudSyncService) BindRepositoryCredentialAndStartImport(ctx context.Context, input BindRepositoryCredentialInput) (uuid.UUID, error) {
	credential, err := s.queries.GetCloudCredential(ctx, toPGUUID(input.CredentialID))
	if err != nil {
		return uuid.Nil, err
	}
	if credential.Status != CredentialStatusConnected {
		return uuid.Nil, fmt.Errorf("cloud credential is not connected")
	}

	if _, err := s.registry.Get(ProviderKind(credential.Provider)); err != nil {
		return uuid.Nil, err
	}

	if _, err := s.queries.UpsertRepositoryCloudBinding(ctx, repo.UpsertRepositoryCloudBindingParams{
		RepositoryID: toPGUUID(input.RepositoryID),
		CredentialID: toPGUUID(input.CredentialID),
		Provider:     credential.Provider,
	}); err != nil {
		return uuid.Nil, err
	}

	return s.StartRepositoryImport(ctx, StartRepositoryImportInput{
		RepositoryID: input.RepositoryID,
		OwnerID:      input.OwnerID,
	})
}

func (s *cloudSyncService) StartRepositoryImport(ctx context.Context, input StartRepositoryImportInput) (uuid.UUID, error) {
	binding, err := s.queries.GetActiveRepositoryCloudBinding(ctx, toPGUUID(input.RepositoryID))
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
	if _, err := s.registry.Get(ProviderKind(credential.Provider)); err != nil {
		return uuid.Nil, err
	}

	runID := uuid.New()
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
		Provider:     credential.Provider,
		Status:       ImportRunStatusQueued,
	})
	if err != nil {
		s.finishActiveImport(runID)
		return uuid.Nil, err
	}
	if _, err := s.queries.UpdateRepositoryCloudBindingLastRun(ctx, repo.UpdateRepositoryCloudBindingLastRunParams{
		RepositoryID: toPGUUID(input.RepositoryID),
		Provider:     credential.Provider,
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
	binding, err := s.queries.GetActiveRepositoryCloudBinding(ctx, toPGUUID(repositoryID))
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
	defer s.finishActiveImport(runID)

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

	credentialProvider, err := s.registry.Get(ProviderKind(credential.Provider))
	if err != nil {
		finish(ImportRunStatusFailed, err)
		return
	}
	provider, err := credentialProvider.NewImporter(ctx, credential)
	if err != nil {
		finish(ImportRunStatusFailed, err)
		return
	}
	if authenticator, ok := provider.(interface{ ForceAuth(context.Context) error }); ok {
		if err := authenticator.ForceAuth(ctx); err != nil {
			_, _ = s.queries.UpdateCloudCredentialStatus(bookkeeping, repo.UpdateCloudCredentialStatusParams{
				CredentialID: toPGUUID(credentialID),
				Status:       CredentialStatusError,
			})
			finish(ImportRunStatusFailed, fmt.Errorf("cloud credential session is not valid; reconnect the credential"))
			return
		}
	}

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

	repository, err := s.queries.GetRepository(ctx, toPGUUID(repositoryID))
	if err != nil {
		finish(ImportRunStatusFailed, fmt.Errorf("get repository: %w", err))
		return
	}
	stagingDir := filepath.Join(repository.Path, ".lumilio", "staging", "incoming")

	stateStore := NewPGSyncStateStore(s.queries, credentialID)
	source := NewCloudImportSource(CloudImportSourceConfig{
		Provider:   provider,
		State:      stateStore,
		StagingDir: stagingDir,
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
		zap.String("provider", credential.Provider),
	)

	runErr := consumer.Run(ctx)
	close(flusherDone)
	flush()

	switch {
	case ctx.Err() != nil:
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
	provider ProviderKind,
	displayName string,
	identity CredentialIdentity,
	authResult CredentialAuthResult,
	createdByUserID *int32,
) (repo.CloudCredential, error) {
	existing, err := s.queries.GetCloudCredentialByIdentity(ctx, repo.GetCloudCredentialByIdentityParams{
		Provider:     string(provider),
		IdentityHash: identity.IdentityHash,
	})
	if err == nil {
		return s.updateCredentialAuthState(ctx, existing, authResult)
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return repo.CloudCredential{}, err
	}

	var userID *int32
	if createdByUserID != nil {
		v := *createdByUserID
		userID = &v
	}
	publicConfig, err := marshalPublicConfig(authResult.PublicConfig)
	if err != nil {
		return repo.CloudCredential{}, err
	}
	artifactDir := nullableString(authResult.ArtifactDir)
	return s.queries.CreateCloudCredential(ctx, repo.CreateCloudCredentialParams{
		CredentialID:     toPGUUID(credentialID),
		Provider:         string(provider),
		DisplayName:      displayName,
		IdentityHash:     identity.IdentityHash,
		MaskedIdentity:   identity.MaskedIdentity,
		Status:           authResult.Status,
		PublicConfig:     publicConfig,
		SecretCiphertext: authResult.SecretCiphertext,
		ArtifactDir:      artifactDir,
		CreatedByUserID:  userID,
	})
}

func (s *cloudSyncService) updateCredentialAuthState(ctx context.Context, credential repo.CloudCredential, authResult CredentialAuthResult) (repo.CloudCredential, error) {
	publicConfig := authResult.PublicConfig
	if publicConfig == nil {
		publicConfig = unmarshalPublicConfig(credential.PublicConfig)
	}
	publicConfigBytes, err := marshalPublicConfig(publicConfig)
	if err != nil {
		return repo.CloudCredential{}, err
	}
	secretCiphertext := authResult.SecretCiphertext
	if secretCiphertext == nil {
		secretCiphertext = credential.SecretCiphertext
	}
	artifactDir := authResult.ArtifactDir
	if strings.TrimSpace(artifactDir) == "" {
		artifactDir = stringPtrValue(credential.ArtifactDir)
	}
	return s.queries.UpdateCloudCredentialAuthState(ctx, repo.UpdateCloudCredentialAuthStateParams{
		CredentialID:     credential.CredentialID,
		Status:           authResult.Status,
		PublicConfig:     publicConfigBytes,
		SecretCiphertext: secretCiphertext,
		ArtifactDir:      nullableString(artifactDir),
	})
}

func nullableString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}
