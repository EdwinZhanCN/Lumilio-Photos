package config

import (
	"errors"
	"fmt"
	"strings"
)

// ProfileName is the stable identity of one deployment shape. Names marked
// Operator below are also `server config init --profile` values, so renaming
// one breaks a documented command line.
type ProfileName string

const (
	ProfileDevVite        ProfileName = "dev-vite"
	ProfileDesktopLocal   ProfileName = "desktop-local"
	ProfileDesktopLANHTTP ProfileName = "desktop-lan-http"
	ProfileDockerHTTP     ProfileName = "docker-http"
	ProfileDockerCaddy    ProfileName = "docker-caddy"
	ProfileDockerACME     ProfileName = "docker-acme"
)

// ProfileInputs are the values an operator supplies for a profile that cannot
// be fully determined in advance. They are one-shot generation inputs and never
// runtime overrides: the generated manifest is complete on its own.
type ProfileInputs struct {
	Hostname          string
	Email             string
	Listen            string
	TrustedProxyCIDRs []string
	StateDir          string
	StorageDir        string
}

// Profile is one fully-specified point in the deployment matrix. Because TOML
// comments cannot express conditional legality ("http_listen is non-empty only
// when tls.mode is acme"), a complete valid manifest per scenario is what
// actually documents the matrix; Notes carries the cross-field reasoning that
// no single key can own.
type Profile struct {
	Name ProfileName
	// Path is the file's location under server/config/examples.
	Path string
	// Summary is the one-line headline in the generated file.
	Summary string
	// Scenario points back at the plan section this profile implements.
	Scenario string
	// Notes are the header comment lines: invariants, warnings, and the
	// operational preconditions that are not visible in any single key.
	Notes []string
	// Operator marks profiles `server config init` will generate. The rest
	// document shapes compiled by the Desktop host.
	Operator bool
	// Defaults supply placeholder inputs when generating the example file.
	Defaults ProfileInputs

	build func(ProfileInputs) manifest
}

// Profiles returns every deployment shape in documentation order.
func Profiles() []Profile { return append([]Profile(nil), profileTable...) }

// ProfileByName resolves one profile by its stable name.
func ProfileByName(name ProfileName) (Profile, bool) {
	for _, profile := range profileTable {
		if profile.Name == name {
			return profile, true
		}
	}
	return Profile{}, false
}

// ProfileNames lists the names `server config init` accepts.
func ProfileNames(operatorOnly bool) []string {
	var names []string
	for _, profile := range profileTable {
		if operatorOnly && !profile.Operator {
			continue
		}
		names = append(names, string(profile.Name))
	}
	return names
}

// Build renders this profile into a complete manifest, then proves it through
// the same presence check and resolver the runtime loader uses. A profile can
// therefore never emit a manifest the server would reject.
func (p Profile) Build(inputs ProfileInputs) (manifest, error) {
	if inputs.Hostname == "" {
		inputs.Hostname = p.Defaults.Hostname
	}
	if inputs.Email == "" {
		inputs.Email = p.Defaults.Email
	}
	if strings.TrimSpace(inputs.Listen) == "" {
		inputs.Listen = p.Defaults.Listen
	}
	if len(inputs.TrustedProxyCIDRs) == 0 {
		inputs.TrustedProxyCIDRs = p.Defaults.TrustedProxyCIDRs
	}
	if strings.TrimSpace(inputs.StateDir) == "" {
		inputs.StateDir = p.Defaults.StateDir
	}
	if strings.TrimSpace(inputs.StorageDir) == "" {
		inputs.StorageDir = p.Defaults.StorageDir
	}
	if inputs.Hostname != "" {
		inputs.Hostname = normalizeHostname(inputs.Hostname)
		if !validACMEHostname(inputs.Hostname) {
			return manifest{}, errors.New("hostname must be a registrable DNS hostname")
		}
	}

	raw := p.build(inputs)
	problems := validateManifestPresence(raw)
	if len(problems) == 0 {
		_, problems = resolveManifest(raw, "/")
	}
	if len(problems) != 0 {
		return manifest{}, fmt.Errorf("profile %s: %w", p.Name, invalidConfig(problems))
	}
	return raw, nil
}

