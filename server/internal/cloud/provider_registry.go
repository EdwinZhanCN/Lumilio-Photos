package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"server/internal/db/repo"
)

const (
	ProviderStatusEnabled = "enabled"

	AuthStatusConnected         = "connected"
	AuthStatusChallengeRequired = "challenge_required"
	AuthStatusPasswordRequired  = "password_required"
)

// ProviderOption describes a select option for provider forms.
type ProviderOption struct {
	Value string
	Label string
}

// ProviderField describes a provider-specific input.
type ProviderField struct {
	Name         string
	Label        string
	Type         string
	Required     bool
	Placeholder  string
	HelpText     string
	Options      []ProviderOption
	Autocomplete string
}

// ProviderDescriptor is a UI-safe cloud provider description.
type ProviderDescriptor struct {
	ID              ProviderKind
	Title           string
	Description     string
	Status          string
	FormFields      []ProviderField
	ChallengeFields []ProviderField
	SecurityNote    string
}

// AuthChallenge describes a pending provider authentication challenge.
type AuthChallenge struct {
	Type        string
	Title       string
	Description string
	Fields      []ProviderField
}

type CredentialIdentity struct {
	IdentityHash       string
	MaskedIdentity     string
	DefaultDisplayName string
	PublicConfig       map[string]string
}

type CredentialAuthInput struct {
	CredentialID uuid.UUID
	DisplayName  string
	Inputs       map[string]string
	ArtifactDir  string
	Identity     CredentialIdentity
}

type CredentialAuthResult struct {
	Status           string
	AuthStatus       string
	PublicConfig     map[string]string
	SecretCiphertext []byte
	ArtifactDir      string
	Challenge        *AuthChallenge
	PendingState     any
}

type CredentialChallengeInput struct {
	Credential   repo.CloudCredential
	Inputs       map[string]string
	PendingState any
}

type CredentialProvider interface {
	Descriptor() ProviderDescriptor
	Identity(inputs map[string]string) (CredentialIdentity, error)
	DefaultArtifactDir(credentialID uuid.UUID) string
	Authenticate(ctx context.Context, input CredentialAuthInput) (CredentialAuthResult, error)
	VerifyChallenge(ctx context.Context, input CredentialChallengeInput) (CredentialAuthResult, error)
	NewImporter(ctx context.Context, credential repo.CloudCredential) (CloudProvider, error)
}

type ProviderRegistry struct {
	providers map[ProviderKind]CredentialProvider
	order     []ProviderKind
}

func NewProviderRegistry(providers ...CredentialProvider) *ProviderRegistry {
	registry := &ProviderRegistry{
		providers: make(map[ProviderKind]CredentialProvider, len(providers)),
		order:     make([]ProviderKind, 0, len(providers)),
	}
	for _, provider := range providers {
		id := provider.Descriptor().ID
		registry.providers[id] = provider
		registry.order = append(registry.order, id)
	}
	return registry
}

func NewDefaultProviderRegistry() *ProviderRegistry {
	return NewProviderRegistry(NewICloudCredentialProvider())
}

func (r *ProviderRegistry) List() []CredentialProvider {
	if r == nil {
		return nil
	}
	items := make([]CredentialProvider, 0, len(r.order))
	for _, id := range r.order {
		if provider := r.providers[id]; provider != nil {
			items = append(items, provider)
		}
	}
	return items
}

func (r *ProviderRegistry) Get(id ProviderKind) (CredentialProvider, error) {
	if r == nil {
		return nil, fmt.Errorf("cloud provider registry is not configured")
	}
	provider := r.providers[id]
	if provider == nil {
		return nil, fmt.Errorf("unsupported cloud provider: %s", id)
	}
	return provider, nil
}

func marshalPublicConfig(config map[string]string) ([]byte, error) {
	if config == nil {
		config = map[string]string{}
	}
	data, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("marshal provider public config: %w", err)
	}
	return data, nil
}

func unmarshalPublicConfig(data []byte) map[string]string {
	if len(data) == 0 {
		return map[string]string{}
	}
	var config map[string]string
	if err := json.Unmarshal(data, &config); err != nil || config == nil {
		return map[string]string{}
	}
	return config
}

func providerArtifactRoot(provider ProviderKind) string {
	storagePath := strings.TrimSpace(os.Getenv("STORAGE_PATH"))
	if storagePath != "" {
		normalized := filepath.Clean(storagePath)
		if strings.EqualFold(filepath.Base(normalized), "primary") {
			normalized = filepath.Dir(normalized)
		}
		return filepath.Join(normalized, ".cloud", string(provider))
	}
	return filepath.Join("data", "storage", ".cloud", string(provider))
}
