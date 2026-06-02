package cloud

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"server/internal/db/repo"
)

type iCloudCredentialProvider struct{}

func NewICloudCredentialProvider() CredentialProvider {
	return &iCloudCredentialProvider{}
}

func (p *iCloudCredentialProvider) Descriptor() ProviderDescriptor {
	return ProviderDescriptor{
		ID:          ProviderICloud,
		Title:       "iCloud",
		Description: "Import originals from iCloud Photos.",
		Status:      ProviderStatusEnabled,
		FormFields: []ProviderField{
			{
				Name:         "username",
				Label:        "Apple ID",
				Type:         "email",
				Required:     true,
				Placeholder:  "you@icloud.com",
				Autocomplete: "username",
			},
			{
				Name:         "password",
				Label:        "App-specific password",
				Type:         "password",
				Required:     true,
				Placeholder:  "xxxx-xxxx-xxxx-xxxx",
				Autocomplete: "current-password",
			},
			{
				Name:     "domain",
				Label:    "Apple domain",
				Type:     "select",
				Required: true,
				Options: []ProviderOption{
					{Value: "com", Label: "Global iCloud"},
					{Value: "cn", Label: "Mainland China iCloud"},
				},
			},
		},
		ChallengeFields: iCloudChallengeFields(),
		SecurityNote:    "Lumilio uses the password only during authentication and stores the resulting session in an isolated credential directory.",
	}
}

func iCloudChallengeFields() []ProviderField {
	return []ProviderField{{
		Name:         "code",
		Label:        "Verification code",
		Type:         "text",
		Required:     true,
		Placeholder:  "123456",
		Autocomplete: "one-time-code",
	}}
}

func (p *iCloudCredentialProvider) Identity(inputs map[string]string) (CredentialIdentity, error) {
	username := strings.TrimSpace(inputs["username"])
	password := strings.TrimSpace(inputs["password"])
	if username == "" || password == "" {
		return CredentialIdentity{}, fmt.Errorf("username and password are required")
	}
	domain := normalizeICloudDomain(inputs["domain"])
	return CredentialIdentity{
		IdentityHash:       accountIdentifierHash(username),
		MaskedIdentity:     maskAccount(username),
		DefaultDisplayName: maskAccount(username),
		PublicConfig: map[string]string{
			"domain": domain,
		},
	}, nil
}

func (p *iCloudCredentialProvider) DefaultArtifactDir(credentialID uuid.UUID) string {
	return filepath.Join(providerArtifactRoot(ProviderICloud), credentialID.String())
}

func (p *iCloudCredentialProvider) Authenticate(ctx context.Context, input CredentialAuthInput) (CredentialAuthResult, error) {
	username := strings.TrimSpace(input.Inputs["username"])
	password := strings.TrimSpace(input.Inputs["password"])
	domain := normalizeICloudDomain(input.Inputs["domain"])
	if username == "" || password == "" {
		return CredentialAuthResult{}, fmt.Errorf("username and password are required")
	}
	if err := ensurePrivateDir(input.ArtifactDir); err != nil {
		return CredentialAuthResult{}, err
	}

	signal := &twoFASignal{}
	provider := NewICloudProvider(ICloudConfig{
		Username:  username,
		Password:  password,
		Domain:    domain,
		CookieDir: input.ArtifactDir,
	})
	provider.SetTwoFACodeGetter(signal)

	if err := provider.ForceAuth(ctx); err != nil {
		if signal.wasTriggered() {
			return CredentialAuthResult{
				Status:       CredentialStatusPendingChallenge,
				AuthStatus:   AuthStatusChallengeRequired,
				PublicConfig: input.Identity.PublicConfig,
				ArtifactDir:  input.ArtifactDir,
				Challenge:    iCloudAuthChallenge(),
				PendingState: pendingICloudAuth{provider: provider, signal: signal},
			}, nil
		}
		return CredentialAuthResult{}, fmt.Errorf("icloud authentication failed: %w", err)
	}

	return CredentialAuthResult{
		Status:       CredentialStatusConnected,
		AuthStatus:   AuthStatusConnected,
		PublicConfig: input.Identity.PublicConfig,
		ArtifactDir:  input.ArtifactDir,
	}, nil
}

func (p *iCloudCredentialProvider) VerifyChallenge(ctx context.Context, input CredentialChallengeInput) (CredentialAuthResult, error) {
	pending, ok := input.PendingState.(pendingICloudAuth)
	if !ok {
		return CredentialAuthResult{}, fmt.Errorf("pending iCloud authentication state is unavailable")
	}
	code := strings.TrimSpace(input.Inputs["code"])
	if code == "" {
		return CredentialAuthResult{}, fmt.Errorf("verification code is required")
	}

	pending.signal.setCode(code)
	if err := pending.provider.ForceAuth(ctx); err != nil {
		return CredentialAuthResult{}, fmt.Errorf("icloud challenge verification failed: %w", err)
	}

	return CredentialAuthResult{
		Status:       CredentialStatusConnected,
		AuthStatus:   AuthStatusConnected,
		PublicConfig: unmarshalPublicConfig(input.Credential.PublicConfig),
		ArtifactDir:  stringPtrValue(input.Credential.ArtifactDir),
	}, nil
}

func (p *iCloudCredentialProvider) NewImporter(ctx context.Context, credential repo.CloudCredential) (CloudProvider, error) {
	_ = ctx
	artifactDir := stringPtrValue(credential.ArtifactDir)
	if strings.TrimSpace(artifactDir) == "" {
		return nil, fmt.Errorf("cloud credential has no artifact directory")
	}
	config := unmarshalPublicConfig(credential.PublicConfig)
	return NewICloudProvider(ICloudConfig{
		Domain:    normalizeICloudDomain(config["domain"]),
		CookieDir: artifactDir,
	}), nil
}

func iCloudAuthChallenge() *AuthChallenge {
	return &AuthChallenge{
		Type:        "verification_code",
		Title:       "Verification required",
		Description: "Enter the code sent to your trusted devices.",
		Fields:      iCloudChallengeFields(),
	}
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
		return maskToken(username)
	}
	return maskToken(username[:at]) + username[at:]
}

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

func ensurePrivateDir(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return fmt.Errorf("create credential artifact dir: %w", err)
	}
	if err := os.Chmod(path, 0o700); err != nil {
		return fmt.Errorf("secure credential artifact dir: %w", err)
	}
	return nil
}

func stringPtrValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
