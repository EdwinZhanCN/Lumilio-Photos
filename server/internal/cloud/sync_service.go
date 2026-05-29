package cloud

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
	"go.uber.org/zap"

	"server/internal/db/repo"
	"server/internal/service"
	"server/internal/sourcing"
)

// CloudProviderStatus describes a configured cloud provider's current state.
type CloudProviderStatus struct {
	Provider        ProviderKind `json:"provider"`
	SyncMode        SyncMode     `json:"sync_mode"`
	Connected       bool         `json:"connected"`
	LastCursor      string       `json:"last_cursor,omitempty"`
	SyncedFileCount int64        `json:"synced_file_count"`
}

// ConnectICloudInput holds the configuration for connecting to iCloud.
type ConnectICloudInput struct {
	Username string
	Password string
	Domain   string // "com" (default) or "cn"
	SyncMode SyncMode
}

// VerifyICloud2FAInput holds the 2FA verification code.
type VerifyICloud2FAInput struct {
	Code string
}

// TriggerSyncInput specifies which provider to sync.
type TriggerSyncInput struct {
	Provider     ProviderKind
	RepositoryID uuid.UUID
	OwnerID      *int32
}

// CloudSyncService manages cloud provider connections and sync operations.
type CloudSyncService interface {
	ConnectICloud(ctx context.Context, input ConnectICloudInput) (needs2FA bool, err error)
	VerifyICloud2FA(ctx context.Context, input VerifyICloud2FAInput) error
	ListProviders(ctx context.Context) ([]CloudProviderStatus, error)
	TriggerSync(ctx context.Context, input TriggerSyncInput) error
	Disconnect(ctx context.Context, provider ProviderKind, repositoryID uuid.UUID) error
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

type cloudSyncService struct {
	queries      *repo.Queries
	materializer *sourcing.SourceMaterializer
	assetService service.AssetService
	logger       *zap.Logger

	mu               sync.Mutex
	icloudProvider   *ICloudProvider
	icloudSyncMode   SyncMode
	icloudSignal     *twoFASignal
	activeSyncCancel context.CancelFunc
}

// NewCloudSyncService creates a CloudSyncService.
func NewCloudSyncService(
	queries *repo.Queries,
	materializer *sourcing.SourceMaterializer,
	assetService service.AssetService,
	logger *zap.Logger,
) CloudSyncService {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &cloudSyncService{
		queries:      queries,
		materializer: materializer,
		assetService: assetService,
		logger:       logger.With(zap.String("component", "cloud_sync_service")),
	}
}

func (s *cloudSyncService) ConnectICloud(ctx context.Context, input ConnectICloudInput) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	username := strings.TrimSpace(input.Username)
	password := strings.TrimSpace(input.Password)
	if username == "" || password == "" {
		return false, fmt.Errorf("username and password are required")
	}

	domain := strings.TrimSpace(input.Domain)
	if domain == "" {
		domain = "com"
	}

	syncMode := input.SyncMode
	if syncMode == "" {
		syncMode = SyncModeImport
	}

	signal := &twoFASignal{}
	provider := NewICloudProvider(ICloudConfig{
		Username: username,
		Password: password,
		Domain:   domain,
	})
	provider.SetTwoFACodeGetter(signal)

	if err := provider.ForceAuth(ctx); err != nil {
		if signal.wasTriggered() {
			s.icloudProvider = provider
			s.icloudSyncMode = syncMode
			s.icloudSignal = signal
			return true, nil
		}
		return false, fmt.Errorf("icloud authentication failed: %w", err)
	}

	s.icloudProvider = provider
	s.icloudSyncMode = syncMode
	s.icloudSignal = nil
	return false, nil
}

func (s *cloudSyncService) VerifyICloud2FA(ctx context.Context, input VerifyICloud2FAInput) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	code := strings.TrimSpace(input.Code)
	if code == "" {
		return fmt.Errorf("2FA code is required")
	}

	if s.icloudProvider == nil || s.icloudSignal == nil {
		return fmt.Errorf("no pending iCloud connection; call ConnectICloud first")
	}

	s.icloudSignal.setCode(code)

	if err := s.icloudProvider.ForceAuth(ctx); err != nil {
		return fmt.Errorf("icloud 2FA verification failed: %w", err)
	}

	s.icloudSignal = nil
	return nil
}

func (s *cloudSyncService) restoreICloudLocked(ctx context.Context) error {
	if s.icloudProvider != nil {
		return nil
	}

	provider := NewICloudProvider(ICloudConfig{
		Domain: "com",
	})
	if err := provider.ForceAuth(ctx); err != nil {
		return err
	}

	s.icloudProvider = provider
	s.icloudSyncMode = SyncModeImport
	if provider.client != nil && provider.client.Data != nil && provider.client.Data.DsInfo != nil {
		provider.config.Username = provider.client.Data.DsInfo.AppleId
	}
	return nil
}

func (s *cloudSyncService) ListProviders(ctx context.Context) ([]CloudProviderStatus, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.icloudProvider == nil {
		_ = s.restoreICloudLocked(ctx)
	}

	var statuses []CloudProviderStatus

	if s.icloudProvider != nil {
		status := CloudProviderStatus{
			Provider:  ProviderICloud,
			SyncMode:  s.icloudSyncMode,
			Connected: s.icloudProvider.IsAuthenticated(),
		}
		statuses = append(statuses, status)
	}

	return statuses, nil
}

func (s *cloudSyncService) TriggerSync(ctx context.Context, input TriggerSyncInput) error {
	s.mu.Lock()

	if s.icloudProvider == nil {
		_ = s.restoreICloudLocked(ctx)
	}

	if s.icloudProvider == nil || !s.icloudProvider.IsAuthenticated() {
		s.mu.Unlock()
		return fmt.Errorf("iCloud provider not connected")
	}

	if s.activeSyncCancel != nil {
		s.activeSyncCancel()
	}

	syncCtx, syncCancel := context.WithCancel(context.Background())
	s.activeSyncCancel = syncCancel
	s.mu.Unlock()

	go func() {
		defer syncCancel()

		stateStore := NewPGSyncStateStore(s.queries)
		stagingDir := defaultStagingDir()

		source := NewCloudImportSource(CloudImportSourceConfig{
			Provider:   s.icloudProvider,
			State:      stateStore,
			StagingDir: stagingDir,
			SyncMode:   s.icloudSyncMode,
			RepoID:     input.RepositoryID,
			OwnerID:    input.OwnerID,
			Logger:     s.logger,
		})

		consumer := NewCloudSyncConsumer(
			source,
			s.materializer,
			s.assetService,
			stateStore,
			s.logger,
		)

		s.logger.Info("cloud sync started",
			zap.String("provider", string(input.Provider)),
			zap.String("repository_id", input.RepositoryID.String()),
		)

		if err := consumer.Run(syncCtx); err != nil {
			s.logger.Error("cloud sync failed", zap.String("provider", string(input.Provider)), zap.Error(err))
			return
		}

		s.logger.Info("cloud sync completed", zap.String("provider", string(input.Provider)))
	}()

	return nil
}

func (s *cloudSyncService) Disconnect(ctx context.Context, provider ProviderKind, repositoryID uuid.UUID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.activeSyncCancel != nil {
		s.activeSyncCancel()
		s.activeSyncCancel = nil
	}

	switch provider {
	case ProviderICloud:
		s.icloudProvider = nil
		s.icloudSyncMode = ""
		s.icloudSignal = nil
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
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