func ptr[T any](value T) *T { return &value }

// layout is the set of paths that move together between a development tree, a
// Desktop app-data directory, and a container volume.
type layout struct {
	database   string
	queueDB    string
	logs       string
	storage    string
	cloudState string
	backups    string
	secretKey  string
	webRoot    string
}

func containerLayout(stateDir, storageDir string) layout {
	stateDir = strings.TrimRight(strings.TrimSpace(stateDir), "/")
	if stateDir == "" {
		stateDir = "/data/app-state"
	}
	storageDir = strings.TrimRight(strings.TrimSpace(storageDir), "/")
	if storageDir == "" {
		storageDir = "/data/storage"
	}
	return layout{
		database:   stateDir + "/library.sqlite3",
		queueDB:    stateDir + "/river.sqlite3",
		logs:       stateDir + "/logs",
		storage:    storageDir,
		cloudState: stateDir + "/cloud",
		backups:    stateDir + "/backups",
		secretKey:  stateDir + "/secrets/lumilio_secret_key",
		webRoot:    "/app/web",
	}
}

func developmentLayout(stateDir, storageDir string) layout {
	l := containerLayout(stateDir, storageDir)
	l.webRoot = ""
	return l
}

func desktopLayout() layout {
	const state = "/Users/example/Library/Application Support/Lumilio"
	return layout{
		database:   state + "/app-state/library.sqlite3",
		queueDB:    state + "/app-state/river.sqlite3",
		logs:       state + "/app-state/logs",
		storage:    "/Users/example/Pictures/Lumilio",
		cloudState: state + "/app-state/cloud",
		backups:    state + "/app-state/backups",
		secretKey:  state + "/app-state/secrets/lumilio_secret_key",
		webRoot:    "/Applications/Lumilio.app/Contents/Resources/web",
	}
}

// baseManifest holds every value that does not vary with the network shape.
// Profiles then override only what their scenario actually changes, which keeps
// the diff between two example files equal to the difference between the two
// deployments.
func baseManifest(environment string, deploymentID string, logLevel string, l layout) manifest {
	return manifest{
		SchemaVersion: ptr(SchemaVersion),
		Environment:   ptr(environment),
		Database:      &databaseManifest{Path: ptr(l.database), QueuePath: ptr(l.queueDB)},
		Server: &serverManifest{
			Listen:             ptr("127.0.0.1:6680"),
			CORSAllowedOrigins: ptr([]string{}),
			WebRoot:            ptr(l.webRoot),
			TLS: &tlsManifest{
				Mode:        ptr(string(TLSModeOff)),
				Hostname:    ptr(""),
				HTTPListen:  ptr(""),
				Email:       ptr(""),
				StoragePath: ptr(""),
			},
			Proxy: &proxyManifest{
				TrustedCIDRs: ptr([]string{}),
			},
		},
		Logging: &loggingManifest{
			Level:                  ptr(logLevel),
			Dir:                    ptr(l.logs),
			ConsoleFormat:          ptr("console"),
			FileFormat:             ptr("json"),
			RepositoryAuditVerbose: ptr(false),
		},
		Storage: &storageManifest{
			Path:           ptr(l.storage),
			CloudStatePath: ptr(l.cloudState),
			BackupsPath:    ptr(l.backups),
		},
		RepositoryScan: &repositoryScanManifest{
			IntervalSeconds: ptr(300),
			SettleSeconds:   ptr(5),
		},
		Auth: &authManifest{
			SecretKeyFile:   ptr(l.secretKey),
			AccessTokenTTL:  ptr("15m"),
			RefreshTokenTTL: ptr("168h"),
			MediaTokenTTL:   ptr("10m"),
			Passkey: &passkeyManifest{
				Enabled: ptr(true),
				Name:    ptr("Lumilio Photos"),
			},
			RateLimit: &authRateLimitManifest{
				IPAttempts:      ptr(60),
				SubjectAttempts: ptr(8),
				Window:          ptr("1m"),
				Lockout:         ptr("5m"),
				MaxEntries:      ptr(10000),
			},
		},
		Transcode: &transcodeManifest{HardwareAccel: ptr("auto")},
		Lumen: &lumenManifest{
			DiscoveryEnabled:     ptr(true),
			DiscoveryMDNSEnabled: ptr(true),
			DiscoveryHubURL:      ptr(""),
			DiscoveryStaticNodes: ptr([]string{}),
			DiscoveryServiceType: ptr("_lumen._tcp"),
			DiscoveryDomain:      ptr("local"),
			DeploymentID:         ptr(deploymentID),
			ResolveTimeout:       ptr("3s"),
			ConnectTimeout:       ptr("3s"),
			FailureCooldownMin:   ptr("10s"),
			FailureCooldownMax:   ptr("2m"),
			ScanInterval:         ptr("30s"),
			ChunkAuto:            ptr(true),
			ChunkThresholdBytes:  ptr(1048576),
			ChunkMaxBytes:        ptr(262144),
		},
		Tools: &toolsManifest{
			ExifToolPath: ptr("exiftool"),
			FFmpegPath:   ptr("ffmpeg"),
			FFprobePath:  ptr("ffprobe"),
		},
		Execution: &executionManifest{
			CPU:                  ptr(3),
			DiskIO:               ptr(4),
			ImageCodec:           ptr(2),
			VideoCodec:           ptr(1),
			Inference:            ptr(2),
			MemoryMiB:            ptr(int64(768)),
			MacroWorkers:         ptr(16),
			MaxWaiting:           ptr(256),
			FFmpegThreads:        ptr(3),
			FFmpegSoftwarePreset: ptr("veryfast"),
		},
	}
}

