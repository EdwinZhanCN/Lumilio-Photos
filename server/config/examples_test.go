package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExamplesAreValidManifests(t *testing.T) {
	for _, profile := range Profiles() {
		t.Run(string(profile.Name), func(t *testing.T) {
			cfg, err := LoadAppConfig(examplePath(t, profile.Name))
			if err != nil {
				t.Fatalf("example %s failed the strict loader: %v", profile.Path, err)
			}
			if cfg.SchemaVersion != SchemaVersion {
				t.Fatalf("schema version = %d, want %d", cfg.SchemaVersion, SchemaVersion)
			}
		})
	}
}

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
		t.Fatal(err)
	}
	if string(onDisk) != string(schema) {
		t.Errorf("%s is stale; run `make config-examples`", SchemaFile)
	}
}

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

func TestProfileScenarioInvariants(t *testing.T) {
	tests := []struct {
		profile ProfileName
		check   func(*testing.T, AppConfig)
	}{
		{ProfileDevVite, func(t *testing.T, cfg AppConfig) {
			if !strings.HasPrefix(cfg.ServerConfig.Listen, "127.0.0.1:") ||
				len(cfg.ServerConfig.Proxy.TrustedCIDRs) != 2 ||
				cfg.ServerConfig.WebRoot != "" {
				t.Fatalf("dev profile = %+v", cfg.ServerConfig)
			}
		}},
		{ProfileDesktopLocal, func(t *testing.T, cfg AppConfig) {
			if !strings.HasPrefix(cfg.ServerConfig.Listen, "127.0.0.1:") {
				t.Fatalf("desktop local listener = %q", cfg.ServerConfig.Listen)
			}
		}},
		{ProfileDesktopLANHTTP, func(t *testing.T, cfg AppConfig) {
			if cfg.ServerConfig.Listen != "0.0.0.0:6680" {
				t.Fatalf("desktop LAN listener = %q", cfg.ServerConfig.Listen)
			}
		}},
		{ProfileDockerHTTP, func(t *testing.T, cfg AppConfig) {
			if cfg.ServerConfig.Listen != ":6680" || cfg.ServerConfig.TLS.Mode != TLSModeOff ||
				cfg.ServerConfig.WebRoot != "/app/web" {
				t.Fatalf("Docker HTTP profile = %+v", cfg.ServerConfig)
			}
		}},
		{ProfileDockerCaddy, func(t *testing.T, cfg AppConfig) {
			if cfg.ServerConfig.Listen != "127.0.0.1:6680" ||
				len(cfg.ServerConfig.Proxy.TrustedCIDRs) != 2 {
				t.Fatalf("Docker Caddy profile = %+v", cfg.ServerConfig)
			}
		}},
		{ProfileDockerACME, func(t *testing.T, cfg AppConfig) {
			tls := cfg.ServerConfig.TLS
			if tls.Mode != TLSModeACME || tls.Hostname != "photos.example.com" ||
				cfg.ServerConfig.Listen != ":443" || tls.HTTPListen != ":80" {
				t.Fatalf("Docker ACME profile = %+v", cfg.ServerConfig)
			}
		}},
	}
	if len(tests) != len(Profiles()) {
		t.Fatalf("%d checks for %d profiles", len(tests), len(Profiles()))
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

func examplePath(t *testing.T, name ProfileName) string {
	t.Helper()
	profile, ok := ProfileByName(name)
	if !ok {
		t.Fatalf("unknown profile %q", name)
	}
	return filepath.Join("examples", filepath.FromSlash(profile.Path))
}
