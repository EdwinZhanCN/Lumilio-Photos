package lumen

import (
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestLauncherPresetContract(t *testing.T) {
	cases := []struct {
		name       string
		wants, not []string
	}{
		{PresetMinimal, []string{"model: siglip2-base-patch16-224", "model: antelopev2"}, []string{"pp-ocrv6-small", "bioclip-2"}},
		{PresetBasic, []string{"siglip2-base-patch16-224", "pp-ocrv6-small", "dataset: TreeOfLife200MCore"}, nil},
		{PresetBrave, []string{"siglip2-so400m-patch14-384", "dataset: TreeOfLife200M"}, nil},
	}
	choices, err := BackendChoicesForHost()
	if err != nil {
		t.Skip(err)
	}
	for _, tc := range cases {
		p, _ := PresetByName(tc.name)
		got := renderConfig(p, ConfigSelection{Preset: tc.name, Backend: choices[0].Name, Profile: choices[0].Profile, CacheDir: filepath.Join(t.TempDir(), "models"), Region: "other"})
		for _, want := range tc.wants {
			if !strings.Contains(got, want) {
				t.Errorf("%s missing %q", tc.name, want)
			}
		}
		for _, no := range tc.not {
			if strings.Contains(got, no) {
				t.Errorf("%s unexpectedly contains %q", tc.name, no)
			}
		}
		if !strings.Contains(got, `host: "127.0.0.1"`) {
			t.Errorf("%s must bind loopback", tc.name)
		}
	}
}

func TestRecommendPreset(t *testing.T) {
	for _, tc := range []struct {
		ram, disk float64
		want      string
	}{{3, 20, PresetMinimal}, {6, 6, PresetBasic}, {16, 10, PresetBrave}, {0, 0, PresetBasic}} {
		if got := RecommendPreset(tc.ram, tc.disk); got != tc.want {
			t.Errorf("RecommendPreset(%v,%v)=%s want %s", tc.ram, tc.disk, got, tc.want)
		}
	}
}

func TestBackendChoicesMatchLauncher(t *testing.T) {
	mac, _ := BackendChoicesFor("darwin", "arm64")
	if mac[0].Profile != "darwin-arm64-metal" || mac[1].Profile != "darwin-arm64-cpu" {
		t.Fatalf("mac choices: %+v", mac)
	}
	win, _ := BackendChoicesFor("windows", "amd64")
	if win[0].Profile != "windows-x64-gpu" || win[1].Profile != "windows-x64-cpu" {
		t.Fatalf("windows choices: %+v", win)
	}
	if _, err := BackendChoicesFor("linux", runtime.GOARCH); err == nil {
		t.Fatal("Desktop should reject unsupported Linux host")
	}
}