func desktopBase() manifest {
	m := baseManifest("production", "desktop", "info", desktopLayout())
	// Desktop supervises its optional local Lumen Hub on loopback. mDNS remains
	// enabled so an explicitly operated LAN Hub can still augment or replace it.
	m.Lumen.DiscoveryStaticNodes = ptr([]string{"127.0.0.1:50051"})
	return m
}

func dockerBase(inputs ProfileInputs) manifest {
	return baseManifest("production", "container", "info", containerLayout(inputs.StateDir, inputs.StorageDir))
}

var profileTable = []Profile{
	{
		Name:     ProfileDevVite,
		Path:     "dev/vite.toml",
		Summary:  "Local development behind the Vite development proxy.",
		Scenario: "dynamic request origin development",
		Operator: true,
		Notes: []string{
			"`make dev` publishes Vite on 0.0.0.0:6657 and keeps the API on",
			"127.0.0.1:6680. Browser origin and passkey identity come from each request.",
		},
		Defaults: ProfileInputs{StateDir: "../state", StorageDir: "../storage"},
		build: func(inputs ProfileInputs) manifest {
			m := baseManifest("development", "local", "debug", developmentLayout(inputs.StateDir, inputs.StorageDir))
			m.Server.Proxy.TrustedCIDRs = ptr([]string{"127.0.0.1/32", "::1/128"})
			return m
		},
	},
	{
		Name:     ProfileDesktopLocal,
		Path:     "desktop/local.toml",
		Summary:  "Desktop default: reachable only from this machine.",
		Scenario: "dynamic request origin desktop local",
		Notes: []string{
			"The listener is loopback-only. A same-host reverse proxy may forward to it",
			"without changing the Desktop runtime manifest.",
		},
		build: func(inputs ProfileInputs) manifest {
			m := desktopBase()
			m.Server.Listen = ptr("127.0.0.1:6680")
			m.Server.Proxy.TrustedCIDRs = ptr([]string{"127.0.0.1/32", "::1/128"})
			return m
		},
	},
	{
		Name:     ProfileDesktopLANHTTP,
		Path:     "desktop/lan-http.toml",
		Summary:  "Desktop LAN sharing over plaintext HTTP.",
		Scenario: "dynamic request origin desktop LAN",
		Notes: []string{
			"The listener accepts LAN connections. HTTP remains available for password and",
			"TOTP, while passkeys become available automatically on a valid HTTPS hostname.",
		},
		build: func(inputs ProfileInputs) manifest {
			m := desktopBase()
			m.Server.Listen = ptr("0.0.0.0:6680")
			return m
		},
	},
	{
		Name:     ProfileDockerHTTP,
		Path:     "docker/http.toml",
		Summary:  "Docker default: host-network HTTP on port 6680.",
		Scenario: "zero-configuration Docker HTTP",
		Operator: true,
		Notes: []string{
			"This complete manifest is embedded in the image. No domain, public URL,",
			"certificate, or reverse proxy is required before first use.",
		},
		Defaults: ProfileInputs{Listen: ":6680"},
		build: func(inputs ProfileInputs) manifest {
			m := dockerBase(inputs)
			m.Server.Listen = ptr(inputs.Listen)
			m.Server.Proxy.TrustedCIDRs = ptr(append([]string(nil), inputs.TrustedProxyCIDRs...))
			return m
		},
	},
	{
		Name:     ProfileDockerCaddy,
		Path:     "docker/caddy.toml",
		Summary:  "Docker behind the bundled same-host Caddy proxy.",
		Scenario: "optional Caddy HTTPS",
		Notes: []string{
			"Lumilio listens only on loopback so public HTTP cannot bypass Caddy.",
			"Caddy supplies the request-facing host and scheme through standard headers.",
		},
		build: func(inputs ProfileInputs) manifest {
			m := dockerBase(inputs)
			m.Server.Listen = ptr("127.0.0.1:6680")
			m.Server.Proxy.TrustedCIDRs = ptr([]string{"127.0.0.1/32", "::1/128"})
			return m
		},
	},
	{
		Name:     ProfileDockerACME,
		Path:     "docker/acme.toml",
		Summary:  "Docker with built-in ACME HTTPS.",
		Scenario: "optional built-in ACME HTTPS",
		Operator: true,
		Notes: []string{
			"The server owns public TCP 80/443 and obtains a certificate for tls.hostname.",
			"Certificate acquisition failure remains fatal and never falls back to HTTP.",
			"",
			"Generate a real one with:",
			"",
			"    server config init --profile docker-acme \\",
			"      --hostname photos.example.com \\",
			"      --email admin@example.com \\",
			"      --output /data/app-state/server.toml",
		},
		Defaults: ProfileInputs{Hostname: "photos.example.com", Email: "admin@example.com"},
		build: func(inputs ProfileInputs) manifest {
			m := dockerBase(inputs)
			stateDir := strings.TrimRight(strings.TrimSpace(inputs.StateDir), "/")
			if stateDir == "" {
				stateDir = "/data/app-state"
			}
			m.Server.Listen = ptr(":443")
			m.Server.TLS.Mode = ptr(string(TLSModeACME))
			m.Server.TLS.Hostname = ptr(inputs.Hostname)
			m.Server.TLS.HTTPListen = ptr(":80")
			m.Server.TLS.Email = ptr(inputs.Email)
			m.Server.TLS.StoragePath = ptr(stateDir + "/tls")
			return m
		},
	},
}

