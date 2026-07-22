package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"desktop/lumen"
	serverapp "server/app"
)

const dashboardLogTailLines = 240

var dashboardLogFiles = map[string]string{
	"app":      "app.log",
	"error":    "error.log",
	"postgres": "postgres.log",
	"lumen":    "lumen-hub.log",
}

func (d *desktopApp) handleStorageLocations(w http.ResponseWriter, r *http.Request) {
	control, err := d.sup.RepositoryControl()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	locations, err := control.ListStorageLocations(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"locations": locations})
}

func (d *desktopApp) handlePickStorageLocation(w http.ResponseWriter, r *http.Request) {
	path, cancelled := d.pickNativeDirectory("Add Storage Location", true)
	if cancelled {
		writeJSON(w, map[string]any{"cancelled": true})
		return
	}
	control, err := d.sup.RepositoryControl()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	location, warnings, err := control.AddStorageLocation(r.Context(), path, filepath.Base(path))
	if err != nil {
		var conflict *serverapp.StorageLocationIdentityConflict
		if errors.As(err, &conflict) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			writeJSON(w, map[string]any{
				"message":  "Storage Location identity is already registered",
				"conflict": conflict,
			})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"location": location, "warnings": warnings})
}

func (d *desktopApp) handleStorageLocationConflict(w http.ResponseWriter, r *http.Request) {
	var body struct {
		RootID string `json:"rootId"`
		Path   string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := d.sup.RepositoryControl()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	location, err := control.ResolveStorageLocationConflict(r.Context(), body.RootID, body.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"location": location})
}

func (d *desktopApp) handleRemoveStorageLocation(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.ID) == "" {
		http.Error(w, "storage location id is required", http.StatusBadRequest)
		return
	}
	control, err := d.sup.RepositoryControl()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	if err := control.RemoveStorageLocation(r.Context(), body.ID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
}

func (d *desktopApp) handleAttachRepository(w http.ResponseWriter, r *http.Request) {
	path, cancelled := d.pickNativeDirectory("Attach Lumilio Repository", false)
	if cancelled {
		writeJSON(w, map[string]any{"cancelled": true})
		return
	}
	control, err := d.sup.RepositoryControl()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	repository, err := control.AttachRepository(r.Context(), path)
	if err != nil {
		var conflict *serverapp.RepositoryIdentityConflict
		if errors.As(err, &conflict) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusConflict)
			writeJSON(w, map[string]any{
				"message":  "Repository identity is already registered",
				"conflict": conflict,
			})
			return
		}
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"repository": repository})
}

func (d *desktopApp) handleRepositoryConflict(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action       string `json:"action"`
		RepositoryID string `json:"repositoryId"`
		Path         string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}
	control, err := d.sup.RepositoryControl()
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	repository, err := control.ResolveRepositoryConflict(r.Context(), body.Action, body.RepositoryID, body.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]any{"repository": repository})
}

func (d *desktopApp) pickNativeDirectory(title string, canCreate bool) (string, bool) {
	if d.nativeDirectoryPicker != nil {
		return d.nativeDirectoryPicker(title, canCreate)
	}
	dialog := d.app.Dialog.OpenFile().
		CanChooseDirectories(true).
		CanChooseFiles(false).
		CanCreateDirectories(canCreate).
		SetTitle(title)
	if d.onboardWin != nil {
		dialog = dialog.AttachToWindow(d.onboardWin)
	}
	path, err := dialog.PromptForSingleSelection()
	return path, err != nil || strings.TrimSpace(path) == ""
}

// handleDashboardLog exposes a bounded, read-only tail of known local logs to
// the private control-panel webview. The fixed key map deliberately prevents a
// caller from using this endpoint as an arbitrary file reader. The Lumen tab
// prefers the hub's structured in-memory tail (control plane) while the hub is
// up, falling back to the log file otherwise.
func (d *desktopApp) handleDashboardLog(w http.ResponseWriter, r *http.Request) {
	key := r.URL.Query().Get("source")
	name, ok := dashboardLogFiles[key]
	if !ok {
		http.Error(w, "unknown log source", http.StatusBadRequest)
		return
	}
	if key == "lumen" && d.lumenHub != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		content, err := lumen.LogTail(ctx, lumen.GRPCEndpoint, dashboardLogTailLines)
		cancel()
		if err == nil {
			writeJSON(w, map[string]any{"source": key,
				"path": lumen.GRPCEndpoint + " · lumen.control.v1/TailLogs", "content": content})
			return
		}
		log.Printf("lumen: control-plane log tail unavailable, using file: %v", err)
	}
	content, err := tailFile(filepath.Join(d.sup.LogDir(), name), dashboardLogTailLines)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"source": key, "path": filepath.Join(d.sup.LogDir(), name), "content": content})
}

