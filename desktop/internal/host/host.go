package host

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/lumen"
	"desktop/internal/operation"
	"desktop/internal/panel"
	"desktop/internal/runtime"
	"desktop/internal/shutdown"
	"desktop/internal/state"
	"desktop/internal/storage"
	"desktop/internal/update"
)

type Options struct {
	Store         *state.Store
	Operations    *operation.Registry
	Runtime       *runtime.Controller
	RuntimeConfig control.RuntimeConfigAdapter
	Lumen         *lumen.Controller
	LumenOwner    shutdown.LumenOwner
	Storage       *storage.Controller
	Update        *update.Controller
	Preferences   control.DesktopPreferencesAdapter
	// OpenRuntimeManifest reveals the generated Server manifests in the
	// system's file manager.
	OpenRuntimeManifest func() error
	OpenURL             func(string) error
	OpenProductOnLaunch bool
}

// settingsWindow is the Settings window as the host's lifecycle logic sees
// it. The native window is created on demand and destroyed when closed.
type settingsWindow interface {
	Show()
	Hide()
}

// Host owns what the tray and the Settings window share: the navigation
// guard, opening the product, and the coordinated quit. It holds no UI
// state; the Settings window's lives in [panel.Panel].
type Host struct {
	store               *state.Store
	operations          *operation.Registry
	runtime             *runtime.Controller
	runtimeConfig       control.RuntimeConfigAdapter
	lumen               *lumen.Controller
	storage             *storage.Controller
	update              *update.Controller
	preferences         control.DesktopPreferencesAdapter
	openRuntimeManifest func() error
	shutdown            *shutdown.Coordinator
	openProductOnLaunch bool

	mu       sync.Mutex
	openURL  func(string) error
	window   settingsWindow
	bootOnce sync.Once
	openOnce sync.Once

	// relaunch makes the armed quit start the app again, into an update the
	// updater installed.
	relaunch atomic.Bool
}

func New(options Options) *Host {
	if options.Store == nil {
		options.Store = state.New()
	}
	if options.Operations == nil {
		options.Operations = operation.New()
	}
	if options.OpenURL == nil {
		options.OpenURL = func(string) error { return errors.New("system browser is unavailable") }
	}
	host := &Host{
		store: options.Store, operations: options.Operations,
		runtime: options.Runtime, runtimeConfig: options.RuntimeConfig, lumen: options.Lumen,
		storage: options.Storage, update: options.Update, preferences: options.Preferences,
		openRuntimeManifest: options.OpenRuntimeManifest, openURL: options.OpenURL,
		openProductOnLaunch: options.OpenProductOnLaunch,
	}
	lumenOwner := options.LumenOwner
	if options.Lumen != nil {
		lumenOwner = options.Lumen
	}
	host.shutdown = shutdown.New(host.store, host.operations, options.Runtime, lumenOwner)
	host.commit(func(snapshot *dto.DesktopSnapshot) {
		*snapshot = control.ProjectCapabilities(*snapshot)
	})
	return host
}

func (h *Host) Store() *state.Store { return h.store }

func (h *Host) Runtime() *runtime.Controller { return h.runtime }

func (h *Host) Shutdown() *shutdown.Coordinator { return h.shutdown }

func (h *Host) SetQuitArmed(callback func()) { h.shutdown.SetOnArmed(callback) }

func (h *Host) SetOpenURL(callback func(string) error) {
	if callback == nil {
		return
	}
	h.mu.Lock()
	h.openURL = callback
	h.mu.Unlock()
}

