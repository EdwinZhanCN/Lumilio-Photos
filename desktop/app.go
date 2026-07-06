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

	"desktop/supervisor"

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
// entitlement. The one webview the host does own is the first-run onboarding
// window (see onboarding.go), which never touches accounts/passkeys.
type desktopApp struct {
	sup  *supervisor.Supervisor
	app  *application.App
	tray *application.SystemTray

	ctx    context.Context
	cancel context.CancelFunc

	lang   string // desktop-native chrome language ("en" or "zh")
	ready  bool
	status string

	// Onboarding coordination.
	onboardWin  *application.WebviewWindow
	onboardOnce sync.Once
	onboardCh   chan struct{}
	onboardFlag atomic.Bool

	lastStage atomic.Value // string: most recent startup stage, for failure dialogs
}

func newDesktopApp() *desktopApp {
	ctx, cancel := context.WithCancel(context.Background())
	d := &desktopApp{
		ctx:       ctx,
		cancel:    cancel,
		onboardCh: make(chan struct{}),
	}
	d.sup = supervisor.New(supervisor.Options{OnStage: d.onStage})
	// Resolve the native-chrome language up front so tray/dialogs are localized
	// from the first frame; onboarding may refine it.
	d.lang = d.onboardingLang()
	d.status = d.tr("starting")
	return d
}

// run creates the menubar item and blocks until the app quits.
func (d *desktopApp) run() error {
	d.app = application.New(application.Options{
		Name:        "Lumilio Photos",
		Description: "Local-first photo management",
		// Accessory = menubar app: no dock icon, no default window.
		Mac: application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
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
	if d.sup.NeedsOnboarding() {
		d.status = d.tr("setup")
		d.refreshMenu()
		d.showOnboarding()
		<-d.onboardCh // wait for /__onb/complete
		if d.onboardWin != nil {
			d.onboardWin.Close()
		}
		// Re-resolve language from the choice just made.
		d.lang = d.onboardingLang()
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

// onStage is the supervisor progress callback: it localizes the stage into the
// tray status. Called from the startup goroutine.
func (d *desktopApp) onStage(stage string) {
	d.lastStage.Store(stage)
	if stage == supervisor.StageReady {
		return // the "Running — <url>" line replaces it in startRuntime
	}
	d.status = d.trStage(stage)
	d.refreshMenu()
}

// refreshMenu rebuilds the tray menu from the current state and re-attaches it,
// so status/enabled changes are reflected after startup.
func (d *desktopApp) refreshMenu() {
	menu := d.app.NewMenu()

	status := menu.Add(d.status)
	status.SetEnabled(false)

	open := menu.Add(d.tr("open"))
	open.SetEnabled(d.ready)
	open.OnClick(func(*application.Context) { d.openInBrowser() })

	menu.AddSeparator()

	quit := menu.Add(d.tr("quit"))
	quit.OnClick(func(*application.Context) { d.app.Quit() })

	d.tray.SetMenu(menu)
}

// startRuntime brings up PostgreSQL + the API server, then opens the browser.
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
		d.app.Quit()
		return
	}
	for _, w := range d.sup.Warnings() {
		log.Printf("desktop startup warning: %s", w)
	}

	d.ready = true
	d.status = fmt.Sprintf(d.tr("running"), strings.TrimPrefix(d.sup.ServerURL(), "http://"))
	d.refreshMenu()

	// Auto-open the app in the default browser on launch.
	d.openInBrowser()
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
	if stage, ok := d.lastStage.Load().(string); ok && stage != "" {
		fmt.Fprintf(&b, "%s\n\n", fmt.Sprintf(d.tr("failStage"), d.trStage(stage)))
	}
	b.WriteString(err.Error())
	if logDir := d.sup.LogDir(); logDir != "" {
		fmt.Fprintf(&b, "\n\n%s", fmt.Sprintf(d.tr("logHint"), logDir))
	}
	return b.String()
}

func (d *desktopApp) openInBrowser() {
	if err := d.app.Browser.OpenURL(d.sup.ServerURL()); err != nil {
		log.Printf("failed to open browser: %v", err)
	}
}

// onShutdown drains the API server and stops PostgreSQL. Wails blocks
// termination until this returns.
func (d *desktopApp) onShutdown() {
	d.cancel()
	if err := d.sup.Stop(); err != nil {
		log.Printf("desktop shutdown error: %v", err)
	}
}

// appVersion is the built version string (set via -ldflags at build time).
func appVersion() string { return buildVersion }
