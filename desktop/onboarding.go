package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"desktop/lumen"
	"desktop/supervisor"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The control-panel UI is a Svelte app in desktop/panel; its build output is
// embedded here. Build it before compiling this module (the desktop-* Makefile
// targets do this): cd panel && vp install && vp run build.
//
//go:embed all:panel/dist
var panelDist embed.FS

// panelFS is panelDist rooted at the dist directory.
var panelFS = func() fs.FS {
	sub, err := fs.Sub(panelDist, "panel/dist")
	if err != nil {
		panic(err)
	}
	return sub
}()

// tosVersion is the accepted-terms revision persisted on completion. Bump it to
// re-prompt users when the bundled licenses or terms change materially
// (NeedsOnboarding compares it against the persisted accepted version).
const tosVersion = "2026-07-09"

// validation is the live check the onboarding window shows for a chosen library
// location: reachable (parent exists), writable, and free space.
type validation struct {
	Reachable bool   `json:"reachable"`
	Writable  bool   `json:"writable"`
	FreeBytes uint64 `json:"freeBytes,omitempty"`
	FreeHuman string `json:"freeHuman,omitempty"`
}

// onboardingHandler serves the first-run setup page and its JSON API. It is wired
// as the Wails asset handler; because the app only ever creates the onboarding
// window (there is no other webview), these routes are hit only during setup.
func (d *desktopApp) onboardingHandler() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/__onb/state", func(w http.ResponseWriter, r *http.Request) {
		path := d.onboardingDefaultPath()
		settings, _ := d.sup.Settings()
		paths, _ := d.sup.DashboardPaths()
		lumenDir, _ := d.sup.LumenDir()
		cacheDir := settings.LumenCacheDir
		if cacheDir == "" {
			cacheDir = filepath.Join(lumenDir, "models")
		}
		cacheValidation := validateStorage(cacheDir)
		if installed, ok := lumen.Installed(lumenDir); ok {
			if settings.LumenInstalledVersion == "" {
				settings.LumenInstalledVersion = installed.Version
			}
			if settings.LumenInstalledProfile == "" {
				settings.LumenInstalledProfile = installed.Profile
			}
		}
		choices, _ := lumen.BackendChoicesForHost()
		hubStatus := d.lumenStatusSnapshot()
		ramGB := totalMemoryGB()
		recommended := lumen.RecommendPreset(ramGB, float64(cacheValidation.FreeBytes)/(1<<30))
		writeJSON(w, map[string]any{
			"mode":       map[bool]string{true: "dashboard", false: "onboarding"}[settings.OnboardingCompleted],
			"lang":       d.onboardingLang(),
			"region":     effectiveRegion(settings.Region, d.onboardingLang()),
			"path":       path,
			"validation": validateStorage(path),
			"version":    appVersion(),
			"tosRev":     tosVersion,
			"ready":      d.ready,
			"serverURL":  d.sup.ServerURL(),
			"stage":      d.status,
			"paths":      paths,
			"lumen": map[string]any{"enabled": settings.LumenEnabled, "state": d.lumenState, "error": d.lumenError,
				"preset": settings.LumenPreset, "backend": settings.LumenBackend, "profile": settings.LumenProfile,
				"cacheDir": cacheDir, "previousCacheDir": settings.LumenPreviousCacheDir,
				"installedVersion": settings.LumenInstalledVersion, "latestVersion": d.lumenLatestVersion,
				"phase": hubStatus.Phase, "download": hubStatus.Download},
			"backends": choices, "presets": lumen.Presets(), "recommendedPreset": recommended,
			"memoryGB": ramGB, "cacheValidation": cacheValidation,
		})
	})

	mux.HandleFunc("/__onb/pick", func(w http.ResponseWriter, r *http.Request) {
		dlg := d.app.Dialog.OpenFile().
			CanChooseDirectories(true).
			CanChooseFiles(false).
			CanCreateDirectories(true).
			SetTitle("Choose photo library location")
		if d.onboardWin != nil {
			dlg = dlg.AttachToWindow(d.onboardWin)
		}
		path, err := dlg.PromptForSingleSelection()
		if err != nil {
			writeJSON(w, map[string]any{"cancelled": true})
			return
		}
		if path == "" {
			writeJSON(w, map[string]any{"cancelled": true})
			return
		}
		writeJSON(w, map[string]any{"path": path, "validation": validateStorage(path)})
	})

	mux.HandleFunc("/__onb/complete", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path        string `json:"path"`
			Lang        string `json:"lang"`
			Region      string `json:"region"`
			Agreed      bool   `json:"agreed"`
			EnableLumen bool   `json:"enableLumen"`
			Preset      string `json:"preset"`
			Backend     string `json:"backend"`
			Profile     string `json:"profile"`
			CacheDir    string `json:"cacheDir"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		if !body.Agreed {
			http.Error(w, "terms not accepted", http.StatusBadRequest)
			return
		}
		storagePath := d.onboardingDefaultPath()
		if v := validateStorage(storagePath); !v.Writable {
			http.Error(w, "storage location is not writable", http.StatusBadRequest)
			return
		}

		settings, err := d.sup.Settings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		settings.StoragePath = storagePath
		settings.Language = normalizeLang(body.Lang)
		settings.Region = effectiveRegion(body.Region, settings.Language)
		settings.TOSAcceptedVersion = tosVersion
		settings.OnboardingCompleted = true
		settings.LumenEnabled = body.EnableLumen
		if body.EnableLumen {
			selection := lumen.ConfigSelection{
				Preset: body.Preset, Backend: body.Backend, Profile: body.Profile,
				CacheDir: body.CacheDir, Region: settings.Region,
			}
			if err := lumen.ValidateConfigSelection(selection); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			settings.LumenPreset, settings.LumenBackend, settings.LumenProfile, settings.LumenCacheDir = body.Preset, body.Backend, body.Profile, body.CacheDir
		}
		if err := d.sup.SaveSettings(settings); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"ok": true})

		// Signal boot() to close the window and start the runtime. Guard against a
		// double signal (the window-closing handler also fires).
		d.markOnboardingDone()
	})

	mux.HandleFunc("/__onb/region", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Region string `json:"region"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		settings, err := d.sup.Settings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		settings.Region = normalizeRegion(body.Region)
		if err := d.sup.SaveSettings(settings); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true, "region": settings.Region})
	})

	mux.HandleFunc("/__onb/pick-cache", func(w http.ResponseWriter, r *http.Request) { d.pickDashboardDir(w, "Choose model cache location") })
	mux.HandleFunc("/__onb/open", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Path string `json:"path"`
		}
		if json.NewDecoder(r.Body).Decode(&body) != nil || strings.TrimSpace(body.Path) == "" {
			http.Error(w, "bad request", 400)
			return
		}
		if err := d.app.Browser.OpenURL((&url.URL{Scheme: "file", Path: body.Path}).String()); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/__onb/open-app", func(w http.ResponseWriter, r *http.Request) {
		d.openInBrowser()
		writeJSON(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/__onb/lumen-save", d.handleLumenSave)
	mux.HandleFunc("/__onb/lumen-action", d.handleLumenAction)
	mux.HandleFunc("/__onb/log", d.handleDashboardLog)
	mux.HandleFunc("/__onb/storage-locations", d.handleStorageLocations)
	mux.HandleFunc("/__onb/pick-storage-location", d.handlePickStorageLocation)
	mux.HandleFunc("/__onb/storage-location-conflict", d.handleStorageLocationConflict)
	mux.HandleFunc("/__onb/remove-storage-location", d.handleRemoveStorageLocation)
	mux.HandleFunc("/__onb/attach-repository", d.handleAttachRepository)
	mux.HandleFunc("/__onb/repository-conflict", d.handleRepositoryConflict)

	mux.HandleFunc("/__onb/legal/license", serveLegalText("licenses/GPL-3.0.txt"))
	mux.HandleFunc("/__onb/legal/third-party", serveLegalText("licenses/THIRD_PARTY_NOTICES.txt"))
	mux.HandleFunc("/__onb/legal/terms", handleTermsOfUse)

	// Everything else serves the embedded control-panel bundle, with a SPA
	// fallback to index.html for any path that is not a built asset.
	assets := http.FileServer(http.FS(panelFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/")
		if name != "" && name != "index.html" {
			if f, err := panelFS.Open(name); err == nil {
				_ = f.Close()
				assets.ServeHTTP(w, r)
				return
			}
		}
		index, err := fs.ReadFile(panelFS, "index.html")
		if err != nil {
			http.Error(w, "control panel bundle missing", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write(index)
	})

	return mux
}

// showOnboarding creates and shows the first-run setup window and blocks (via the
// caller's channel) until completion is signalled. Window/dialog methods marshal
// to the UI thread internally, so this is safe to call from boot()'s goroutine.
func (d *desktopApp) showOnboarding() { d.showControlPanel() }

func (d *desktopApp) showDashboard() { d.showControlPanel() }

func (d *desktopApp) showControlPanel() {
	if d.onboardWin != nil {
		d.onboardWin.Show()
		d.onboardWin.Focus()
		return
	}
	win := d.app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "control-panel",
		Title:     "Lumilio Photos",
		Width:     760,
		Height:    720,
		MinWidth:  640,
		MinHeight: 620,
		Mac: application.MacWindow{
			// Transparent, content-under-titlebar chrome so the page owns its full
			// surface (HIG); the page reserves a drag strip + safe-area insets so
			// nothing underlaps the traffic lights.
			TitleBar:                application.MacTitleBar{AppearsTransparent: true, HideTitle: true, FullSizeContent: true},
			InvisibleTitleBarHeight: 30,
		},
	})
	d.onboardWin = win

	win.OnWindowEvent(events.Common.WindowClosing, func(*application.WindowEvent) {
		// Quitting mid-setup is a valid cancel: there is no validated storage
		// location to boot with, so exit rather than starting half-configured.
		if !d.onboardingDone() {
			d.app.Quit()
		}
		d.onboardWin = nil
	})

	win.Center()
	win.Show()
	win.Focus()
}

