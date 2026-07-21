package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"desktop/supervisor"
)

func TestDashboardLogEndpoint(t *testing.T) {
	d := newTestApp(t)
	logDir := d.sup.LogDir()
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(logDir, "app.log"), []byte("first\nsecond\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	d.onboardingHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/__onb/log?source=app", nil))
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "second") {
		t.Fatalf("log response = %d %s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	d.onboardingHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/__onb/log?source=../../secrets", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("arbitrary log source status = %d, want 400", rec.Code)
	}

	// security.log may contain a one-time BreakGlass password and must never be
	// exposed through the desktop log panel.
	rec = httptest.NewRecorder()
	d.onboardingHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/__onb/log?source=security", nil))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("security log source status = %d, want 400", rec.Code)
	}
}

func newTestApp(t *testing.T) *desktopApp {
	t.Helper()
	t.Setenv("LUMILIO_APP_DATA", t.TempDir())
	d := &desktopApp{onboardCh: make(chan struct{}), lang: "en"}
	d.sup = supervisor.New(supervisor.Options{Logf: func(string, ...any) {}})
	return d
}

func TestCancelledRepositoryDirectoryPickersDoNotAccessControlPlane(t *testing.T) {
	d := &desktopApp{
		nativeDirectoryPicker: func(string, bool) (string, bool) { return "", true },
	}
	for _, handler := range []struct {
		name string
		fn   http.HandlerFunc
	}{
		{name: "storage location", fn: d.handlePickStorageLocation},
		{name: "attach repository", fn: d.handleAttachRepository},
	} {
		t.Run(handler.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			handler.fn(rec, httptest.NewRequest(http.MethodPost, "/", nil))
			if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"cancelled":true`) {
				t.Fatalf("cancel response = %d %s", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestNormalizeLang(t *testing.T) {
	cases := map[string]string{
		"zh": "zh", "zh-CN": "zh", "zh_CN.UTF-8": "zh", "ZH": "zh",
		"en": "en", "en-US": "en", "fr": "en", "": "en",
	}
	for in, want := range cases {
		if got := normalizeLang(in); got != want {
			t.Errorf("normalizeLang(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestHumanBytes(t *testing.T) {
	cases := map[uint64]string{
		512:                    "512 B",
		2 * 1024:               "2.0 KB",
		5 * 1024 * 1024:        "5.0 MB",
		3 * 1024 * 1024 * 1024: "3.0 GB",
	}
	for in, want := range cases {
		if got := humanBytes(in); got != want {
			t.Errorf("humanBytes(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateStorage(t *testing.T) {
	dir := t.TempDir()

	// A creatable path under an existing parent is reachable + writable, with a
	// free-space reading.
	v := validateStorage(filepath.Join(dir, "library"))
	if !v.Reachable || !v.Writable {
		t.Fatalf("creatable path: got %+v, want reachable+writable", v)
	}
	if v.FreeBytes == 0 || v.FreeHuman == "" {
		t.Errorf("expected a free-space reading, got %+v", v)
	}

	// A path under a non-existent parent (unmounted drive) is unreachable.
	if v := validateStorage(filepath.Join(dir, "missing", "library")); v.Reachable {
		t.Errorf("path under missing parent should be unreachable, got %+v", v)
	}

	// Empty path is not reachable.
	if v := validateStorage(""); v.Reachable || v.Writable {
		t.Errorf("empty path should be neither reachable nor writable, got %+v", v)
	}
}

func TestOnboardingStateEndpoint(t *testing.T) {
	d := newTestApp(t)
	h := d.onboardingHandler()

	req := httptest.NewRequest(http.MethodGet, "/__onb/state", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("state status = %d", rec.Code)
	}
	var got struct {
		Lang       string     `json:"lang"`
		Path       string     `json:"path"`
		Validation validation `json:"validation"`
		Version    string     `json:"version"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode state: %v\n%s", err, rec.Body.String())
	}
	if got.Lang != "en" && got.Lang != "zh" {
		t.Errorf("lang = %q, want en|zh", got.Lang)
	}
	if got.Path == "" {
		t.Error("expected a default path")
	}
}

func TestOnboardingIndexServed(t *testing.T) {
	d := newTestApp(t)
	rec := httptest.NewRecorder()
	d.onboardingHandler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("index status = %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Lumilio Photos") {
		t.Error("index HTML should contain the product name")
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("index content-type = %q", ct)
	}
}

func TestOnboardingComplete(t *testing.T) {
	d := newTestApp(t)
	h := d.onboardingHandler()
	lib := filepath.Join(t.TempDir(), "My Library")
	defaultLib, err := d.sup.DefaultStoragePath()
	if err != nil {
		t.Fatal(err)
	}

	body := `{"path":` + jsonString(lib) + `,"lang":"zh","agreed":true}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/__onb/complete", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("complete status = %d: %s", rec.Code, rec.Body.String())
	}

	if !d.onboardingDone() {
		t.Error("onboarding flag should be set after complete")
	}
	select {
	case <-d.onboardCh:
	default:
		t.Error("onboarding channel should be closed after complete")
	}

	settings, err := d.sup.Settings()
	if err != nil {
		t.Fatalf("Settings: %v", err)
	}
	if !settings.OnboardingCompleted {
		t.Error("OnboardingCompleted not persisted")
	}
	if settings.StoragePath != defaultLib {
		t.Errorf("StoragePath = %q, want fixed default %q", settings.StoragePath, defaultLib)
	}
	if settings.Language != "zh" {
		t.Errorf("Language = %q, want zh", settings.Language)
	}
	if settings.TOSAcceptedVersion != tosVersion {
		t.Errorf("TOSAcceptedVersion = %q, want %q", settings.TOSAcceptedVersion, tosVersion)
	}
	if d.sup.NeedsOnboarding(tosVersion) {
		t.Error("NeedsOnboarding should be false after completion")
	}
	if !d.sup.NeedsOnboarding("some-newer-revision") {
		t.Error("a bumped ToS revision should re-prompt a completed user")
	}
}

func TestOnboardingCompleteRejectsUnaccepted(t *testing.T) {
	d := newTestApp(t)
	h := d.onboardingHandler()
	lib := filepath.Join(t.TempDir(), "lib")

	body := `{"path":` + jsonString(lib) + `,"lang":"en","agreed":false}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/__onb/complete", strings.NewReader(body)))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 when terms not accepted", rec.Code)
	}
	if d.onboardingDone() {
		t.Error("onboarding must not complete when terms are declined")
	}
}

