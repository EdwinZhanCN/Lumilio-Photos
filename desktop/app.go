package main

import (
	"context"
	_ "embed"
	"errors"
	"log"
	"strings"

	"desktop/supervisor"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

//go:embed assets/lumilio-photos-tray.png
var trayIcon []byte

// desktopApp is a menubar (system tray) controller: it starts the private
// database + API server via the supervisor and exposes "Open in browser" + a
// status line + Quit. There is deliberately no embedded webview — the UI runs in
// the user's real browser at http://localhost:6680, which (unlike an embedded
// WKWebView) surfaces platform passkeys correctly and needs no Apple entitlement.
type desktopApp struct {
	sup  *supervisor.Supervisor
	app  *application.App
	tray *application.SystemTray

	ctx    context.Context
	cancel context.CancelFunc

	ready  bool
	status string
}

func newDesktopApp() *desktopApp {
	ctx, cancel := context.WithCancel(context.Background())
	return &desktopApp{
		sup:    supervisor.New(supervisor.Options{}),
		ctx:    ctx,
		cancel: cancel,
		status: "Starting…",
	}
}

// run creates the menubar item and blocks until the app quits.
func (d *desktopApp) run() error {
	d.app = application.New(application.Options{
		Name:        "Lumilio Photos",
		Description: "Local-first photo management",
		// Accessory = menubar app: no dock icon, no default window.
		Mac:        application.MacOptions{ActivationPolicy: application.ActivationPolicyAccessory},
		OnShutdown: d.onShutdown,
	})

	d.tray = d.app.SystemTray.New()
	d.tray.SetTemplateIcon(trayIcon)
	d.tray.SetTooltip("Lumilio Photos")
	d.refreshMenu()

	// Boot the runtime once the app's event loop is running.
	d.app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		go d.startRuntime()
	})

	return d.app.Run()
}

// refreshMenu rebuilds the tray menu from the current state and re-attaches it,
// so status/enabled changes are reflected after startup.
func (d *desktopApp) refreshMenu() {
	menu := d.app.NewMenu()

	status := menu.Add(d.status)
	status.SetEnabled(false)

	open := menu.Add("Open Lumilio Photos")
	open.SetEnabled(d.ready)
	open.OnClick(func(*application.Context) { d.openInBrowser() })

	menu.AddSeparator()

	quit := menu.Add("Quit Lumilio Photos")
	quit.OnClick(func(*application.Context) { d.app.Quit() })

	d.tray.SetMenu(menu)
}

// startRuntime brings up PostgreSQL + the API server, then opens the browser.
func (d *desktopApp) startRuntime() {
	if err := d.sup.Start(d.ctx); err != nil {
		title := "Lumilio Photos failed to start"
		if errors.Is(err, supervisor.ErrAlreadyRunning) {
			title = "Lumilio Photos is already running"
		}
		log.Printf("desktop runtime failed to start: %v", err)
		d.app.Dialog.Error().SetTitle(title).SetMessage(err.Error()).Show()
		d.app.Quit()
		return
	}
	for _, w := range d.sup.Warnings() {
		log.Printf("desktop startup warning: %s", w)
	}

	d.ready = true
	d.status = "Running — " + strings.TrimPrefix(d.sup.ServerURL(), "http://")
	d.refreshMenu()

	// Auto-open the app in the default browser on launch.
	d.openInBrowser()
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