func (d *desktopApp) pickDashboardDir(w http.ResponseWriter, title string) {
	dlg := d.app.Dialog.OpenFile().CanChooseDirectories(true).CanChooseFiles(false).CanCreateDirectories(true).SetTitle(title)
	if d.onboardWin != nil {
		dlg = dlg.AttachToWindow(d.onboardWin)
	}
	path, err := dlg.PromptForSingleSelection()
	if err != nil || path == "" {
		writeJSON(w, map[string]any{"cancelled": true})
		return
	}
	writeJSON(w, map[string]any{"path": path, "validation": validateStorage(path)})
}

func totalMemoryGB() float64 {
	v, err := mem.VirtualMemory()
	if err != nil {
		return 0
	}
	return float64(v.Total) / (1 << 30)
}

// onboardingDefaultPath is the fixed machine-local default Storage Location.
// External locations are authorized explicitly from the running Control Panel,
// where they can also be reconciled when a removable volume moves.
func (d *desktopApp) onboardingDefaultPath() string {
	if def, err := d.sup.DefaultStoragePath(); err == nil {
		return def
	}
	return ""
}

// onboardingLang is the desktop-native language to open setup in: a persisted
// choice if any, else the OS locale, else English.
func (d *desktopApp) onboardingLang() string {
	if settings, err := d.sup.Settings(); err == nil && settings.Language != "" {
		return normalizeLang(settings.Language)
	}
	return detectOSLang()
}

