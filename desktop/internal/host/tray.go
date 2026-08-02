package host

import (
	"context"
	"encoding/base64"
	"runtime"
	"sync"
	"sync/atomic"

	"desktop/internal/control"
	"desktop/internal/control/dto"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Tray struct {
	host *Host
	tray *application.SystemTray
	menu *application.Menu

	runtimeRoot    *application.MenuItem
	runtimeStart   *application.MenuItem
	runtimeStop    *application.MenuItem
	runtimeRestart *application.MenuItem
	lumenRoot      *application.MenuItem
	lumenStart     *application.MenuItem
	lumenStop      *application.MenuItem
	lumenRestart   *application.MenuItem
	open           *application.MenuItem
	manageStorage  *application.MenuItem
	storageMenu    *application.Menu
	storageVersion uint64
	storageLoaded  bool
	storageReady   bool
	settings       *application.MenuItem
	quit           *application.MenuItem

	mu       sync.RWMutex
	actions  map[string]TrayActionBinding
	sequence atomic.Uint64
}

type TrayActionBinding struct {
	CommandKind      string
	AggregateVersion uint64
}

func NewTray(app *application.App, host *Host, icon, darkIcon []byte) *Tray {
	tray := &Tray{host: host, actions: make(map[string]TrayActionBinding)}
	tray.tray = app.SystemTray.New()
	tray.menu = app.NewMenu()
	tray.buildMenu()
	if runtime.GOOS == "darwin" {
		// macOS: the framework's pre-click event monitor drives native menu
		// tracking (proper highlight, no app activation). Registering
		// OnClick/OnRightClick replaces that path with the handler, and the
		// programmatic OpenMenu synthesizes a mouse-down that re-enters the
		// monitor — unreliable. Leave both buttons on the native menu.
		tray.tray.SetMenu(tray.menu)
	} else {
		// Windows: NotifyIcon has no native menu-on-click, so both buttons
		// open the menu programmatically.
		tray.tray.SetMenu(tray.menu).OnClick(tray.tray.OpenMenu).OnRightClick(tray.tray.OpenMenu)
	}
	if len(icon) > 0 {
		// SetIcon first (Windows light mode / fallback), then SetTemplateIcon:
		// the template flag only affects macOS, where the black outline is
		// auto-coloured for light/dark menu bars.
		tray.tray.SetIcon(icon).SetTemplateIcon(icon)
	}
	if len(darkIcon) > 0 {
		// Windows has no template mechanism; dark mode needs a white outline.
		tray.tray.SetDarkModeIcon(darkIcon)
	}
	tray.refresh(host.Store().Get())
	ch, cancel := host.Store().Subscribe(1)
	go func() {
		defer cancel()
		for notice := range ch {
			_ = notice
			tray.refresh(host.Store().Get())
		}
	}()
	return tray
}

func (t *Tray) buildMenu() {
	t.runtimeRoot = application.NewSubmenu("Lumilio Photos", application.NewMenu())
	t.menu.Append(application.NewMenuFromItems(t.runtimeRoot))
	t.runtimeStart = t.runtimeRoot.GetSubmenu().Add("Start")
	t.runtimeStop = t.runtimeRoot.GetSubmenu().Add("Stop")
	t.runtimeRestart = t.runtimeRoot.GetSubmenu().Add("Restart")
	t.runtimeStart.OnClick(func(*application.Context) { t.dispatch("runtime-start") })
	t.runtimeStop.OnClick(func(*application.Context) { t.dispatch("runtime-stop") })
	t.runtimeRestart.OnClick(func(*application.Context) { t.dispatch("runtime-restart") })

	t.lumenRoot = application.NewSubmenu("Lumen Hub", application.NewMenu())
	t.menu.Append(application.NewMenuFromItems(t.lumenRoot))
	t.lumenStart = t.lumenRoot.GetSubmenu().Add("Start")
	t.lumenStop = t.lumenRoot.GetSubmenu().Add("Stop")
	t.lumenRestart = t.lumenRoot.GetSubmenu().Add("Restart")
	t.lumenStart.OnClick(func(*application.Context) { t.dispatch("lumen-start") })
	t.lumenStop.OnClick(func(*application.Context) { t.dispatch("lumen-stop") })
	t.lumenRestart.OnClick(func(*application.Context) { t.dispatch("lumen-restart") })

	t.menu.AddSeparator()
	t.open = t.menu.Add("Open")
	t.open.OnClick(func(*application.Context) { _ = t.host.OpenProduct() })
	storage := application.NewSubmenu("Storage Locations", application.NewMenu())
	t.storageMenu = storage.GetSubmenu()
	t.menu.Append(application.NewMenuFromItems(storage))
	t.manageStorage = t.storageMenu.Add("Manage Storage Locations…")
	t.manageStorage.OnClick(func(*application.Context) { _ = t.host.ShowSettings("/storage") })
	t.settings = t.menu.Add("Settings…")
	t.settings.OnClick(func(*application.Context) { _ = t.host.ShowSettings("/general") })
	t.menu.AddSeparator()
	t.quit = t.menu.Add("Quit Lumilio Photos")
	t.quit.OnClick(func(*application.Context) {
		snapshot := t.host.Store().Get()
		_, _ = t.host.RequestQuit(t.nextRequestID("quit"), snapshot.Revision)
	})
}

func (t *Tray) refresh(snapshot dto.DesktopSnapshot) {
	snapshot = control.ProjectCapabilities(snapshot)
	runtime := snapshot.Runtime
	lumen := snapshot.Lumen
	t.runtimeRoot.SetLabel("Lumilio Photos (" + runtime.Presentation.Label + ")")
	t.runtimeRoot.SetBitmap(bitmapFor(runtime.Presentation.Color))
	t.lumenRoot.SetLabel("Lumen Hub (" + lumen.Presentation.Label + ")")
	t.lumenRoot.SetBitmap(bitmapFor(lumen.Presentation.Color))

	t.runtimeStart.SetEnabled(runtime.Capabilities.CanStartRuntime)
	t.runtimeStop.SetEnabled(runtime.Capabilities.CanStopRuntime || runtime.Capabilities.CanRetryCleanupRuntime)
	t.runtimeRestart.SetEnabled(runtime.Capabilities.CanRestartRuntime)
	t.lumenStart.SetEnabled(lumen.Capabilities.CanStartLumen)
	t.lumenStop.SetEnabled(lumen.Capabilities.CanStopLumen || lumen.Capabilities.CanRetryCleanupLumen)
	t.lumenRestart.SetEnabled(lumen.Capabilities.CanRestartLumen)
	t.open.SetEnabled(runtime.Capabilities.CanOpenProduct)
	// Storage management needs a configured, running runtime; during
	// onboarding the entry point is the wizard itself, not the tray.
	t.manageStorage.SetEnabled(snapshot.Runtime.Configured && snapshot.Host.Shutdown.Phase != dto.ShutdownArmed)
	t.refreshStorage(snapshot)
	t.settings.SetEnabled(snapshot.Host.Shutdown.Phase != dto.ShutdownArmed)
	t.quit.SetEnabled(snapshot.Host.Shutdown.Phase == dto.ShutdownIdle || snapshot.Host.Shutdown.Phase == dto.ShutdownFailed)

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
	t.menu.Update()
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

func (t *Tray) refreshStorage(snapshot dto.DesktopSnapshot) {
	if t.host.storage == nil || t.storageMenu == nil {
		return
	}
	runtimeReady := snapshot.Runtime.Phase == dto.RuntimeRunning && snapshot.Runtime.Ownership == dto.OwnershipHeld
	if t.storageLoaded && t.storageVersion == snapshot.Storage.Version && (!runtimeReady || t.storageReady) {
		return
	}
	items, err := t.host.storage.ListShortcuts(context.Background())
	if err != nil {
		return
	}
	t.storageVersion = snapshot.Storage.Version
	t.storageLoaded = true
	t.storageReady = runtimeReady
	t.storageMenu.Clear()
	for _, item := range items {
		shortcut := item
		menuItem := t.storageMenu.Add(shortcut.Name)
		menuItem.SetEnabled(shortcut.CanOpen && snapshot.Host.Shutdown.Phase != dto.ShutdownQuiescing && snapshot.Host.Shutdown.Phase != dto.ShutdownFailed && snapshot.Host.Shutdown.Phase != dto.ShutdownArmed)
		menuItem.OnClick(func(*application.Context) {
			go func() { _ = t.host.storage.OpenLocation(context.Background(), shortcut.ID) }()
		})
	}
	if len(items) > 0 {
		t.storageMenu.AddSeparator()
	}
	t.manageStorage = t.storageMenu.Add("Manage Storage Locations…")
	t.manageStorage.OnClick(func(*application.Context) { _ = t.host.ShowSettings("/storage") })
	t.manageStorage.SetEnabled(snapshot.Host.Shutdown.Phase != dto.ShutdownArmed)
}

func (t *Tray) nextRequestID(prefix string) string {
	return prefix + "-" + t.host.Store().Get().InstanceID + "-" + uintString(t.sequence.Add(1))
}

func uintString(value uint64) string {
	// Avoid fmt in tray callbacks; request IDs are opaque but deterministic
	// within one host instance.
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}

func bitmapFor(color dto.DotColor) []byte {
	encoded := map[dto.DotColor]string{
		dto.DotGray:   "iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAD0lEQVR42mNowAEYhpYEAILzYAEllbNjAAAAAElFTkSuQmCC",
		dto.DotYellow: "iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAEUlEQVR42mN4tIoBK2IYWhIA54JjAQalaOsAAAAASUVORK5CYII=",
		dto.DotGreen:  "iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAEUlEQVR42mPQXRWKFTEMLQkARHtLASUwB/oAAAAASUVORK5CYII=",
		dto.DotRed:    "iVBORw0KGgoAAAANSUhEUgAAAAgAAAAICAIAAABLbSncAAAAEUlEQVR42mM46+aGFTEMLQkA1XdWQaJukugAAAAASUVORK5CYII=",
	}
	data, _ := base64.StdEncoding.DecodeString(encoded[color])
	return data
}