func TestOnboardingCompleteIgnoresCallerSuppliedStoragePath(t *testing.T) {
	d := newTestApp(t)
	h := d.onboardingHandler()
	// Host paths are authorized later through the running Control Panel. A stale
	// or tampered onboarding payload cannot redirect the default media root.
	lib := filepath.Join(t.TempDir(), "missing", "lib")

	body := `{"path":` + jsonString(lib) + `,"lang":"en","agreed":true}`
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, "/__onb/complete", strings.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 using the fixed local default: %s", rec.Code, rec.Body.String())
	}
	settings, err := d.sup.Settings()
	if err != nil {
		t.Fatal(err)
	}
	if settings.StoragePath == lib {
		t.Fatalf("caller-supplied path %q was persisted", lib)
	}
}

func TestLegalEndpoints(t *testing.T) {
	d := newTestApp(t)
	h := d.onboardingHandler()
	for _, path := range []string{"/__onb/legal/license", "/__onb/legal/third-party", "/__onb/legal/terms", "/__onb/legal/terms?lang=zh"} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Errorf("%s status = %d", path, rec.Code)
		}
		if rec.Body.Len() < 500 {
			t.Errorf("%s text suspiciously short (%d bytes)", path, rec.Body.Len())
		}
	}
}

func jsonString(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}
