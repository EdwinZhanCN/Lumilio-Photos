package host

import (
	"bytes"
	"context"
	"image"
	"image/color"
	"image/png"
	"strconv"
	"sync"
	"sync/atomic"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/locales"
	"desktop/internal/panel"

	"github.com/egoist/mygo"
)

// Tray is the menu-bar icon and its native menu. It is built and changed on
// the UI thread; its labels come from the same catalog and the same status
// names as the Settings window.
type Tray struct {
	host *Host
	tray *mygo.Tray
	dots map[dto.DotColor]*mygo.MenuIcon

	runtimeRoot, runtimeStart, runtimeStop, runtimeRestart *mygo.MenuItem
	lumenRoot, lumenStart, lumenStop, lumenRestart         *mygo.MenuItem
	open, storageRoot, manageStorage, settings, quit       *mygo.MenuItem

	shortcuts      []dto.StorageShortcut
	shortcutItems  []*mygo.MenuItem
	storageVersion uint64
	storageLoaded  bool
	storageReady   bool
	storageLoading bool

	mu       sync.RWMutex
	actions  map[string]TrayActionBinding
	sequence atomic.Uint64
}

// TrayActionBinding is the command a menu item sends and the aggregate
// version it was offered at. A click on a menu that showed an older state
// is refused as stale instead of acting on a state the person did not see.
type TrayActionBinding struct {
	CommandKind      string
	AggregateVersion uint64
}

func newTray(host *Host, icon []byte) (*Tray, error) {
	t := &Tray{host: host, actions: make(map[string]TrayActionBinding), dots: make(map[dto.DotColor]*mygo.MenuIcon)}
	for name, ink := range map[dto.DotColor]color.NRGBA{
		dto.DotGray:   {R: 142, G: 142, B: 147, A: 255},
		dto.DotYellow: {R: 255, G: 204, B: 0, A: 255},
		dto.DotGreen:  {R: 52, G: 199, B: 89, A: 255},
		dto.DotRed:    {R: 255, G: 59, B: 48, A: 255},
	} {
		if dot, err := mygo.NewMenuIcon(statusDot(ink), false); err == nil {
			t.dots[name] = dot
		}
	}
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID: "lumilio-photos", Icon: icon, IconIsTemplate: true, ToolTip: locales.TrayToolTip(),
	})
	if err != nil {
		return nil, err
	}
	t.tray = tray
	t.rebuild()
	return t, nil
}

// rebuild makes the menu again. It is needed when its structure changes:
// the Storage Locations it lists, or the language of its labels.
func (t *Tray) rebuild() {
	command := func(name string) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { t.dispatch(name) }
	}
	settingsRoute := func(route string) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { _ = t.host.ShowSettings(route) }
	}
	t.runtimeStart = &mygo.MenuItem{Click: command("runtime-start")}
	t.runtimeStop = &mygo.MenuItem{Click: command("runtime-stop")}
	t.runtimeRestart = &mygo.MenuItem{Click: command("runtime-restart")}
	t.runtimeRoot = &mygo.MenuItem{Submenu: []*mygo.MenuItem{t.runtimeStart, t.runtimeStop, t.runtimeRestart}}
	t.lumenStart = &mygo.MenuItem{Click: command("lumen-start")}
	t.lumenStop = &mygo.MenuItem{Click: command("lumen-stop")}
	t.lumenRestart = &mygo.MenuItem{Click: command("lumen-restart")}
	t.lumenRoot = &mygo.MenuItem{Submenu: []*mygo.MenuItem{t.lumenStart, t.lumenStop, t.lumenRestart}}
	t.open = &mygo.MenuItem{Click: func(*mygo.MenuItem, *mygo.Window) { _ = t.host.OpenProduct() }}

	t.shortcutItems = t.shortcutItems[:0]
	var storage []*mygo.MenuItem
	for _, shortcut := range t.shortcuts {
		id := shortcut.ID
		item := &mygo.MenuItem{Label: shortcut.Name, Click: func(*mygo.MenuItem, *mygo.Window) {
			go func() { _ = t.host.storage.OpenLocation(context.Background(), id) }()
		}}
		t.shortcutItems = append(t.shortcutItems, item)
		storage = append(storage, item)
	}
	if len(storage) > 0 {
		storage = append(storage, mygo.Separator())
	}
	t.manageStorage = &mygo.MenuItem{Click: settingsRoute(panel.RouteStorage)}
	t.storageRoot = &mygo.MenuItem{Submenu: append(storage, t.manageStorage)}
	t.settings = &mygo.MenuItem{Click: settingsRoute(panel.RouteGeneral)}
	t.quit = &mygo.MenuItem{Click: func(*mygo.MenuItem, *mygo.Window) {
		_, _ = t.host.RequestQuit(t.nextRequestID("quit"), t.host.Store().Get().Revision)
	}}

	t.label(control.ProjectCapabilities(t.host.Store().Get()))
	t.tray.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
		t.runtimeRoot, t.lumenRoot, mygo.Separator(),
		t.open, t.storageRoot, t.settings, mygo.Separator(), t.quit,
	}))
	t.tray.SetToolTip(locales.TrayToolTip())
}

