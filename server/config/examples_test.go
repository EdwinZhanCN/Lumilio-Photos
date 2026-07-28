package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// TestExamplesAreValidManifests is the property that makes the examples worth
// checking in: each one is a real, loadable manifest, not prose that drifted
// away from what the server accepts.
func TestExamplesAreValidManifests(t *testing.T) {
	for _, profile := range Profiles() {
		t.Run(string(profile.Name), func(t *testing.T) {
			path := filepath.Join("examples", filepath.FromSlash(profile.Path))
			cfg, err := LoadAppConfig(path)
			if err != nil {
				t.Fatalf("example %s failed the strict loader: %v", profile.Path, err)
			}
			if cfg.SchemaVersion != SchemaVersion {
				t.Fatalf("schema version = %d, want %d", cfg.SchemaVersion, SchemaVersion)
			}
		})
	}
}

// TestGeneratedFilesAreCurrent locks the checked-in artifacts to the profile
// table. Editing an example by hand, or changing a doc comment or enum without
// regenerating, fails here — which is the actual anti-drift mechanism. The
// comments themselves prevent nothing.
func TestGeneratedFilesAreCurrent(t *testing.T) {
	examples, err := RenderExamples()
	if err != nil {
		t.Fatal(err)
	}
	for name, want := range examples {
		path := filepath.Join("examples", filepath.FromSlash(name))
		got, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(got) != string(want) {
			t.Errorf("%s is stale; run `make config-examples`", path)
		}
	}

	schema, err := GenerateJSONSchema()
	if err != nil {
		t.Fatal(err)
	}
	onDisk, err := os.ReadFile(SchemaFile)
	if err != nil {
		t.Fatalf("read %s: %v", SchemaFile, err)
	}
	if string(onDisk) != string(schema) {
		t.Errorf("%s is stale; run `make config-examples`", SchemaFile)
	}
}

