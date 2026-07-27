package main

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"desktop/lumen"
	"desktop/supervisor"
	serverapp "server/app"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// buildVersion is the product version, injected at build time via
// -ldflags "-X main.buildVersion=<version>". It mirrors the server version the
// same build stamps into server/internal/version.
var buildVersion = "dev"

//go:embed assets/lumilio-photos-tray.png
var trayIcon []byte

// desktopApp is a menubar (system tray) controller: it starts the private
// database + API server via the supervisor and exposes "Open in browser" + a
// status line + Quit. There is deliberately no embedded webview for the app UI —
// that runs in the user's real browser at http://localhost:6680, which (unlike an
// embedded WKWebView) surfaces platform passkeys correctly and needs no Apple
// entitlement. The one webview the host does own is the private onboarding and
// supervisor Control Panel (see onboarding.go), which never handles product
// accounts or passkeys.
type desktopApp struct {
	sup  *supervisor.Supervisor
	app  *application.App
	tray *application.SystemTray

	ctx    context.Context
	cancel context.CancelFunc

	lang string // desktop-native chrome language ("en" or "zh")

	// Update availability, populated by an async post-launch check.
	updateVersion string
	updateURL     string

	// Onboarding coordination.
	onboardWin  *application.WebviewWindow
	onboardOnce sync.Once
	onboardCh   chan struct{}
	onboardFlag atomic.Bool
	// nativeDirectoryPicker is a test seam for proving that cancelled native
	// grants stop before the in-process repository control plane is accessed.
	nativeDirectoryPicker func(title string, canCreate bool) (string, bool)

	// Local AI (supervised Lumen Hub) — see lumen_app.go.
	lumenHub           *lumen.Hub
	lumenState         string
	lumenStopRequested atomic.Bool
	lumenMu            sync.Mutex
	lumenError         string
	lumenStatus        lumen.Status // latest control-plane snapshot (guarded by lumenMu)
	lumenLatestVersion string
}

func newDesktopApp(controls ...serverapp.OperatorControls) *desktopApp {
	ctx, cancel := context.WithCancel(context.Background())
	d := &desktopApp{
		ctx:       ctx,
		cancel:    cancel,
		onboardCh: make(chan struct{}),
	}
	operatorControls := serverapp.OperatorControls{}
	if len(controls) > 0 {
		operatorControls = controls[0]
	}
	d.sup = supervisor.New(supervisor.Options{
		OnSnapshot:       d.onRuntimeSnapshot,
		OperatorControls: operatorControls,
	})
	// Do not read or migrate persisted settings until run has acquired the host
	// lock. The OS locale is sufficient for construction-time fallback chrome.
	d.lang = detectOSLang()
	return d
}

// prepareHost acquires the single-instance boundary before Wails creates any
// tray or window. Only another live Desktop host is fatal here; ordinary
// config/reconciliation failures are retried by Start and surfaced through the
// retained recovery dashboard.
func (d *desktopApp) prepareHost() error {
	if err := d.sup.Prepare(); err != nil {
		if runtimeStartFailureIsHostFatal(err) {
			return err
		}
		log.Printf("desktop host preparation deferred to runtime recovery: %v", err)
	}
	d.lang = d.onboardingLang()
	return nil
}

// run owns the host lock, creates the menubar item, and blocks until the app
// quits. No persisted host state is read before prepareHost.
func (d *desktopApp) run() error {
	if err := d.prepareHost(); err != nil {
		return err
	}
	d.app = application.New(application.Options{
		Name:        "Lumilio Photos",
		Description: "Local-first photo management",
		// Accessory = menubar app: no dock icon, no default window.
		Mac: application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
		// This is a tray app; closing the first-run onboarding window (the only
		// window it ever creates) must NOT quit it — otherwise the app exits after
		// setup and before the runtime starts. On Windows/Linux the auto-quit is on
		// by default, so disable it explicitly (macOS already defaults off). Quit
		// happens only through the tray's "Quit" item.
		Windows: application.WindowsOptions{DisableQuitOnLastWindowClosed: true},
		Linux:   application.LinuxOptions{DisableQuitOnLastWindowClosed: true},
		// The asset handler serves the first-run onboarding window (and its JSON
		// API). It is only ever reached when that window is created.
		Assets:     application.AssetOptions{Handler: d.onboardingHandler()},
		OnShutdown: d.onShutdown,
	})

	d.tray = d.app.SystemTray.New()
	d.tray.SetTemplateIcon(trayIcon)
	d.tray.SetTooltip("Lumilio Photos")
	d.refreshMenu()

	// Boot the runtime once the app's event loop is running.
	d.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go d.boot()
	})

	return d.app.Run()
}