// validateOperatorInputs rejects flag combinations that name a value the chosen
// profile has no place to put, which is friendlier than silently ignoring it.
func validateOperatorInputs(name ProfileName, inputs ProfileInputs) error {
	switch name {
	case ProfileDevVite:
		if strings.TrimSpace(inputs.Hostname) != "" || strings.TrimSpace(inputs.Email) != "" {
			return errors.New("dev-vite does not accept ACME inputs")
		}
		if strings.TrimSpace(inputs.Listen) != "" {
			return errors.New("dev-vite owns its loopback listener and does not accept --listen")
		}
	case ProfileDockerACME:
		if strings.TrimSpace(inputs.Hostname) == "" {
			return errors.New("docker-acme requires --hostname")
		}
		if strings.TrimSpace(inputs.Email) == "" {
			return errors.New("docker-acme requires --email")
		}
		if strings.TrimSpace(inputs.Listen) != "" {
			return errors.New("docker-acme owns fixed host listeners and does not accept --listen")
		}
		if len(inputs.TrustedProxyCIDRs) != 0 {
			return errors.New("docker-acme does not accept trusted proxies")
		}
	case ProfileDockerHTTP:
		if strings.TrimSpace(inputs.Hostname) != "" || strings.TrimSpace(inputs.Email) != "" {
			return errors.New("docker-http does not accept ACME inputs")
		}
	}
	return nil
}