// Services is the validating facade the Settings window calls. Each service
// delegates to an adapter, so the window never reaches a controller.
func (h *Host) Services() panel.Services {
	var runtimeController control.RuntimeController
	if h.runtime != nil {
		runtimeController = h.runtime
	}
	var storageAdapter control.StorageAdapter
	if h.storage != nil {
		storageAdapter = h.storage
	}
	var lumenAdapter control.LumenAdapter
	if h.lumen != nil {
		lumenAdapter = h.lumen
	}
	var updateAdapter control.UpdateAdapter
	if h.update != nil {
		updateAdapter = h.update
	}
	return panel.Services{
		Desktop: &control.DesktopService{
			Store: h.store, Host: h, Preferences: h.preferences, OpenRuntimeManifestAction: h.openRuntimeManifest,
		},
		Runtime: &control.RuntimeService{Controller: runtimeController, Config: h.runtimeConfig},
		Storage: &control.StorageService{Adapter: storageAdapter},
		Lumen:   &control.LumenService{Controller: lumenAdapter},
		Update:  &control.UpdateService{Store: h.store, Adapter: updateAdapter},
	}
}

func (h *Host) attachWindow(window settingsWindow) {
	h.mu.Lock()
	h.window = window
	h.mu.Unlock()
}

func (h *Host) Boot(ctx context.Context) {
	h.bootOnce.Do(func() {
		h.commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Host.BootPhase = "booting" })
		allowLifecycleStart := h.store.Get().Host.Recovery.Code == ""
		if allowLifecycleStart && h.runtime != nil {
			h.runtime.StartActor(ctx)
			snapshot := h.store.Get()
			if snapshot.Runtime.Configured && snapshot.Runtime.DesiredState == dto.DesiredRunning {
				if _, err := h.runtime.Start("boot-runtime", 0); err != nil {
					h.commit(func(current *dto.DesktopSnapshot) {
						current.Host.Recovery = dto.Error{Code: operation.ErrorCodeOf(err), Message: "runtime failed to start"}
					})
				}
			}
		}
		if allowLifecycleStart && h.lumen != nil {
			h.lumen.StartActor(ctx)
			snapshot := h.store.Get()
			if snapshot.Lumen.InstallPhase == dto.LumenInstalled && snapshot.Lumen.DesiredState == dto.DesiredRunning {
				if _, err := h.lumen.Start("boot-lumen", 0); err != nil {
					h.commit(func(current *dto.DesktopSnapshot) {
						current.Host.Recovery = dto.Error{Code: operation.ErrorCodeOf(err), Message: "Lumen failed to start"}
					})
				}
			}
		}
		if h.openProductOnLaunch {
			h.openProductWhenReady(ctx)
		}
		h.commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Host.BootPhase = "ready" })
	})
}

func (h *Host) openProductWhenReady(ctx context.Context) {
	h.openOnce.Do(func() {
		if h.store.Get().Runtime.Capabilities.CanOpenProduct {
			_ = h.OpenProduct()
			return
		}
		ch, cancel := h.store.Subscribe(1)
		go func() {
			defer cancel()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ch:
					snapshot := h.store.Get()
					if snapshot.Runtime.Capabilities.CanOpenProduct {
						_ = h.OpenProduct()
						return
					}
					if snapshot.Host.Recovery.Code != "" || !snapshot.Runtime.Configured || snapshot.Runtime.DesiredState != dto.DesiredRunning || snapshot.Runtime.Phase == dto.RuntimeFailed {
						return
					}
				}
			}
		}()
	})
}

func (h *Host) Close() {
	if h.runtime != nil {
		h.runtime.Close()
	}
	if h.lumen != nil {
		h.lumen.Close()
	}
	if h.operations != nil {
		h.operations.Close()
	}
	if h.store != nil {
		h.store.Close()
	}
}

