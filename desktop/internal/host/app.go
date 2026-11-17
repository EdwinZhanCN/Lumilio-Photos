package host

import (
	"context"
	"log"
	"sync"

	"desktop/internal/control/dto"
	"desktop/internal/locales"
	"desktop/internal/panel"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// AppName is the name the system shows and the single-instance lock is
// taken under. A development build takes the same lock on purpose: it reads
// the same application data, which must have one owner.
const AppName = "Lumilio Photos"

// RunOptions configures the application the host runs.
type RunOptions struct {
	// TrayIcon is a black-on-transparent PNG. macOS tints it for the menu
	// bar, and Windows shows it white on a dark taskbar.
	TrayIcon []byte
	// FrameworkData is where the framework keeps its own small state (the
	// window's place, the chosen language), apart from the files the
	// Desktop controllers own.
	FrameworkData string
	// ShowOnLaunch is the Settings route to open at launch, or "" to start
	// in the tray alone.
	ShowOnLaunch string
}

// Run starts the tray application and returns when it quits. It returns at
// once, without error, when another instance already runs: that instance
// was handed this one's arguments.
func (h *Host) Run(options RunOptions) error {
	mygo.App.SetName(AppName)
	if options.FrameworkData != "" {
		mygo.App.SetPath(mygo.PathUserData, options.FrameworkData)
	}
	if !mygo.App.RequestSingleInstanceLock() {
		return nil
	}
	// The app lives in the menu bar: no Dock icon, and it outlives its
	// windows. The Settings window is created on demand and destroyed when
	// closed, so nothing is kept in memory for a window nobody looks at.
	mygo.App.SetActivationPolicy(mygo.ActivationPolicyAccessory)
	mygo.App.OnWindowAllClosed(func() {})
	mygo.App.OnSecondInstance(func(args []string, _ string) { h.HandleSecondInstance(args) })
	mygo.App.OnBeforeQuit(func(event *mygo.QuitEvent) {
		// A quit from the system (the app menu, a signal) goes through the
		// same coordinated shutdown as the tray's: it is held until the
		// Server and Lumen are released. A session that ends cannot be held.
		if h.ShouldQuit() || event.SessionEnding {
			return
		}
		event.PreventDefault()
		if _, err := h.RequestQuit("system-quit-"+h.store.Get().InstanceID, h.store.Get().Revision); err != nil {
			log.Printf("coordinated quit was not accepted: %v", err)
		}
	})
	mygo.App.OnQuit(h.Close)

	h.SetOpenURL(mygo.Shell.OpenExternal)
	h.SetQuitArmed(func() {
		mygo.RunOnMain(func() {
			if h.relaunch.Load() {
				mygo.App.Relaunch()
				return
			}
			mygo.App.Quit()
		})
	})
	view := &settingsView{host: h}
	if h.storage != nil {
		h.storage.SetPickDirectory(view.pickDirectory)
		h.storage.SetOpenFileManager(openFileManager)
	}
	if h.lumen != nil {
		h.lumen.SetPickDirectory(view.pickDirectory)
	}
	if h.update != nil {
		h.update.SetProvider(frameworkUpdates{})
		h.update.SetApply(func(requestID string, _ uint64) (dto.OperationReceipt, error) {
			return h.RequestRelaunch("update-"+requestID, h.store.Get().Revision)
		})
	}

	mygo.App.WhenReady(func() { h.ready(view, options) })
	return mygo.App.Run()
}

func (h *Host) ready(view *settingsView, options RunOptions) {
	ctx, cancel := context.WithCancel(context.Background())
	mygo.App.OnQuit(cancel)

	view.panel = panel.New(panel.Options{
		Services:         h.Services(),
		Post:             view.post,
		Go:               func(fn func()) { go fn() },
		ApplyPreferences: applyPreferences,
	})
	h.attachWindow(view)
	applyPreferences(h.store.Get().Host.Preferences)
	view.panel.SetSnapshot(h.store.Get())

	tray, err := newTray(h, options.TrayIcon)
	if err != nil {
		// Without a tray there is no way back to a closed window, so the
		// Settings window is the application.
		log.Printf("tray icon is unavailable: %v", err)
		if options.ShowOnLaunch == "" {
			options.ShowOnLaunch = panel.RouteGeneral
		}
	}
	// A menu-bar app that serves the network must not be throttled while
	// none of its windows shows.
	mygo.Power.KeepRunning("Serving " + AppName)
	mygo.App.OnLocaleChanged(func(string) {
		view.retitle()
		if tray != nil {
			tray.rebuild()
		}
	})

	changes, stop := h.store.Subscribe(1)
	go func() {
		defer stop()
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-changes:
				if !ok {
					return
				}
				snapshot := h.store.Get()
				mygo.RunOnMain(func() {
					view.panel.SetSnapshot(snapshot)
					view.invalidate()
					if tray != nil {
						tray.refresh(snapshot)
					}
				})
			}
		}
	}()

	h.Boot(ctx)
	if options.ShowOnLaunch != "" {
		_ = h.ShowSettings(options.ShowOnLaunch)
	}
}

