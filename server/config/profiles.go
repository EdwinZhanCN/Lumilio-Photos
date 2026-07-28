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
	ProfileDevVite                      ProfileName = "dev-vite"
	ProfileDesktopLocal                 ProfileName = "desktop-local"
	ProfileDesktopLANHTTP               ProfileName = "desktop-lan-http"
	ProfileDesktopExternalHTTPSSameHost ProfileName = "desktop-external-https-samehost"
	ProfileDesktopExternalHTTPSRemote   ProfileName = "desktop-external-https-remote"
	ProfileDockerACME                   ProfileName = "docker-acme"
	ProfileDockerExternalProxy          ProfileName = "docker-external-proxy"
	ProfileDockerDevHTTP                ProfileName = "docker-dev-http"
)

// ProfileInputs are the values an operator supplies for a profile that cannot
// be fully determined in advance. They are one-shot generation inputs and never
// runtime overrides: the generated manifest is complete on its own.
type ProfileInputs struct {
	Origin            string
	Email             string
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
	// Operator marks profiles `server config init` will generate. The rest are
	// documentation of shapes produced by Desktop or by the dev toolchain.
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
	if inputs.Origin == "" {
		inputs.Origin = p.Defaults.Origin
	}
	if inputs.Email == "" {
		inputs.Email = p.Defaults.Email
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
	if inputs.Origin != "" {
		normalized, _, err := NormalizeOrigin(inputs.Origin)
		if err != nil {
			return manifest{}, fmt.Errorf("origin must be an exact http(s) origin: %w", err)
		}
		inputs.Origin = normalized
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
		logs:       stateDir + "/logs",
		storage:    storageDir,
		cloudState: stateDir + "/cloud",
		backups:    stateDir + "/backups",
		secretKey:  stateDir + "/secrets/lumilio_secret_key",
		webRoot:    "/app/web",
	}
}

func desktopLayout() layout {
	const state = "/Users/example/Library/Application Support/Lumilio"
	return layout{
		database:   state + "/app-state/library.sqlite3",
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
		Database:      &databaseManifest{Path: ptr(l.database)},
		Server: &serverManifest{
			Listen:             ptr("127.0.0.1:6680"),
			PrimaryOrigin:      ptr("http://localhost:6680"),
			CORSAllowedOrigins: ptr([]string{}),
			WebRoot:            ptr(l.webRoot),
			TLS: &tlsManifest{
				Mode:        ptr(string(TLSModeOff)),
				HTTPListen:  ptr(""),
				Email:       ptr(""),
				StoragePath: ptr(""),
			},
			Proxy: &proxyManifest{
				Mode:         ptr(string(ProxyModeDisabled)),
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
			Enabled:            ptr(true),
			IntervalSeconds:    ptr(300),
			SettleSeconds:      ptr(5),
			MaxConcurrentRepos: ptr(1),
			BatchSize:          ptr(500),
		},
		Geocoding: &geocodingManifest{
			Provider:          ptr("disabled"),
			NominatimEndpoint: ptr("https://nominatim.openstreetmap.org/reverse"),
			Language:          ptr("en"),
			UserAgent:         ptr("Lumilio-Photos/1.0"),
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
			DiscoveryEnabled:      ptr(true),
			DiscoveryMDNSEnabled:  ptr(true),
			DiscoveryHubURL:       ptr(""),
			DiscoveryStaticNodes:  ptr([]string{}),
			DiscoveryServiceType:  ptr("_lumen._tcp"),
			DiscoveryDomain:       ptr("local"),
			DeploymentID:          ptr(deploymentID),
			ResolveTimeout:        ptr("3s"),
			ConnectTimeout:        ptr("3s"),
			RediscoveryBackoffMin: ptr("10s"),
			RediscoveryBackoffMax: ptr("2m"),
			ScanInterval:          ptr("30s"),
			ChunkAuto:             ptr(true),
			ChunkThresholdBytes:   ptr(1048576),
			ChunkMaxBytes:         ptr(262144),
		},
		Tools: &toolsManifest{
			ExifToolPath: ptr("exiftool"),
			FFmpegPath:   ptr("ffmpeg"),
			FFprobePath:  ptr("ffprobe"),
		},
	}
}

func desktopBase() manifest {
	return baseManifest("production", "desktop", "info", desktopLayout())
}

func dockerBase(inputs ProfileInputs) manifest {
	return baseManifest("production", "container", "info", containerLayout(inputs.StateDir, inputs.StorageDir))
}

// externalProxy applies the shape shared by every reverse-proxy deployment:
// the proxy owns HTTPS, this server speaks plain HTTP on an internal socket,
// and only the proxy's own address may present forwarded headers.
func externalProxy(m *manifest, origin string, listen string, cidrs []string) {
	m.Server.Listen = ptr(listen)
	m.Server.PrimaryOrigin = ptr(origin)
	m.Server.TLS.Mode = ptr(string(TLSModeExternal))
	m.Server.Proxy.Mode = ptr(string(ProxyModeRequired))
	m.Server.Proxy.TrustedCIDRs = ptr(append([]string(nil), cidrs...))
}

var profileTable = []Profile{
	{
		Name:     ProfileDevVite,
		Path:     "dev/vite.toml",
		Summary:  "Local development: single origin on the Vite dev server, LAN reachable.",
		Scenario: "deployment-origin-tls-plan.md 5.7 Development",
		Notes: []string{
			"`make dev` runs Vite on 0.0.0.0:6657 and this server on 127.0.0.1:6680. Vite",
			"proxies /api to the API, so the browser only ever talks to the Vite origin.",
			"Development is therefore single-origin like production, and",
			"cors_allowed_origins is empty: there is no cross-origin request to allow.",
			"",
			"Only Vite is published to the network. The API stays on loopback because the",
			"proxy runs on this machine, so a phone at http://<lan-ip>:6657 reaches the whole",
			"app without the API listener ever being exposed.",
			"",
			"The Vite proxy must not rewrite the Host header (changeOrigin stays false). The",
			"server derives its target origin from Host and compares the browser's Origin",
			"against it, so a rewritten Host would make every session request look",
			"cross-origin and be rejected.",
			"",
			"primary_origin is the Vite origin, not the API listener: WebAuthn signs the",
			"origin the browser is actually on, so the RP ID here is localhost. That means",
			"passkeys work on this machine at http://localhost:6657 and are unavailable from",
			"a LAN address, which is plain HTTP and not a secure context. Remote devices get",
			"password plus TOTP, exactly as in the Desktop LAN profile.",
		},
		Defaults: ProfileInputs{Origin: "http://localhost:6657"},
		build: func(inputs ProfileInputs) manifest {
			m := baseManifest("development", "local", "debug", layout{
				database:   "../.local/lumilio/library.sqlite3",
				logs:       "../logs",
				storage:    "../data/storage",
				cloudState: "../data/app-state/cloud",
				backups:    "../data/app-state/backups",
				secretKey:  "../data/app-state/secrets/lumilio_secret_key",
				webRoot:    "",
			})
			m.Geocoding.Language = ptr("zh")
			m.Server.PrimaryOrigin = ptr(inputs.Origin)
			return m
		},
	},
	{
		Name:     ProfileDesktopLocal,
		Path:     "desktop/local.toml",
		Summary:  "Desktop default: reachable only from this machine.",
		Scenario: "deployment-origin-tls-plan.md 5.1 Desktop local",
		Notes: []string{
			"The zero-configuration Desktop shape. The listener is bound to loopback, so no",
			"other device on the network can open a connection at all.",
			"",
			"Passkey works because browsers treat http://localhost as a secure context. This",
			"is the only way plain HTTP and WebAuthn coexist legitimately.",
			"",
			"Desktop generates this file itself from its network settings; the paths below",
			"are illustrative of a macOS install. Desktop never uses tls.mode = acme.",
		},
		Defaults: ProfileInputs{Origin: "http://localhost:6680"},
		build: func(inputs ProfileInputs) manifest {
			m := desktopBase()
			m.Server.Listen = ptr("127.0.0.1:6680")
			m.Server.PrimaryOrigin = ptr(inputs.Origin)
			return m
		},
	},
	{
		Name:     ProfileDesktopLANHTTP,
		Path:     "desktop/lan-http.toml",
		Summary:  "Desktop LAN sharing over plaintext HTTP.",
		Scenario: "deployment-origin-tls-plan.md 5.2 Desktop LAN HTTP",
		Notes: []string{
			"Turning LAN sharing on changes exactly one key: listen moves from loopback to",
			"0.0.0.0. primary_origin deliberately stays on localhost so the derived RP ID",
			"does not change and passkeys already registered on this machine keep working.",
			"",
			"LAN traffic is unencrypted. Passwords, TOTP codes, session cookies and media all",
			"cross the network in the clear; a home network is not a trusted transport.",
			"",
			"Passkey is therefore available on this machine through http://localhost only.",
			"A remote device at http://<lan-ip>:6680 is not on primary_origin, so the server",
			"reports passkey unavailable and the browser is offered password plus TOTP with a",
			"persistent unencrypted-connection warning.",
			"",
			"0.0.0.0 covers IPv4 interfaces only; it does not imply IPv6.",
		},
		Defaults: ProfileInputs{Origin: "http://localhost:6680"},
		build: func(inputs ProfileInputs) manifest {
			m := desktopBase()
			m.Server.Listen = ptr("0.0.0.0:6680")
			m.Server.PrimaryOrigin = ptr(inputs.Origin)
			return m
		},
	},
	{
		Name:     ProfileDesktopExternalHTTPSSameHost,
		Path:     "desktop/external-https-samehost.toml",
		Summary:  "Desktop behind a reverse proxy running on the same machine.",
		Scenario: "deployment-origin-tls-plan.md 5.3 Desktop external HTTPS (same host)",
		Notes: []string{
			"A proxy such as Caddy terminates HTTPS on this machine and forwards to the",
			"loopback listener:",
			"",
			"    photos.example.com {",
			"        reverse_proxy 127.0.0.1:6680",
			"    }",
			"",
			"Because the proxy is local, trusted_cidrs is loopback only: nothing off this",
			"machine can present forwarded headers, and the listener is not reachable from",
			"the network in the first place.",
			"",
			"Every device now uses one canonical HTTPS address, so passkeys work everywhere.",
			"The RP ID becomes photos.example.com; passkeys previously registered against",
			"localhost survive but cannot be used here and must be re-registered after a",
			"password plus TOTP login.",
		},
		Defaults: ProfileInputs{
			Origin:            "https://photos.example.com",
			TrustedProxyCIDRs: []string{"127.0.0.1/32", "::1/128"},
		},
		build: func(inputs ProfileInputs) manifest {
			m := desktopBase()
			externalProxy(&m, inputs.Origin, "127.0.0.1:6680", inputs.TrustedProxyCIDRs)
			return m
		},
	},
	{
		Name:     ProfileDesktopExternalHTTPSRemote,
		Path:     "desktop/external-https-remote.toml",
		Summary:  "Desktop behind a reverse proxy running on another host.",
		Scenario: "deployment-origin-tls-plan.md 5.4 Desktop external HTTPS (remote proxy)",
		Notes: []string{
			"The proxy lives on a different machine, so the listener has to be reachable on a",
			"LAN interface rather than loopback. Bind the specific interface address, not",
			"0.0.0.0, so the exposure is deliberate and visible.",
			"",
			"trusted_cidrs pins the proxy's exact address with a host mask. Any other LAN",
			"client that reaches the listener directly is rejected with 403 before any auth",
			"handler runs, because proxy.mode = required trusts the immediate TCP peer only.",
			"",
			"Loopback health endpoints keep a narrow exception so readiness probes work",
			"without going through the proxy.",
		},
		Defaults: ProfileInputs{
			Origin:            "https://photos.example.com",
			TrustedProxyCIDRs: []string{"192.168.1.10/32"},
		},
		build: func(inputs ProfileInputs) manifest {
			m := desktopBase()
			externalProxy(&m, inputs.Origin, "192.168.1.20:6680", inputs.TrustedProxyCIDRs)
			return m
		},
	},
	{
		Name:     ProfileDockerACME,
		Path:     "docker/acme.toml",
		Summary:  "Docker with built-in ACME: this server obtains and renews its own certificate.",
		Scenario: "deployment-origin-tls-plan.md 5.5 Docker built-in ACME HTTPS",
		Operator: true,
		Notes: []string{
			"There is no reverse proxy here, so proxy.mode stays disabled and the server",
			"owns TLS end to end. The certificate hostname is derived from primary_origin;",
			"there is no separate domain key that could drift away from it.",
			"",
			"Two listeners are required. Publish them from unprivileged container ports:",
			"",
			"    ports:",
			"      - \"80:8080\"     # http_listen: ACME HTTP-01 challenge, then 308 to HTTPS",
			"      - \"443:8443\"    # listen: the application over HTTPS",
			"",
			"Preconditions: you control the domain, its A/AAAA records point at this",
			"deployment, and a public CA can reach port 80 or 443.",
			"",
			"storage_path must be a persistent volume. Losing it re-requests certificates on",
			"every restart and will hit CA rate limits. Certificate acquisition failure is",
			"fatal by design: the server refuses to start rather than fall back to plaintext.",
			"",
			"Generate a real one with:",
			"",
			"    server config init --profile docker-acme \\",
			"      --origin https://photos.example.com \\",
			"      --email admin@example.com \\",
			"      --output /data/app-state/server.toml",
		},
		Defaults: ProfileInputs{
			Origin: "https://photos.example.com",
			Email:  "admin@example.com",
		},
		build: func(inputs ProfileInputs) manifest {
			m := dockerBase(inputs)
			stateDir := strings.TrimRight(strings.TrimSpace(inputs.StateDir), "/")
			if stateDir == "" {
				stateDir = "/data/app-state"
			}
			m.Server.Listen = ptr("0.0.0.0:8443")
			m.Server.PrimaryOrigin = ptr(inputs.Origin)
			m.Server.TLS.Mode = ptr(string(TLSModeACME))
			m.Server.TLS.HTTPListen = ptr("0.0.0.0:8080")
			m.Server.TLS.Email = ptr(inputs.Email)
			m.Server.TLS.StoragePath = ptr(stateDir + "/tls")
			return m
		},
	},
	{
		Name:     ProfileDockerExternalProxy,
		Path:     "docker/external-proxy.toml",
		Summary:  "Docker behind an external reverse proxy that owns HTTPS.",
		Scenario: "deployment-origin-tls-plan.md 5.6 Docker external reverse proxy",
		Operator: true,
		Notes: []string{
			"The proxy terminates HTTPS and this container speaks plain HTTP on an internal",
			"network. Do not publish this listener to the host:",
			"",
			"    services:",
			"      lumilio:",
			"        expose:",
			"          - \"6680\"",
			"        networks:",
			"          - lumilio_proxy",
			"",
			"Network isolation is the first layer of protection and trusted_cidrs is the",
			"second. Make the CIDR match that dedicated proxy network exactly: a broad subnet",
			"shared with unrelated containers would let any of them forge the forwarded",
			"origin and defeat the check.",
			"",
			"The proxy must overwrite, not pass through, the client's proto and host headers.",
			"Conflicting or ambiguous Forwarded and X-Forwarded-* values are rejected.",
			"",
			"Generate a real one with:",
			"",
			"    server config init --profile docker-external-proxy \\",
			"      --origin https://photos.example.com \\",
			"      --trusted-proxy 172.30.0.0/24 \\",
			"      --output /data/app-state/server.toml",
		},
		Defaults: ProfileInputs{
			Origin:            "https://photos.example.com",
			TrustedProxyCIDRs: []string{"172.30.0.0/24"},
		},
		build: func(inputs ProfileInputs) manifest {
			m := dockerBase(inputs)
			externalProxy(&m, inputs.Origin, "0.0.0.0:6680", inputs.TrustedProxyCIDRs)
			return m
		},
	},
	{
		Name:     ProfileDockerDevHTTP,
		Path:     "docker/dev-http.toml",
		Summary:  "Container over plaintext HTTP, for development and tests only.",
		Scenario: "deployment-origin-tls-plan.md 12.2 docker-compose.dev.yml",
		Notes: []string{
			"Used by docker-compose.dev.yml. The image serves the SPA and the API on one",
			"origin, so cors_allowed_origins is empty.",
			"",
			"This is not a production shape and must not be treated as a default. Production",
			"operators pick docker/acme.toml or docker/external-proxy.toml, which is why the",
			"release image ships no bootable HTTP manifest.",
			"",
			"primary_origin is http://localhost:6680, so this only behaves correctly when the",
			"browser reaches it at exactly that address on the container host.",
		},
		Defaults: ProfileInputs{Origin: "http://localhost:6680"},
		build: func(inputs ProfileInputs) manifest {
			m := dockerBase(inputs)
			m.Server.Listen = ptr("0.0.0.0:6680")
			m.Server.PrimaryOrigin = ptr(inputs.Origin)
			return m
		},
	},
}

// validateOperatorInputs rejects flag combinations that name a value the chosen
// profile has no place to put, which is friendlier than silently ignoring it.
func validateOperatorInputs(name ProfileName, inputs ProfileInputs) error {
	switch name {
	case ProfileDockerACME:
		if strings.TrimSpace(inputs.Email) == "" {
			return errors.New("docker-acme requires --email")
		}
		if len(inputs.TrustedProxyCIDRs) != 0 {
			return errors.New("docker-acme does not accept trusted proxies")
		}
	case ProfileDockerExternalProxy:
		if strings.TrimSpace(inputs.Email) != "" {
			return errors.New("docker-external-proxy does not accept --email")
		}
		if len(inputs.TrustedProxyCIDRs) == 0 {
			return errors.New("docker-external-proxy requires at least one --trusted-proxy")
		}
	}
	return nil
}