// boot runs the first-run onboarding window if needed, then brings up the
// runtime. It runs on its own goroutine; Wails window/tray methods marshal to the
// UI thread internally.
func (d *desktopApp) boot() {
	if d.sup.NeedsOnboarding(tosVersion) {
		d.refreshMenu()
		d.showOnboarding()
		<-d.onboardCh // wait for /__onb/complete
		// Re-resolve language from the choice just made.
		d.lang = d.onboardingLang()
	} else {
		d.markOnboardingDone()
	}
	d.startRuntime()
}

// markOnboardingDone signals completion exactly once.
func (d *desktopApp) markOnboardingDone() {
	d.onboardFlag.Store(true)
	d.onboardOnce.Do(func() { close(d.onboardCh) })
}

// onboardingDone reports whether onboarding has been completed (used to
// distinguish a completion-close from a user cancel).
func (d *desktopApp) onboardingDone() bool { return d.onboardFlag.Load() }

// onRuntimeSnapshot keeps native tray state on the same typed source consumed
// by the private control panel.
func (d *desktopApp) onRuntimeSnapshot(supervisor.RuntimeSnapshot) {
	if d.app != nil && d.tray != nil {
		d.refreshMenu()
	}
}

// refreshMenu rebuilds the tray menu from the current state and re-attaches it,
// so status/enabled changes are reflected after startup.
func (d *desktopApp) refreshMenu() {
	menu := d.app.NewMenu()
	runtime := d.sup.RuntimeSnapshot()

	status := menu.Add(d.runtimeStatusText(runtime))
	status.SetEnabled(false)

	open := menu.Add(d.tr("open"))
	open.SetEnabled(runtime.CanOpen)
	open.OnClick(func(*application.Context) { d.openInBrowser() })

	dashboard := menu.Add(d.tr("dashboard"))
	dashboard.OnClick(func(*application.Context) { d.showDashboard() })

	if d.updateURL != "" {
		upd := menu.Add(fmt.Sprintf(d.tr("updateAvailable"), d.updateVersion))
		upd.OnClick(func(*application.Context) {
			if err := d.app.Browser.OpenURL(d.updateURL); err != nil {
				log.Printf("failed to open update URL: %v", err)
			}
		})
	}

	d.appendLumenMenu(menu)

	menu.AddSeparator()

	quit := menu.Add(d.tr("quit"))
	quit.OnClick(func(*application.Context) { d.app.Quit() })

	d.tray.SetMenu(menu)
}

func (d *desktopApp) runtimeStatusText(runtime supervisor.RuntimeSnapshot) string {
	if !d.onboardingDone() {
		return d.tr("setup")
	}
	switch runtime.Phase {
	case supervisor.RuntimeRunning:
		return fmt.Sprintf(d.tr("running"), strings.TrimPrefix(runtime.BrowserURL, "http://"))
	case supervisor.RuntimeRestarting:
		return d.tr("restarting")
	case supervisor.RuntimeFailed:
		return d.tr("runtimeFailed")
	case supervisor.RuntimeStarting:
		if runtime.Stage != "" {
			return d.trStage(runtime.Stage)
		}
		return d.tr("starting")
	default:
		return d.tr("starting")
	}
}