// settingsView owns the native Settings window. Its window is nil while
// closed. The panel outlives the window, so an unsaved draft is still there
// when the window opens again.
type settingsView struct {
	host  *Host
	panel *panel.Panel

	mu     sync.Mutex
	window *mygo.Window
}

func (v *settingsView) current() *mygo.Window {
	v.mu.Lock()
	defer v.mu.Unlock()
	return v.window
}

func (v *settingsView) set(window *mygo.Window) {
	v.mu.Lock()
	v.window = window
	v.mu.Unlock()
}

func (v *settingsView) Show() {
	mygo.RunOnMain(func() {
		if window := v.current(); window != nil && !window.IsDestroyed() {
			if !window.IsVisible() {
				window.Show()
			}
			if !window.IsFocused() {
				window.Focus()
			}
			return
		}
		v.panel.Opened()
		window := mygo.NewWindow(mygo.WindowOptions{
			Title: locales.WindowTitle(), Width: 940, Height: 680, MinWidth: 760, MinHeight: 520,
			Content: ui.View(v.panel.View),
		})
		window.OnClosed(func() {
			v.set(nil)
			v.host.settingsClosed()
		})
		v.set(window)
		window.Focus()
	})
}

func (v *settingsView) Hide() {
	mygo.RunOnMain(func() {
		if window := v.current(); window != nil && !window.IsDestroyed() {
			window.Close()
		}
	})
}

func (v *settingsView) invalidate() {
	if window := v.current(); window != nil && !window.IsDestroyed() {
		window.Invalidate()
	}
}

func (v *settingsView) retitle() {
	if window := v.current(); window != nil && !window.IsDestroyed() {
		window.SetTitle(locales.WindowTitle())
	}
}

// post runs fn on the UI thread and draws the window again if it is open.
func (v *settingsView) post(fn func()) {
	mygo.RunOnMain(func() {
		fn()
		v.invalidate()
	})
}

// pickDirectory shows the system's folder picker. It blocks until the
// person answers and returns "" when they cancel, so it runs off the UI
// thread.
func (v *settingsView) pickDirectory(title string) (string, error) {
	paths, err := mygo.Dialog.Open(mygo.OpenDialogOptions{
		Parent: v.current(), Title: title, Directory: true, CreateDirectories: true,
	})
	if err != nil || len(paths) == 0 {
		return "", err
	}
	return paths[0], nil
}

func openFileManager(path string, selectFile bool) error {
	if selectFile {
		mygo.Shell.ShowItemInFolder(path)
		return nil
	}
	return mygo.Shell.OpenPath(path)
}

// applyPreferences makes the host follow the Desktop preferences: the
// language the whole app shows and its light or dark appearance.
func applyPreferences(preferences dto.DesktopPreferences) {
	if tag := localeTag(preferences.Locale); mygo.App.ChosenLocale() != tag {
		mygo.App.SetLocale(tag)
	}
	source := mygo.ThemeSystem
	switch preferences.Theme {
	case "light":
		source = mygo.ThemeLight
	case "dark":
		source = mygo.ThemeDark
	}
	if mygo.Theme.Source() != source {
		mygo.Theme.SetSource(source)
	}
}

// localeTag maps the language a Desktop preference stores to the tag of its
// catalog.
func localeTag(locale string) string {
	switch locale {
	case "zh-CN", "zh-Hans", "zh":
		return "zh-Hans"
	default:
		return "en"
	}
}

// OpenFolder shows a folder in the system's file manager.
func OpenFolder(path string) error { return openFileManager(path, false) }
