package config

import (
	"bytes"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// InitProfile is an operator-facing one-shot manifest generation profile.
type InitProfile string

const (
	InitProfileDockerACME          InitProfile = "docker-acme"
	InitProfileDockerExternalProxy InitProfile = "docker-external-proxy"
)

// InitOptions contains only values needed to generate a complete immutable
// Docker manifest. These are CLI generation inputs, never runtime overrides.
type InitOptions struct {
	Profile             InitProfile
	Origin              string
	Email               string
	TrustedProxyCIDRs   []string
	ApplicationStateDir string
	StorageDir          string
}

//go:embed server.container.toml
var containerManifestBase []byte

// GenerateManifest returns a complete schema-v3 manifest for one explicit
// production profile and validates it through the same schema resolver used at
// runtime.
func GenerateManifest(options InitOptions) ([]byte, error) {
	stateDir := strings.TrimSpace(options.ApplicationStateDir)
	if stateDir == "" {
		stateDir = "/data/app-state"
	}
	storageDir := strings.TrimSpace(options.StorageDir)
	if storageDir == "" {
		storageDir = "/data/storage"
	}
	origin, _, err := NormalizeOrigin(options.Origin)
	if err != nil {
		return nil, fmt.Errorf("origin must be an exact http(s) origin: %w", err)
	}

	var raw manifest
	decoder := toml.NewDecoder(bytes.NewReader(containerManifestBase)).DisallowUnknownFields()
	if err := decoder.Decode(&raw); err != nil {
		return nil, fmt.Errorf("decode embedded container manifest base: %w", err)
	}
	setString(raw.Database.Path, stateDir+"/library.sqlite3")
	setString(raw.Server.PrimaryOrigin, origin)
	setString(raw.Logging.Dir, stateDir+"/logs")
	setString(raw.Storage.Path, storageDir)
	setString(raw.Storage.CloudStatePath, stateDir+"/cloud")
	setString(raw.Storage.BackupsPath, stateDir+"/backups")
	setString(raw.Auth.SecretKeyFile, stateDir+"/secrets/lumilio_secret_key")

	switch options.Profile {
	case InitProfileDockerACME:
		if strings.TrimSpace(options.Email) == "" {
			return nil, errors.New("docker-acme requires --email")
		}
		if len(options.TrustedProxyCIDRs) != 0 {
			return nil, errors.New("docker-acme does not accept trusted proxies")
		}
		setString(raw.Server.Listen, "0.0.0.0:8443")
		setString(raw.Server.TLS.Mode, string(TLSModeACME))
		setString(raw.Server.TLS.HTTPListen, "0.0.0.0:8080")
		setString(raw.Server.TLS.Email, strings.TrimSpace(options.Email))
		setString(raw.Server.TLS.StoragePath, stateDir+"/tls")
		setString(raw.Server.Proxy.Mode, string(ProxyModeDisabled))
		*raw.Server.Proxy.TrustedCIDRs = []string{}
	case InitProfileDockerExternalProxy:
		if strings.TrimSpace(options.Email) != "" {
			return nil, errors.New("docker-external-proxy does not accept --email")
		}
		if len(options.TrustedProxyCIDRs) == 0 {
			return nil, errors.New("docker-external-proxy requires at least one --trusted-proxy")
		}
		setString(raw.Server.Listen, "0.0.0.0:6680")
		setString(raw.Server.TLS.Mode, string(TLSModeExternal))
		setString(raw.Server.TLS.HTTPListen, "")
		setString(raw.Server.TLS.Email, "")
		setString(raw.Server.TLS.StoragePath, "")
		setString(raw.Server.Proxy.Mode, string(ProxyModeRequired))
		*raw.Server.Proxy.TrustedCIDRs = append([]string(nil), options.TrustedProxyCIDRs...)
	default:
		return nil, fmt.Errorf("unsupported config init profile %q", options.Profile)
	}

	problems := validateManifestPresence(raw)
	if len(problems) == 0 {
		_, problems = resolveManifest(raw, "/")
	}
	if len(problems) != 0 {
		return nil, invalidConfig(problems)
	}
	data, err := toml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("encode generated manifest: %w", err)
	}
	return data, nil
}

func setString(target *string, value string) {
	*target = value
}
