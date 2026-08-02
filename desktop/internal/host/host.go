// Package host adapts the Desktop control plane to Wails. It owns window and
// tray adapters, while runtime and Lumen controllers remain the lifecycle
// owners of their respective processes.
package host

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/lumen"
	"desktop/internal/operation"
	"desktop/internal/runtime"
	"desktop/internal/shutdown"
	"desktop/internal/state"
	"desktop/internal/storage"
	"desktop/internal/update"

	"github.com/wailsapp/wails/v3/pkg/application"
)

type Options struct {
	Store               *state.Store
	Operations          *operation.Registry
	Runtime             *runtime.Controller
	RuntimeConfig       control.RuntimeConfigAdapter
	Lumen               *lumen.Controller
	LumenOwner          shutdown.LumenOwner
	Storage             *storage.Controller
	Update              *update.Controller
	OpenURL             func(string) error
	OpenProductOnLaunch bool
}

type Host struct {
	store               *state.Store
	operations          *operation.Registry
	runtime             *runtime.Controller
	runtimeConfig       control.RuntimeConfigAdapter
	lumen               *lumen.Controller
	storage             *storage.Controller
	update              *update.Controller
	shutdown            *shutdown.Coordinator
	openURL             func(string) error
	openProductOnLaunch bool

	mu       sync.Mutex
	settings application.Window
	app      *application.App
	bootOnce sync.Once
	openOnce sync.Once
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
		runtime: options.Runtime, runtimeConfig: options.RuntimeConfig, lumen: options.Lumen, storage: options.Storage, update: options.Update, openURL: options.OpenURL,
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

func (h *Host) Services() []application.Service {
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
	return []application.Service{
		application.NewService(&control.DesktopService{Store: h.store, Host: h}),
		application.NewService(&control.RuntimeService{Controller: runtimeController, Config: h.runtimeConfig}),
		application.NewService(&control.StorageService{Adapter: storageAdapter}),
		application.NewService(&control.LumenService{Controller: lumenAdapter}),
		application.NewService(&control.UpdateService{Store: h.store, Adapter: updateAdapter}),
		application.NewService(&control.DiagnosticsService{Store: h.store}),
	}
}

func (h *Host) AttachApplication(app *application.App) {
	h.mu.Lock()
	h.app = app
	h.mu.Unlock()
}

func (h *Host) AttachSettingsWindow(window application.Window) {
	h.mu.Lock()
	h.settings = window
	h.mu.Unlock()
	if window != nil {
		window.Hide()
	}
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

func (h *Host) ShowSettings(route string) error {
	if !validRoute(route) {
		return operation.NewError(dto.ErrorInvalidArgument, "unknown Settings route")
	}
	snapshot := h.store.Get()
	// Recovery and onboarding own the navigation: any entry point (tray menu,
	// second-instance args, frontend links) must not be able to leave them
	// before the user resolves recovery or completes onboarding — otherwise
	// the app shows a Not Configured state with no path back to the wizard.
	if route != "/recovery" && route != "/onboarding" {
		if snapshot.Host.Recovery.Code != "" {
			route = "/recovery"
		} else if !snapshot.Runtime.Configured {
			route = "/onboarding"
		}
	}
	h.commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Host.SettingsNavigation.Sequence++
		snapshot.Host.SettingsNavigation.Route = route
		snapshot.Host.SettingsVisible = true
	})
	h.mu.Lock()
	window := h.settings
	h.mu.Unlock()
	if window != nil {
		window.Show()
		window.Focus()
	}
	return nil
}

func (h *Host) HideSettings() {
	h.commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Host.SettingsVisible = false })
	h.mu.Lock()
	window := h.settings
	h.mu.Unlock()
	if window != nil {
		window.Hide()
	}
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

func (h *Host) ResumeAfterFailedShutdown(requestID string, expectedVersion uint64) (dto.OperationReceipt, error) {
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
			_ = h.ShowSettings("/general")
		case "open-product":
			_ = h.OpenProduct()
		default:
			_ = h.ShowSettings("/general")
		}
		return
	}
	_ = h.ShowSettings("/general")
}

func (h *Host) EmitSnapshotChanges(ctx context.Context) {
	app := h.application()
	if app == nil {
		return
	}
	ch, cancel := h.store.Subscribe(1)
	go func() {
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			case notice, ok := <-ch:
				if !ok {
					return
				}
				app.Event.Emit(dto.SnapshotChangedEvent, notice)
			}
		}
	}()
}

func (h *Host) application() *application.App {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.app
}

func (h *Host) commit(reducer func(*dto.DesktopSnapshot)) {
	h.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		reducer(snapshot)
		*snapshot = control.ProjectCapabilities(*snapshot)
	})
}

func validRoute(route string) bool {
	switch route {
	case "/onboarding", "/general", "/server", "/storage", "/lumen", "/updates", "/diagnostics", "/recovery", "/overview", "/runtime", "/settings":
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
