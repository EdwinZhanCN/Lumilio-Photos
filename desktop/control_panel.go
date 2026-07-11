package main

import (
	"encoding/json"
	"log"
	"net/http"

	"desktop/lumen"
)

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
	selection := lumen.ConfigSelection{Preset: body.Preset, Backend: body.Backend, Profile: body.Profile, CacheDir: body.CacheDir, Region: regionForLang(d.lang)}
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