// ShowSettings records where the Settings window should be and shows it.
func (h *Host) ShowSettings(route string) error {
	if !validRoute(route) {
		return operation.NewError(dto.ErrorInvalidArgument, "unknown Settings route")
	}
	snapshot := h.store.Get()
	// Recovery and onboarding own the navigation: any entry point (tray menu,
	// second-instance args, the window's own sidebar) must not be able to
	// leave them before the user resolves recovery or completes onboarding —
	// otherwise the app shows a Not Configured state with no path back to
	// the wizard.
	if route != panel.RouteRecovery && route != panel.RouteOnboarding {
		if snapshot.Host.Recovery.Code != "" {
			route = panel.RouteRecovery
		} else if !snapshot.Runtime.Configured {
			route = panel.RouteOnboarding
		}
	}
	h.commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Host.SettingsNavigation.Sequence++
		snapshot.Host.SettingsNavigation.Route = route
		snapshot.Host.SettingsVisible = true
	})
	h.mu.Lock()
	window := h.window
	h.mu.Unlock()
	if window != nil {
		window.Show()
	}
	return nil
}

func (h *Host) HideSettings() {
	h.settingsClosed()
	h.mu.Lock()
	window := h.window
	h.mu.Unlock()
	if window != nil {
		window.Hide()
	}
}

// settingsClosed records that the Settings window no longer shows.
func (h *Host) settingsClosed() {
	h.commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Host.SettingsVisible = false })
}

func (h *Host) OpenProduct() error {
	snapshot := h.store.Get()
	if !snapshot.Runtime.Capabilities.CanOpenProduct {
		return operation.NewError(dto.ErrorRuntimeNotReady, "the product is not ready")
	}
	parsed, err := url.Parse(snapshot.Runtime.ProductURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return operation.NewError(dto.ErrorInvalidArgument, "runtime published an invalid product URL")
	}
	h.mu.Lock()
	openURL := h.openURL
	h.mu.Unlock()
	return openURL(parsed.String())
}

func (h *Host) RequestQuit(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	return h.shutdown.RequestQuit(requestID, expectedVersion)
}

// RequestRelaunch quiesces like a quit and starts the app again once the
// shutdown is armed. It is how an installed update is applied.
func (h *Host) RequestRelaunch(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	h.relaunch.Store(true)
	receipt, err := h.shutdown.RequestQuit(requestID, expectedVersion)
	if err != nil {
		h.relaunch.Store(false)
	}
	return receipt, err
}

func (h *Host) ResumeAfterFailedShutdown(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
	h.relaunch.Store(false)
	return h.shutdown.ResumeAfterFailedShutdown(requestID, expectedVersion)
}

func (h *Host) ForceQuit(requestID string, expectedVersion uint64, confirmation string) (dto.OperationReceipt, error) {
	if strings.TrimSpace(confirmation) != "FORCE QUIT" {
		return dto.OperationReceipt{}, operation.NewError(dto.ErrorInvalidArgument, "force quit confirmation is required")
	}
	return h.shutdown.ForceQuit(requestID, expectedVersion, confirmation)
}

func (h *Host) ShouldQuit() bool { return h.shutdown.ShouldQuit() }

func (h *Host) HandleSecondInstance(args []string) {
	for _, arg := range args {
		switch arg {
		case "show-settings":
			_ = h.ShowSettings(panel.RouteGeneral)
		case "open-product":
			_ = h.OpenProduct()
		default:
			_ = h.ShowSettings(panel.RouteGeneral)
		}
		return
	}
	_ = h.ShowSettings(panel.RouteGeneral)
}

func (h *Host) commit(reducer func(*dto.DesktopSnapshot)) {
	h.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		reducer(snapshot)
		*snapshot = control.ProjectCapabilities(*snapshot)
	})
}

func validRoute(route string) bool {
	switch route {
	case panel.RouteOnboarding, panel.RouteGeneral, panel.RouteServer, panel.RouteStorage, panel.RouteLumen,
		panel.RouteUpdates, panel.RouteRecovery, "/diagnostics", "/overview", "/runtime", "/settings":
		return true
	default:
		return false
	}
}

func (h *Host) String() string {
	if h == nil {
		return "<nil host>"
	}
	return fmt.Sprintf("Lumilio Desktop %s", h.store.Get().InstanceID)
}