// startRuntime brings up the in-process SQLite/API runtime, then opens the browser.
func (d *desktopApp) startRuntime() {
	if err := d.sup.Start(d.ctx); err != nil {
		title := d.tr("failTitle")
		switch {
		case errors.Is(err, supervisor.ErrAlreadyRunning):
			title = d.tr("alreadyTitle")
		case errors.Is(err, supervisor.ErrPortInUse):
			title = d.tr("portTitle")
		}
		log.Printf("desktop runtime failed to start: %v", err)
		d.app.Dialog.Error().SetTitle(title).SetMessage(d.failureMessage(err)).Show()
		if runtimeStartFailureIsHostFatal(err) {
			d.app.Quit()
			return
		}
		d.showDashboard()
		d.refreshMenu()
		return
	}
	for _, w := range d.sup.Warnings() {
		log.Printf("desktop startup warning: %s", w)
	}

	d.refreshMenu()

	// Auto-open the app in the default browser on launch.
	d.openInBrowser()

	// Check for a newer release in the background (best-effort, off the critical
	// path); surface it in the tray if found.
	go d.checkUpdate()
	go d.checkLumenUpdate()

	// Resume local AI if the user enabled it previously.
	go d.autoStartLumen()
}

func runtimeStartFailureIsHostFatal(err error) bool {
	return errors.Is(err, supervisor.ErrAlreadyRunning)
}

// checkUpdate queries GitHub for a newer release and, if one exists, shows it in
// the tray. Best-effort: any failure is silent.
func (d *desktopApp) checkUpdate() {
	info, ok := checkForUpdate(d.ctx, buildVersion, d.desktopRegion())
	if !ok {
		return
	}
	log.Printf("update available: %s (current %s)", info.Version, buildVersion)
	d.updateVersion = info.Version
	d.updateURL = info.URL
	d.refreshMenu()
}

// desktopRegion returns the persisted desktop download region (cn|other).
func (d *desktopApp) desktopRegion() string {
	settings, err := d.sup.Settings()
	if err != nil {
		return defaultRegion(d.lang)
	}
	return effectiveRegion(settings.Region, d.onboardingLang())
}

// failureMessage composes an actionable error: which stage failed, the cause, and
// where to find the logs.
func (d *desktopApp) failureMessage(err error) string {
	var b strings.Builder
	// A port conflict has a clear, actionable explanation; lead with it instead of
	// the raw bind error.
	if errors.Is(err, supervisor.ErrPortInUse) {
		b.WriteString(d.tr("portHint"))
		if logDir := d.sup.LogDir(); logDir != "" {
			fmt.Fprintf(&b, "\n\n%s", fmt.Sprintf(d.tr("logHint"), logDir))
		}
		return b.String()
	}
	if stage := d.sup.RuntimeSnapshot().Stage; stage != "" {
		fmt.Fprintf(&b, "%s\n\n", fmt.Sprintf(d.tr("failStage"), d.trStage(stage)))
	}
	b.WriteString(err.Error())
	if logDir := d.sup.LogDir(); logDir != "" {
		fmt.Fprintf(&b, "\n\n%s", fmt.Sprintf(d.tr("logHint"), logDir))
	}
	return b.String()
}

func (d *desktopApp) openInBrowser() {
	runtime := d.sup.RuntimeSnapshot()
	if !runtime.CanOpen || runtime.BrowserURL == "" {
		return
	}
	if err := d.app.Browser.OpenURL(runtime.BrowserURL); err != nil {
		log.Printf("failed to open browser: %v", err)
	}
}

// onShutdown stops the local AI hub and drains the in-process API/SQLite
// runtime. Wails blocks termination until this returns. The hub is stopped
// before the context is cancelled so it gets a graceful signal rather than the
// CommandContext kill.
func (d *desktopApp) onShutdown() {
	d.lumenStopRequested.Store(true)
	if hub := d.lumenHub; hub != nil {
		hub.Stop(10 * time.Second)
	}
	d.cancel()
	if err := d.sup.Close(); err != nil {
		log.Printf("desktop shutdown error: %v", err)
	}
}

// appVersion is the built version string (set via -ldflags at build time).
func appVersion() string { return buildVersion }
