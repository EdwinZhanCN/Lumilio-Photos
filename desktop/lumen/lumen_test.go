package lumen

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestProfileFor(t *testing.T) {
	cases := []struct {
		goos, goarch, want string
	}{
		{"darwin", "arm64", "darwin-arm64-metal"},
		{"windows", "amd64", "windows-x64-gpu"},
	}
	for _, c := range cases {
		got, err := profileFor(c.goos, c.goarch)
		if err != nil || got != c.want {
			t.Errorf("profileFor(%s/%s) = %q, %v; want %q", c.goos, c.goarch, got, err, c.want)
		}
	}
	for _, unsupported := range [][2]string{{"darwin", "amd64"}, {"linux", "amd64"}, {"windows", "arm64"}} {
		if _, err := profileFor(unsupported[0], unsupported[1]); err == nil {
			t.Errorf("profileFor(%s/%s) should fail", unsupported[0], unsupported[1])
		}
	}
}

func TestStripFirstComponent(t *testing.T) {
	cases := map[string]string{
		"lumen-hub-darwin-arm64-metal/bin/lumen-hub": "bin/lumen-hub",
		"lumen-hub-x/warmup/sample.jpg":              "warmup/sample.jpg",
		"lumen-hub-x/":                               "",
		"toplevel-file":                              "",
	}
	for in, want := range cases {
		if got := stripFirstComponent(in); got != want {
			t.Errorf("stripFirstComponent(%q) = %q, want %q", in, got, want)
		}
	}
}

// buildHubZip assembles an in-memory release zip in the real dist layout
// (one wrapping profile dir, bin/ + warmup/).
func buildHubZip(t *testing.T, profile string) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	for name, content := range map[string]string{
		"lumen-hub-" + profile + "/bin/lumen-hub":     "#!/bin/sh\necho hub\n",
		"lumen-hub-" + profile + "/bin/lumen-hub.exe": "MZ fake",
		"lumen-hub-" + profile + "/warmup/sample.txt": "warmup",
		"lumen-hub-" + profile + "/README.txt":        "readme",
	} {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := f.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestInstallFromManifest(t *testing.T) {
	profile, err := ProfileForHost()
	if err != nil {
		t.Skipf("unsupported host: %v", err)
	}

	zipData := buildHubZip(t, profile)
	sum := sha256.Sum256(zipData)

	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	defer server.Close()
	mux.HandleFunc("/hub.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(zipData)
	})
	mux.HandleFunc("/manifest.json", func(w http.ResponseWriter, r *http.Request) {
		manifest := Manifest{
			Version: "v9.9.9-test",
			Hub: []Artifact{{
				Profile:  profile,
				FileName: "lumen-hub-" + profile + ".zip",
				URL:      server.URL + "/hub.zip",
				SHA256:   hex.EncodeToString(sum[:]),
			}},
		}
		_ = json.NewEncoder(w).Encode(manifest)
	})

	dir := t.TempDir()
	state, err := Install(context.Background(), dir, server.URL+"/manifest.json", t.Logf)
	if err != nil {
		t.Fatalf("Install: %v", err)
	}
	if state.Version != "v9.9.9-test" || state.Profile != profile {
		t.Errorf("state = %+v", state)
	}

	got, ok := Installed(dir)
	if !ok || got != state {
		t.Errorf("Installed = %+v, %v; want %+v", got, ok, state)
	}

	// The wrapper dir must be stripped and bin/ made executable.
	bin := HubBinary(dir)
	info, err := os.Stat(bin)
	if err != nil {
		t.Fatalf("hub binary missing: %v", err)
	}
	if runtime.GOOS != "windows" && info.Mode()&0o111 == 0 {
		t.Errorf("hub binary is not executable: %v", info.Mode())
	}
	if _, err := os.Stat(filepath.Join(dir, "hub", "warmup", "sample.txt")); err != nil {
		t.Errorf("warmup assets missing: %v", err)
	}

	// A corrupted download must fail and leave the previous install intact.
	mux.HandleFunc("/bad.zip", func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("tampered"))
	})
	mux.HandleFunc("/manifest-bad.json", func(w http.ResponseWriter, r *http.Request) {
		manifest := Manifest{
			Version: "v9.9.10-test",
			Hub: []Artifact{{
				Profile:  profile,
				FileName: "lumen-hub-" + profile + ".zip",
				URL:      server.URL + "/bad.zip",
				SHA256:   hex.EncodeToString(sum[:]),
			}},
		}
		_ = json.NewEncoder(w).Encode(manifest)
	})
	if _, err := Install(context.Background(), dir, server.URL+"/manifest-bad.json", t.Logf); err == nil {
		t.Fatal("Install with checksum mismatch should fail")
	} else if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Fatalf("want checksum error, got: %v", err)
	}
	if _, ok := Installed(dir); !ok {
		t.Error("failed install must not clobber the previous one")
	}
}

func TestExtractZipRejectsEscape(t *testing.T) {
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	f, err := w.Create("wrapper/../../evil")
	if err != nil {
		t.Fatal(err)
	}
	_, _ = f.Write([]byte("x"))
	_ = w.Close()

	dir := t.TempDir()
	zipPath := filepath.Join(dir, "evil.zip")
	if err := os.WriteFile(zipPath, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := extractZip(zipPath, filepath.Join(dir, "out")); err == nil {
		t.Fatal("zip-slip entry should be rejected")
	}
}

func TestRestorePreviousInstall(t *testing.T) {
	dir := t.TempDir()
	current := hubDir(dir)
	previous := current + ".previous"
	if err := os.MkdirAll(filepath.Join(current, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(previous, "bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(current, "bin", "new"), []byte("new"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(previous, "bin", "old"), []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldState := `{"version":"old","profile":"darwin-arm64-cpu"}`
	if err := os.WriteFile(previousStateFile(dir), []byte(oldState), 0o600); err != nil {
		t.Fatal(err)
	}
	if !RestorePrevious(dir) {
		t.Fatal("RestorePrevious returned false")
	}
	if _, err := os.Stat(filepath.Join(current, "bin", "old")); err != nil {
		t.Fatalf("old install not restored: %v", err)
	}
	state, err := os.ReadFile(stateFile(dir))
	if err != nil || string(state) != oldState {
		t.Fatalf("state=%q err=%v", state, err)
	}
}

func TestWriteConfig(t *testing.T) {
	dir := t.TempDir()
	if err := WriteConfig(dir, "zh"); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(ConfigPath(dir))
	if err != nil {
		t.Fatal(err)
	}
	config := string(data)
	for _, want := range []string{
		"region: cn",
		fmt.Sprintf("cache_dir: '%s'", filepath.Join(dir, "models")),
		`host: "127.0.0.1"`,
		"port: 50051",
		"- siglip", "- face", "- ocr",
	} {
		if !strings.Contains(config, want) {
			t.Errorf("config missing %q:\n%s", want, config)
		}
	}

	if err := WriteConfig(dir, "en"); err != nil {
		t.Fatal(err)
	}
	data, _ = os.ReadFile(ConfigPath(dir))
	if !strings.Contains(string(data), "region: other") {
		t.Error("non-zh language should select region other")
	}
}