// TestExampleFilesAreAllAccountedFor catches an example left behind after a
// profile is renamed or removed.
func TestExampleFilesAreAllAccountedFor(t *testing.T) {
	expected := make(map[string]bool)
	for _, profile := range Profiles() {
		expected[filepath.FromSlash(profile.Path)] = true
	}

	err := filepath.Walk("examples", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relative, err := filepath.Rel("examples", path)
		if err != nil {
			return err
		}
		if !expected[relative] {
			t.Errorf("examples/%s has no profile; delete it or add the profile", relative)
		}
		delete(expected, relative)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for missing := range expected {
		t.Errorf("profile expects examples/%s but the file is absent", missing)
	}
}

// TestProfileScenarioInvariants asserts the cross-field shape of each scenario.
// These are exactly the rules a per-key TOML comment or a JSON Schema keyword
// cannot express, so they are asserted against the resolved config instead.
func TestProfileScenarioInvariants(t *testing.T) {
	tests := []struct {
		profile ProfileName
		check   func(*testing.T, AppConfig)
	}{
		{
			profile: ProfileDevVite,
			check: func(t *testing.T, cfg AppConfig) {
				// The browser origin is Vite's, not the API listener's, and it
				// is the RP ID source. Regressing this silently breaks passkeys
				// in development.
				if cfg.ServerConfig.PrimaryOrigin != "http://localhost:6657" {
					t.Errorf("primary origin = %q", cfg.ServerConfig.PrimaryOrigin)
				}
				if cfg.Auth.PasskeyIdentity.RPID != "localhost" {
					t.Errorf("RP ID = %q", cfg.Auth.PasskeyIdentity.RPID)
				}
				// Vite proxies /api, so the browser never makes a cross-origin
				// request. A non-empty list here would mean the proxy was
				// bypassed and the SPA had gone back to calling the API directly.
				if len(cfg.ServerConfig.CORSAllowedOrigins) != 0 {
					t.Errorf("dev is single-origin behind the Vite proxy; CORS must be empty, got %v",
						cfg.ServerConfig.CORSAllowedOrigins)
				}
				// The proxy runs on this machine, so the API never needs to be
				// exposed even though the dev server itself is LAN-reachable.
				if !strings.HasPrefix(cfg.ServerConfig.Listen, "127.0.0.1:") {
					t.Errorf("dev API listener must stay on loopback, got %q", cfg.ServerConfig.Listen)
				}
				if cfg.ServerConfig.WebRoot != "" {
					t.Error("Vite serves the SPA in development; web_root must be empty")
				}
			},
		},
		{
			profile: ProfileDesktopLocal,
			check: func(t *testing.T, cfg AppConfig) {
				if !strings.HasPrefix(cfg.ServerConfig.Listen, "127.0.0.1:") {
					t.Errorf("desktop local must bind loopback, got %q", cfg.ServerConfig.Listen)
				}
				if !cfg.Auth.Passkey.Enabled {
					t.Error("http://localhost is a secure context; passkey should be on")
				}
			},
		},
		{
			profile: ProfileDesktopLANHTTP,
			check: func(t *testing.T, cfg AppConfig) {
				if cfg.ServerConfig.Listen != "0.0.0.0:6680" {
					t.Errorf("LAN mode must widen the listener, got %q", cfg.ServerConfig.Listen)
				}
				// The whole point of the LAN profile: only `listen` moves, so
				// the RP ID is unchanged and existing passkeys keep working.
				local, err := LoadAppConfig(examplePath(t, ProfileDesktopLocal))
				if err != nil {
					t.Fatal(err)
				}
				if cfg.ServerConfig.PrimaryOrigin != local.ServerConfig.PrimaryOrigin {
					t.Errorf(
						"LAN mode changed primary origin to %q; it must stay %q so the RP ID is stable",
						cfg.ServerConfig.PrimaryOrigin, local.ServerConfig.PrimaryOrigin,
					)
				}
				if cfg.Auth.PasskeyIdentity.RPID != local.Auth.PasskeyIdentity.RPID {
					t.Error("LAN mode must not change the RP ID")
				}
			},
		},
		{
			profile: ProfileDesktopExternalHTTPSSameHost,
			check: func(t *testing.T, cfg AppConfig) {
				assertExternalProxy(t, cfg)
				if !strings.HasPrefix(cfg.ServerConfig.Listen, "127.0.0.1:") {
					t.Errorf("a same-host proxy should reach loopback, got %q", cfg.ServerConfig.Listen)
				}
				for _, cidr := range cfg.ServerConfig.Proxy.TrustedCIDRs {
					if !cidr.Addr().IsLoopback() {
						t.Errorf("same-host profile trusts non-loopback %s", cidr)
					}
				}
			},
		},
		{
			profile: ProfileDesktopExternalHTTPSRemote,
			check: func(t *testing.T, cfg AppConfig) {
				assertExternalProxy(t, cfg)
				if strings.HasPrefix(cfg.ServerConfig.Listen, "127.0.0.1:") {
					t.Error("a remote proxy cannot reach a loopback listener")
				}
				// A host mask is what keeps every other LAN client out.
				for _, cidr := range cfg.ServerConfig.Proxy.TrustedCIDRs {
					if cidr.Bits() != cidr.Addr().BitLen() {
						t.Errorf("remote proxy trust should pin one host, got %s", cidr)
					}
				}
			},
		},
		{
			profile: ProfileDockerACME,
			check: func(t *testing.T, cfg AppConfig) {
				server := cfg.ServerConfig
				if server.TLS.Mode != TLSModeACME {
					t.Fatalf("tls mode = %q", server.TLS.Mode)
				}
				if server.TLS.HTTPListen == "" {
					t.Error("ACME needs an HTTP listener for challenges and redirects")
				}
				if server.TLS.Email == "" || server.TLS.StoragePath == "" {
					t.Error("ACME needs an account email and persistent storage")
				}
				if server.Proxy.Mode != ProxyModeDisabled {
					t.Error("ACME terminates TLS itself; no proxy is involved")
				}
				if server.Listen == server.TLS.HTTPListen {
					t.Error("the two ACME listeners must differ")
				}
			},
		},
		{
			profile: ProfileDockerExternalProxy,
			check: func(t *testing.T, cfg AppConfig) {
				assertExternalProxy(t, cfg)
				if cfg.ServerConfig.WebRoot == "" {
					t.Error("the container image serves the SPA itself")
				}
				if len(cfg.ServerConfig.CORSAllowedOrigins) != 0 {
					t.Error("SPA and API share one origin here; CORS must be empty")
				}
			},
		},
		{
			profile: ProfileDockerDevHTTP,
			check: func(t *testing.T, cfg AppConfig) {
				if cfg.ServerConfig.TLS.Mode != TLSModeOff {
					t.Errorf("tls mode = %q", cfg.ServerConfig.TLS.Mode)
				}
				if cfg.ServerConfig.Proxy.Mode != ProxyModeDisabled {
					t.Error("plaintext HTTP cannot require a proxy")
				}
			},
		},
	}

	if len(tests) != len(Profiles()) {
		t.Fatalf("every profile needs scenario assertions: %d checks for %d profiles",
			len(tests), len(Profiles()))
	}

	for _, test := range tests {
		t.Run(string(test.profile), func(t *testing.T) {
			cfg, err := LoadAppConfig(examplePath(t, test.profile))
			if err != nil {
				t.Fatal(err)
			}
			test.check(t, cfg)
		})
	}
}

// assertExternalProxy holds the invariant shared by every reverse-proxy
// scenario: the proxy owns HTTPS and only it may speak for the client.
func assertExternalProxy(t *testing.T, cfg AppConfig) {
	t.Helper()
	server := cfg.ServerConfig
	if server.TLS.Mode != TLSModeExternal {
		t.Fatalf("tls mode = %q, want external", server.TLS.Mode)
	}
	if server.Proxy.Mode != ProxyModeRequired {
		t.Fatal("an external proxy deployment must require the proxy, or direct clients bypass it")
	}
	if len(server.Proxy.TrustedCIDRs) == 0 {
		t.Fatal("proxy required with no trusted CIDR would reject everything")
	}
	if !strings.HasPrefix(server.PrimaryOrigin, "https://") {
		t.Errorf("primary origin = %q, want https", server.PrimaryOrigin)
	}
	if server.TLS.HTTPListen != "" || server.TLS.Email != "" || server.TLS.StoragePath != "" {
		t.Error("ACME fields must stay empty when the proxy owns TLS")
	}
}

func examplePath(t *testing.T, name ProfileName) string {
	t.Helper()
	profile, ok := ProfileByName(name)
	if !ok {
		t.Fatalf("unknown profile %q", name)
	}
	return filepath.Join("examples", filepath.FromSlash(profile.Path))
}

// TestEnumTagsMatchValidation is the contract between the two consumers of a
// closed value set. The `jsonschema:"enum=…"` tags feed the editor schema and
// the generated comments; the enum vars feed requireOneOf at runtime. If they
// disagree, the schema either rejects a config the server accepts or advertises
// one it does not.
func TestEnumTagsMatchValidation(t *testing.T) {
	expected := map[string][]string{
		"manifest.Environment":            environmentValues,
		"tlsManifest.Mode":                tlsModeValues,
		"proxyManifest.Mode":              proxyModeValues,
		"loggingManifest.Level":           logLevelValues,
		"loggingManifest.ConsoleFormat":   logFormatValues,
		"loggingManifest.FileFormat":      logFormatValues,
		"geocodingManifest.Provider":      geocodingProviders,
		"transcodeManifest.HardwareAccel": hardwareAccelValues,
	}

	found := make(map[string][]string)
	for _, structType := range manifestStructTypes() {
		for i := 0; i < structType.NumField(); i++ {
			field := structType.Field(i)
			values := enumTagValues(field.Tag.Get("jsonschema"))
			if len(values) == 0 {
				continue
			}
			found[structType.Name()+"."+field.Name] = values
		}
	}

	for key, want := range expected {
		got, ok := found[key]
		if !ok {
			t.Errorf("%s lost its jsonschema enum tag", key)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s tag enum %v != validation list %v", key, got, want)
		}
		delete(found, key)
	}
	for key, values := range found {
		t.Errorf("%s carries enum tag %v with no matching validation list", key, values)
	}
}

func enumTagValues(tag string) []string {
	var values []string
	for _, part := range strings.Split(tag, ",") {
		if after, ok := strings.CutPrefix(strings.TrimSpace(part), "enum="); ok {
			values = append(values, after)
		}
	}
	return values
}

// manifestStructTypes walks the manifest tree so a new section cannot escape
// the enum contract check by being unreachable from the root.
func manifestStructTypes() []reflect.Type {
	var types []reflect.Type
	seen := make(map[reflect.Type]bool)

	var visit func(reflect.Type)
	visit = func(t reflect.Type) {
		for t.Kind() == reflect.Ptr {
			t = t.Elem()
		}
		if t.Kind() != reflect.Struct || seen[t] {
			return
		}
		seen[t] = true
		types = append(types, t)
		for i := 0; i < t.NumField(); i++ {
			visit(t.Field(i).Type)
		}
	}
	visit(reflect.TypeOf(manifest{}))
	return types
}