// refresh shows a new snapshot in the menu and reads the Storage Locations
// again when they changed.
func (t *Tray) refresh(snapshot dto.DesktopSnapshot) {
	snapshot = control.ProjectCapabilities(snapshot)
	t.label(snapshot)
	t.loadShortcuts(snapshot)
}

// label sets every label and state from a snapshot. Labels are set on each
// call, so they follow the language as it changes.
func (t *Tray) label(snapshot dto.DesktopSnapshot) {
	runtime, lumen, shutdown := snapshot.Runtime, snapshot.Lumen, snapshot.Host.Shutdown.Phase

	t.runtimeRoot.SetLabel(locales.TrayProcess(locales.ProductCompactName(), panel.StatusLabel(runtime.Presentation)))
	t.runtimeRoot.SetIcon(t.dots[runtime.Presentation.Color])
	t.lumenRoot.SetLabel(locales.TrayProcess("Lumen Hub", panel.StatusLabel(lumen.Presentation)))
	t.lumenRoot.SetIcon(t.dots[lumen.Presentation.Color])

	stopLabel := func(retryCleanup bool) string {
		if retryCleanup {
			return locales.CommonRetryCleanup()
		}
		return locales.CommonStop()
	}
	t.runtimeStart.SetLabel(locales.CommonStart())
	t.runtimeStop.SetLabel(stopLabel(runtime.Capabilities.CanRetryCleanupRuntime))
	t.runtimeRestart.SetLabel(locales.CommonRestart())
	t.lumenStart.SetLabel(locales.CommonStart())
	t.lumenStop.SetLabel(stopLabel(lumen.Capabilities.CanRetryCleanupLumen))
	t.lumenRestart.SetLabel(locales.CommonRestart())
	t.open.SetLabel(locales.HeaderOpenProduct())
	t.storageRoot.SetLabel(locales.TrayStorageLocations())
	t.manageStorage.SetLabel(locales.TrayManageStorage())
	t.settings.SetLabel(locales.TraySettings())
	t.quit.SetLabel(locales.TrayQuit())

	t.runtimeStart.SetEnabled(runtime.Capabilities.CanStartRuntime)
	t.runtimeStop.SetEnabled(runtime.Capabilities.CanStopRuntime || runtime.Capabilities.CanRetryCleanupRuntime)
	t.runtimeRestart.SetEnabled(runtime.Capabilities.CanRestartRuntime)
	t.lumenStart.SetEnabled(lumen.Capabilities.CanStartLumen)
	t.lumenStop.SetEnabled(lumen.Capabilities.CanStopLumen || lumen.Capabilities.CanRetryCleanupLumen)
	t.lumenRestart.SetEnabled(lumen.Capabilities.CanRestartLumen)
	t.open.SetEnabled(runtime.Capabilities.CanOpenProduct)
	// Storage management needs a configured runtime; during onboarding the
	// entry point is the wizard itself, not the tray.
	t.manageStorage.SetEnabled(runtime.Configured && shutdown != dto.ShutdownArmed)
	t.settings.SetEnabled(shutdown != dto.ShutdownArmed)
	t.quit.SetEnabled(shutdown == dto.ShutdownIdle || shutdown == dto.ShutdownFailed)
	canOpenLocations := shutdown == dto.ShutdownIdle
	for i, item := range t.shortcutItems {
		item.SetEnabled(canOpenLocations && i < len(t.shortcuts) && t.shortcuts[i].CanOpen)
	}

	t.setAction("runtime-start", "start", runtime.Version)
	if runtime.Capabilities.CanRetryCleanupRuntime {
		t.setAction("runtime-stop", "retry-cleanup", runtime.Version)
	} else {
		t.setAction("runtime-stop", "stop", runtime.Version)
	}
	t.setAction("runtime-restart", "restart", runtime.Version)
	t.setAction("lumen-start", "start", lumen.Version)
	if lumen.Capabilities.CanRetryCleanupLumen {
		t.setAction("lumen-stop", "retry-cleanup", lumen.Version)
	} else {
		t.setAction("lumen-stop", "stop", lumen.Version)
	}
	t.setAction("lumen-restart", "restart", lumen.Version)
}