func tailFile(path string, limit int) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	const maxRead = int64(8 << 20)
	if size, seekErr := f.Seek(0, io.SeekEnd); seekErr != nil {
		return "", seekErr
	} else if size > maxRead {
		if _, seekErr = f.Seek(-maxRead, io.SeekEnd); seekErr != nil {
			return "", seekErr
		}
	} else if _, seekErr = f.Seek(0, io.SeekStart); seekErr != nil {
		return "", seekErr
	}

	lines := make([]string, 0, limit)
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 64<<10), 1<<20)
	for scanner.Scan() {
		if len(lines) == limit {
			copy(lines, lines[1:])
			lines = lines[:limit-1]
		}
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("read log: %w", err)
	}
	if len(lines) == 0 {
		return "", nil
	}
	result := lines[0]
	for _, line := range lines[1:] {
		result += "\n" + line
	}
	return result, nil
}

type lumenSaveRequest struct {
	Preset   string `json:"preset"`
	Backend  string `json:"backend"`
	Profile  string `json:"profile"`
	CacheDir string `json:"cacheDir"`
}

func (d *desktopApp) handleLumenSave(w http.ResponseWriter, r *http.Request) {
	var body lumenSaveRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad request", 400)
		return
	}
	dir, err := d.sup.LumenDir()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	selection := lumen.ConfigSelection{Preset: body.Preset, Backend: body.Backend, Profile: body.Profile, CacheDir: body.CacheDir, Region: d.desktopRegion()}
	if err := lumen.ValidateConfigSelection(selection); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	if v := validateStorage(body.CacheDir); !v.Writable {
		http.Error(w, "model cache location is not writable", 400)
		return
	}
	if err := lumen.WriteConfigFor(dir, selection); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}
	settings, err := d.sup.Settings()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	if settings.LumenCacheDir != "" && settings.LumenCacheDir != body.CacheDir {
		settings.LumenPreviousCacheDir = settings.LumenCacheDir
	}
	settings.LumenPreset, settings.LumenBackend, settings.LumenProfile, settings.LumenCacheDir = body.Preset, body.Backend, body.Profile, body.CacheDir
	settings.LumenEnabled = true
	if err := d.sup.SaveSettings(settings); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	writeJSON(w, map[string]any{"ok": true})
	go func() { d.stopLumen(); d.runLumen() }()
}

func (d *desktopApp) handleLumenAction(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Action string `json:"action"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		http.Error(w, "bad request", 400)
		return
	}
	switch body.Action {
	case "enable":
		d.enableLumen()
	case "disable":
		d.disableLumen()
	case "restart":
		go func() { d.stopLumen(); d.runLumen() }()
	case "check":
		go d.checkLumenUpdate()
	case "update":
		go d.updateLumen()
	default:
		http.Error(w, "unknown action", 400)
		return
	}
	writeJSON(w, map[string]any{"accepted": true})
}

func (d *desktopApp) checkLumenUpdate() {
	m, err := lumen.FetchManifest(d.ctx, lumen.DefaultManifestURL)
	if err != nil {
		log.Printf("lumen update check: %v", err)
		return
	}
	d.lumenLatestVersion = m.Version
	d.refreshMenu()
}

func (d *desktopApp) updateLumen() {
	d.lumenMu.Lock()
	defer d.lumenMu.Unlock()
	settings, err := d.sup.Settings()
	if err != nil {
		d.failLumen(err)
		return
	}
	dir, err := d.sup.LumenDir()
	if err != nil {
		d.failLumen(err)
		return
	}
	selection, err := d.lumenSelection(dir, settings)
	if err != nil {
		d.failLumen(err)
		return
	}
	d.stopLumen()
	d.setLumenState(lumenInstalling)
	state, err := lumen.InstallProfile(d.ctx, dir, "", selection.Profile, log.Printf)
	if err != nil {
		d.failLumen(err)
		return
	}
	settings.LumenInstalledVersion, settings.LumenInstalledProfile = state.Version, state.Profile
	settings.LumenEnabled = true
	_ = d.sup.SaveSettings(settings)
	go d.runLumen()
}
