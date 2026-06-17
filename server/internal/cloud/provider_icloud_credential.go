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

	"server/internal/cloud/icloud"
	"server/internal/db/repo"
)

type iCloudCredentialProvider struct {
	storageRoot string
}

func NewICloudCredentialProvider(storageRoot string) CredentialProvider {
	return &iCloudCredentialProvider{storageRoot: storageRoot}
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
				Label:        "Password",
				Type:         "password",
				Required:     true,
				Placeholder:  "",
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
	return filepath.Join(providerArtifactRoot(p.storageRoot, ProviderICloud), credentialID.String())
}

func (p *iCloudCredentialProvider) Authenticate(ctx context.Context, input CredentialAuthInput) (CredentialAuthResult, error) {
	_ = ctx
	username := strings.TrimSpace(input.Inputs["username"])
	password := strings.TrimSpace(input.Inputs["password"])
	domain := normalizeICloudDomain(input.Inputs["domain"])
	if username == "" || password == "" {
		return CredentialAuthResult{}, fmt.Errorf("username and password are required")
	}
	if err := ensurePrivateDir(input.ArtifactDir); err != nil {
		return CredentialAuthResult{}, err
	}

	client, err := icloud.NewClient(&icloud.ClientOption{
		AppID:     username,
		Password:  password,
		CookieDir: input.ArtifactDir,
		Domain:    domain,
	})
	if err != nil {
		return CredentialAuthResult{}, fmt.Errorf("create icloud client: %w", err)
	}

	if err := client.SignIn(password); err != nil {
		return CredentialAuthResult{}, fmt.Errorf("icloud authentication failed: %w", err)
	}

	if !client.IsRequires2FA() {
		if err := client.Flush(); err != nil {
			return CredentialAuthResult{}, fmt.Errorf("persist icloud session: %w", err)
		}
		return CredentialAuthResult{
			Status:       CredentialStatusConnected,
			AuthStatus:   AuthStatusConnected,
			PublicConfig: input.Identity.PublicConfig,
			ArtifactDir:  input.ArtifactDir,
		}, nil
	}

	phones, err := client.GetTrustedPhoneNumbers()
	if err != nil {
		return CredentialAuthResult{}, fmt.Errorf("get trusted phone numbers: %w", err)
	}
	if len(phones) == 0 {
		return CredentialAuthResult{}, fmt.Errorf("no trusted phone numbers available for SMS verification")
	}

	phone := phones[0]
	mode := phone.PushMode
	if mode == "" {
		mode = "sms"
	}

	if err := client.RequestSMSCode(phone.ID, mode); err != nil {
		return CredentialAuthResult{}, fmt.Errorf("request SMS verification code: %w", err)
	}

	return CredentialAuthResult{
		Status:       CredentialStatusPendingChallenge,
		AuthStatus:   AuthStatusChallengeRequired,
		PublicConfig: input.Identity.PublicConfig,
		ArtifactDir:  input.ArtifactDir,
		Challenge:    iCloudSMSChallenge(phone.NumberWithDialCode),
		PendingState: pendingICloudAuth{client: client, phoneID: phone.ID, phoneMode: mode},
	}, nil
}

func (p *iCloudCredentialProvider) VerifyChallenge(ctx context.Context, input CredentialChallengeInput) (CredentialAuthResult, error) {
	_ = ctx
	pending, ok := input.PendingState.(pendingICloudAuth)
	if !ok {
		return CredentialAuthResult{}, fmt.Errorf("pending iCloud authentication state is unavailable")
	}
	code := strings.TrimSpace(input.Inputs["code"])
	if code == "" {
		return CredentialAuthResult{}, fmt.Errorf("verification code is required")
	}

	if err := pending.client.VerifySMSCode(pending.phoneID, code, pending.phoneMode); err != nil {
		return CredentialAuthResult{}, fmt.Errorf("icloud SMS verification failed: %w", err)
	}

	if err := pending.client.Flush(); err != nil {
		return CredentialAuthResult{}, fmt.Errorf("persist icloud session: %w", err)
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

func iCloudSMSChallenge(maskedPhone string) *AuthChallenge {
	desc := "Enter the verification code sent via SMS."
	if maskedPhone != "" {
		desc = fmt.Sprintf("Enter the verification code sent to %s.", maskedPhone)
	}
	return &AuthChallenge{
		Type:        "verification_code",
		Title:       "Verification required",
		Description: desc,
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