func (t *Tray) setAction(name, command string, version uint64) {
	t.mu.Lock()
	t.actions[name] = TrayActionBinding{CommandKind: command, AggregateVersion: version}
	t.mu.Unlock()
}

func (t *Tray) dispatch(name string) {
	t.mu.RLock()
	binding, ok := t.actions[name]
	t.mu.RUnlock()
	if !ok {
		return
	}
	requestID := t.nextRequestID("tray")
	switch name {
	case "runtime-start", "runtime-stop", "runtime-restart":
		if t.host.Runtime() == nil {
			return
		}
		switch binding.CommandKind {
		case "start":
			_, _ = t.host.Runtime().Start(requestID, binding.AggregateVersion)
		case "stop":
			_, _ = t.host.Runtime().Stop(requestID, binding.AggregateVersion)
		case "retry-cleanup":
			_, _ = t.host.Runtime().RetryCleanup(requestID, binding.AggregateVersion)
		case "restart":
			_, _ = t.host.Runtime().Restart(requestID, binding.AggregateVersion)
		}
	case "lumen-start", "lumen-stop", "lumen-restart":
		if t.host.lumen == nil {
			return
		}
		switch binding.CommandKind {
		case "start":
			_, _ = t.host.lumen.Start(requestID, binding.AggregateVersion)
		case "stop":
			_, _ = t.host.lumen.Stop(requestID, binding.AggregateVersion)
		case "retry-cleanup":
			_, _ = t.host.lumen.RetryCleanup(requestID, binding.AggregateVersion)
		case "restart":
			_, _ = t.host.lumen.Restart(requestID, binding.AggregateVersion)
		}
	}
}

// loadShortcuts reads the Storage Locations off the UI thread when the
// storage aggregate changed or the Server became ready, then rebuilds the
// menu with them.
func (t *Tray) loadShortcuts(snapshot dto.DesktopSnapshot) {
	if t.host.storage == nil || t.storageLoading {
		return
	}
	ready := snapshot.Runtime.Phase == dto.RuntimeRunning && snapshot.Runtime.Ownership == dto.OwnershipHeld
	if t.storageLoaded && t.storageVersion == snapshot.Storage.Version && (!ready || t.storageReady) {
		return
	}
	t.storageLoading = true
	version := snapshot.Storage.Version
	go func() {
		items, err := t.host.storage.ListShortcuts(context.Background())
		mygo.RunOnMain(func() {
			t.storageLoading = false
			if err != nil {
				return
			}
			t.storageVersion, t.storageLoaded, t.storageReady = version, true, ready
			t.shortcuts = items
			t.rebuild()
		})
	}()
}

func (t *Tray) nextRequestID(prefix string) string {
	return prefix + "-" + t.host.Store().Get().InstanceID + "-" + strconv.FormatUint(t.sequence.Add(1), 10)
}

// statusDot draws the filled circle of a process status for a menu item.
func statusDot(ink color.NRGBA) []byte {
	const size = 32
	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	center, radius := float64(size-1)/2, float64(size)/2-7
	for y := range size {
		for x := range size {
			dx, dy := float64(x)-center, float64(y)-center
			if dx*dx+dy*dy <= radius*radius {
				img.SetNRGBA(x, y, ink)
			}
		}
	}
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, img)
	return buffer.Bytes()
}
