package main

import (
	"errors"
	"log"
	"path/filepath"
	"time"

	"desktop/lumen"
	"desktop/supervisor"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// Local-AI (supervised Lumen Hub) lifecycle states shown in the tray.
const (
	lumenOff        = ""
	lumenInstalling = "installing"
	lumenStarting   = "starting"
	lumenRunning    = "running"
	lumenFailed     = "failed"
)

// lumenReadyTimeout is generous because the hub's first start downloads model
// weights (~1.3 GB) before it begins serving.
const lumenReadyTimeout = 30 * time.Minute

// autoStartLumen resumes local AI on launch when the user previously enabled
// it. Runs on its own goroutine after the runtime is up.
func (d *desktopApp) autoStartLumen() {
	settings, err := d.sup.Settings()
	if err != nil || !settings.LumenEnabled {
		return
	}
	d.runLumen()
}

// enableLumen is the tray action: install if needed, persist the choice, run.
// Ignored while an install/start is already underway.
func (d *desktopApp) enableLumen() {
	if d.lumenState == lumenInstalling || d.lumenState == lumenStarting || d.lumenState == lumenRunning {
		return
	}
	go func() {
		settings, err := d.sup.Settings()
		if err != nil {
			log.Printf("lumen: read settings: %v", err)
			return
		}
		settings.LumenEnabled = true
		if err := d.sup.SaveSettings(settings); err != nil {
			log.Printf("lumen: save settings: %v", err)
			return
		}
		d.runLumen()
	}()
}

// disableLumen stops the hub and persists the choice.
func (d *desktopApp) disableLumen() {
	go func() {
		d.stopLumen()

		settings, err := d.sup.Settings()
		if err == nil {
			settings.LumenEnabled = false
			if err := d.sup.SaveSettings(settings); err != nil {
				log.Printf("lumen: save settings: %v", err)
			}
		}
	}()
}

func (d *desktopApp) stopLumen() {
	d.lumenStopRequested.Store(true)
	d.setLumenState(lumenOff)
	d.lumenMu.Lock()
	d.lumenStatus = lumen.Status{}
	d.lumenMu.Unlock()
	if hub := d.lumenHub; hub != nil {
		hub.Stop(10 * time.Second)
		d.lumenHub = nil
	}
	d.lumenStopRequested.Store(false)
}

// runLumen owns one full hub lifecycle: install if missing, start, wait ready,
// then watch for exit. Blocking; run it on a goroutine.
func (d *desktopApp) runLumen() {
	d.lumenMu.Lock()
	d.lumenError = ""
	d.lumenMu.Unlock()
	dir, err := d.sup.LumenDir()
	if err != nil {
		log.Printf("lumen: resolve dir: %v", err)
		d.setLumenState(lumenFailed)
		return
	}

	settings, err := d.sup.Settings()
	if err != nil {
		d.failLumen(err)
		return
	}
	selection, err := d.lumenSelection(dir, settings)
	if err != nil {
		d.failLumen(err)
		return
	}

	installed, ok := lumen.Installed(dir)
	if !ok || installed.Profile != selection.Profile {
		d.setLumenState(lumenInstalling)
		installed, err = lumen.InstallProfile(d.ctx, dir, "", selection.Profile, log.Printf)
		if err != nil {
			log.Printf("lumen: install: %v", err)
			d.failLumen(err)
			return
		}
		settings.LumenInstalledVersion = installed.Version
		settings.LumenInstalledProfile = installed.Profile
		_ = d.sup.SaveSettings(settings)
	}

	d.setLumenState(lumenStarting)
	hub, err := lumen.StartWithConfig(d.ctx, dir, selection, d.lumenLogPath())
	if err != nil {
		log.Printf("lumen: start: %v", err)
		if lumen.RestorePrevious(dir) {
			log.Printf("lumen: restored previous Hub after start failure")
		}
		d.failLumen(err)
		return
	}
	d.lumenHub = hub

	if err := d.awaitLumenReady(hub); err != nil {
		if d.ctx.Err() != nil || d.lumenStopRequested.Load() {
			return // shutting down / deliberately disabled
		}
		log.Printf("lumen: %v", err)
		if lumen.RestorePrevious(dir) {
			log.Printf("lumen: restored previous Hub after readiness failure")
		}
		d.failLumen(err)
		return
	}
	d.setLumenState(lumenRunning)
	log.Printf("lumen: hub ready at %s", lumen.GRPCEndpoint)

	// Block until the process exits; an exit we didn't ask for is a failure.
	exitErr, channelOpen := <-hub.Done()
	if d.ctx.Err() != nil || d.lumenStopRequested.Load() || d.lumenState == lumenOff {
		return
	}
	if channelOpen && exitErr != nil {
		log.Printf("lumen: hub exited: %v", exitErr)
	} else {
		log.Printf("lumen: hub exited unexpectedly")
	}
	d.lumenHub = nil
	d.failLumen(exitErr)
}

// awaitLumenReady blocks until the hub reports READY on its control plane,
// fails, exits, or times out. The status watch keeps running after readiness
// so the dashboard sees live phase/download/error updates; hubs predating the
// control plane fall back to the TCP probe.
func (d *desktopApp) awaitLumenReady(hub *lumen.Hub) error {
	ready := make(chan error, 1)
	report := func(err error) {
		select {
		case ready <- err:
		default:
		}
	}

	go d.watchLumenStatus(hub, report)

	select {
	case err := <-ready:
		return err
	case <-time.After(lumenReadyTimeout):
		return errors.New("lumen hub not ready in time")
	case exitErr, open := <-hub.Done():
		if open && exitErr != nil {
			return exitErr
		}
		return errors.New("lumen hub exited during startup")
	case <-d.ctx.Done():
		return d.ctx.Err()
	}
}

// watchLumenStatus maintains a control-plane watch for the lifetime of one hub
// process, mirroring snapshots into d.lumenStatus / d.lumenState. The first
// READY or FAILED snapshot is also reported through report.
func (d *desktopApp) watchLumenStatus(hub *lumen.Hub, report func(error)) {
	for {
		select {
		case <-hub.Done():
			return
		case <-d.ctx.Done():
			return
		default:
		}

		err := lumen.WatchStatus(d.ctx, lumen.GRPCEndpoint, func(status lumen.Status) {
			d.applyLumenStatus(status)
			switch status.Phase {
			case "ready":
				report(nil)
			case "failed":
				message := status.Error
				if message == "" {
					message = "lumen hub failed to start"
				}
				report(errors.New(message))
			}
		})
		if errors.Is(err, lumen.ErrControlUnsupported) {
			// Old hub build: single TCP readiness probe, no live status.
			log.Printf("lumen: control plane unsupported by this hub build; using TCP probe")
			report(hub.WaitReady(d.ctx, lumenReadyTimeout))
			return
		}

		// Stream ended (hub still binding, restarting, or transient error):
		// retry until the process exits or we shut down.
		select {
		case <-hub.Done():
			return
		case <-d.ctx.Done():
			return
		case <-time.After(500 * time.Millisecond):
		}
	}
}

// applyLumenStatus records the latest control-plane snapshot and derives the
// coarse tray state. The tray/menu only refreshes on state transitions, not on
// every throttled download tick.
func (d *desktopApp) applyLumenStatus(status lumen.Status) {
	d.lumenMu.Lock()
	d.lumenStatus = status
	if status.Phase == "failed" && status.Error != "" {
		d.lumenError = status.Error
	}
	d.lumenMu.Unlock()

	var state string
	switch status.Phase {
	case "ready":
		state = lumenRunning
	case "failed":
		state = lumenFailed
	case "stopping":
		return
	default: // starting/downloading/loading/warmup
		state = lumenStarting
	}
	if d.lumenState != state && d.lumenState != lumenOff {
		d.setLumenState(state)
	}
}

func (d *desktopApp) lumenStatusSnapshot() lumen.Status {
	d.lumenMu.Lock()
	defer d.lumenMu.Unlock()
	return d.lumenStatus
}

func (d *desktopApp) lumenSelection(dir string, settings supervisor.DesktopSettings) (lumen.ConfigSelection, error) {
	defaults, err := lumen.DefaultConfigSelection(dir, d.lang)
	if err != nil {
		return lumen.ConfigSelection{}, err
	}
	if settings.LumenPreset != "" {
		defaults.Preset = settings.LumenPreset
	}
	if settings.LumenBackend != "" {
		defaults.Backend = settings.LumenBackend
	}
	if settings.LumenProfile != "" {
		defaults.Profile = settings.LumenProfile
	}
	if settings.LumenCacheDir != "" {
		defaults.CacheDir = settings.LumenCacheDir
	}
	return defaults, lumen.ValidateConfigSelection(defaults)
}

func (d *desktopApp) failLumen(err error) {
	if err != nil {
		d.lumenError = err.Error()
	}
	d.setLumenState(lumenFailed)
}

func (d *desktopApp) setLumenState(state string) {
	d.lumenState = state
	d.refreshMenu()
}

// lumenLogPath is the hub's output log, next to the server logs so the
// failure dialog's "Logs:" hint covers it too.
func (d *desktopApp) lumenLogPath() string {
	return filepath.Join(d.sup.LogDir(), "lumen-hub.log")
}

// appendLumenMenu adds the local-AI section to the tray menu. Hidden until the
// runtime is up, and entirely absent on hosts with no hub build (Intel macs).
func (d *desktopApp) appendLumenMenu(menu *application.Menu) {
	if !d.ready {
		return
	}
	if _, err := lumen.ProfileForHost(); err != nil {
		return
	}
	menu.AddSeparator()

	switch d.lumenState {
	case lumenInstalling:
		menu.Add(d.tr("aiInstalling")).SetEnabled(false)
	case lumenStarting:
		menu.Add(d.tr("aiStarting")).SetEnabled(false)
	case lumenRunning:
		menu.Add(d.tr("aiRunning")).SetEnabled(false)
		menu.Add(d.tr("aiDisable")).OnClick(func(*application.Context) { d.disableLumen() })
	case lumenFailed:
		menu.Add(d.tr("aiFailed")).SetEnabled(false)
		menu.Add(d.tr("aiRetry")).OnClick(func(*application.Context) { d.enableLumen() })
	default: // off
		menu.Add(d.tr("aiEnable")).OnClick(func(*application.Context) { d.enableLumen() })
	}
}