// validateStorage checks a candidate library location without creating it: it
// probes the nearest existing ancestor for writability and free space, so the
// user can pick around freely before committing.
func validateStorage(path string) validation {
	if path == "" {
		return validation{}
	}
	v := validation{Reachable: supervisor.StorageReachable(path)}
	if !v.Reachable {
		return v
	}

	probe := path
	for {
		if _, err := os.Stat(probe); err == nil {
			break
		}
		parent := filepath.Dir(probe)
		if parent == probe {
			return v
		}
		probe = parent
	}

	if f, err := os.CreateTemp(probe, ".lumilio-write-*"); err == nil {
		v.Writable = true
		name := f.Name()
		_ = f.Close()
		_ = os.Remove(name)
	}
	if free, err := freeBytes(probe); err == nil {
		v.FreeBytes = free
		v.FreeHuman = humanBytes(free)
	}
	return v
}

// normalizeLang collapses an arbitrary language tag to the two desktop-native
// languages ("zh" or "en").
func normalizeLang(lang string) string {
	if len(lang) >= 2 && (lang[:2] == "zh" || lang[:2] == "ZH") {
		return "zh"
	}
	return "en"
}

// detectOSLang best-effort reads the OS UI language from common env vars,
// defaulting to English. macOS GUI apps rarely set LANG, so English is the safe
// default and the window offers a prominent language toggle regardless.
func detectOSLang() string {
	for _, key := range []string{"LC_ALL", "LC_MESSAGES", "LANG"} {
		if v := os.Getenv(key); v != "" {
			return normalizeLang(v)
		}
	}
	return "en"
}

// humanBytes renders a byte count as a compact human-readable size.
func humanBytes(b uint64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := uint64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}
