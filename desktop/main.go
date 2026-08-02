package main

import (
	"embed"
	"errors"
	"io/fs"
	"log"
	"net/http"
	"os"

	"desktop/internal/buildinfo"
	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/host"
	"desktop/internal/lumen"
	"desktop/internal/operation"
	"desktop/internal/platform"
	"desktop/internal/preferences"
	"desktop/internal/resources"
	"desktop/internal/runtime"
	"desktop/internal/runtime/runtimeconfig"
	"desktop/internal/state"
	"desktop/internal/storage"
	"desktop/internal/update"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// The source tree is embedded as a compile-safe fallback for `go test` before
// the Vite build has produced frontend/dist. Wails production builds select the
// generated dist subtree when it is present.
//
//go:embed frontend
var frontendAssets embed.FS

//go:embed resources/manifest.json all:resources/payload
var packagedResources embed.FS

// System tray icons: tray.png is the black-outline template glyph (macOS
// template + Windows light mode), tray-dark.png is the white outline for
// Windows dark mode. Kept out of packagedResources because the tray is not a
// staged payload item — it is baked into the binary.
//
//go:embed build/tray.png
var trayIcon []byte

//go:embed build/tray-dark.png
var trayIconDark []byte

const applicationID = "com.lumilio.photos.desktop"

func main() {
	paths, err := platform.ResolvePaths()
	if err != nil {
		log.Fatalf("resolve Desktop app-data: %v", err)
	}
	if err := paths.Ensure(); err != nil {
		log.Fatalf("prepare Desktop app-data: %v", err)
	}
	resourceData, resourceErr := fs.ReadFile(packagedResources, "resources/manifest.json")
	if resourceErr == nil {
		var resourceManifest resources.Manifest
		resourceManifest, resourceErr = resources.Load(resourceData)
		if resourceErr == nil {
			resourceErr = resources.NewManager(paths, packagedResources, "resources/payload", resourceManifest).Ensure()
		}
	}
	if resourceErr != nil {
		log.Printf("packaged Desktop resources require recovery: %v", resourceErr)
	}

	settings, settingsErr := runtimeconfig.LoadSettings(paths.SettingsFile)
	settingsRecovery := settingsErr != nil
	if settingsErr != nil {
		log.Printf("Desktop settings require recovery: %v", settingsErr)
		// Keep a safe in-memory value for non-mutating UI projection, but never
		// write it back or treat a corrupt settings file as onboarding success.
		settings = runtimeconfig.DefaultSettings()
	}
	configStore := runtimeconfig.NewStore(paths)
	configured := false
	configLoadRecovery := false
	currentPointer, pointerErr := configStore.CurrentPointer()
	if pointerErr != nil {
		configLoadRecovery = true
		log.Printf("Desktop runtime pointer is not ready: %v", pointerErr)
	} else if currentPointer.Fingerprint != "" {
		if _, err := configStore.LoadCurrentConfig(); err == nil {
			configured = settings.OnboardingComplete && !settingsRecovery
		} else {
			configLoadRecovery = true
			log.Printf("Desktop runtime configuration is not ready: %v", err)
		}
	}
	if settings.OnboardingComplete && currentPointer.Fingerprint == "" {
		configLoadRecovery = true
	}
	configReconcile, configReconcileErr := configStore.Reconcile()
	configRecovery := configLoadRecovery || configReconcileErr != nil || configReconcile.NeedsResume || configReconcile.NeedsRollback
	if configReconcileErr != nil {
		log.Printf("Desktop runtime journal requires recovery: %v", configReconcileErr)
	}
	configured = configured && resourceErr == nil && !configRecovery
	lumenInstalled := false
	lumenVersion := ""
	lumenProfile := ""
	lumenRecovery := false
	if current, currentErr := lumen.LoadCurrent(paths.LumenDir); currentErr == nil {
		lumenInstalled = true
		lumenVersion = current.Version
		lumenProfile = current.Profile
	} else if !errors.Is(currentErr, os.ErrNotExist) {
		log.Printf("Desktop Lumen installation requires recovery: %v", currentErr)
		lumenRecovery = true
	}

	store := state.New()
	operations := operation.New()
	serverFactory := runtime.ServerFactory{Load: configStore.LoadCurrentConfig}
	runtimeController := runtime.NewController(runtime.Options{
		Store: store, Operations: operations,
		Desired: runtimeconfig.NewDesiredStateStore(paths.SettingsFile),
		Factory: serverFactory, Configured: configured && !configRecovery,
		OnReady: func() error {
			if err := configStore.PromoteCurrentToLastKnownGood(); err != nil {
				return err
			}
			store.Commit(func(snapshot *dto.DesktopSnapshot) {
				snapshot.Runtime.PendingConfigValidation = false
			})
			return nil
		},
	})
	configController := runtimeconfig.NewTransactionController(configStore, store, operations, runtimeController)
	lumenController := lumen.NewController(lumen.Options{
		Store: store, Operations: operations,
		Desired:   runtimeconfig.NewLumenDesiredStateStore(paths.SettingsFile),
		Installed: lumenInstalled, InstalledVer: lumenVersion, Profile: lumenProfile,
	})
	storageController := storage.NewController(storage.Options{
		Paths: paths, Runtime: runtimeController, Store: store,
	})
	updateController := update.NewController(update.Options{
		Store: store, Operations: operations, StagingDir: paths.UpdatesStaging,
		Channel: settings.UpdateChannel, CurrentVersion: buildinfo.Version,
	})
	preferencesController := preferences.NewController(preferences.Options{
		Path: paths.SettingsFile, Settings: settings, Store: store, Updates: updateController,
	})

	desktopHost := host.New(host.Options{
		Store: store, Operations: operations, Runtime: runtimeController, RuntimeConfig: configController, Lumen: lumenController, Storage: storageController, Update: updateController, OpenProductOnLaunch: settings.OpenProductOnLaunch,
	})
	updateController.SetApply(func(requestID string, _ uint64) (dto.OperationReceipt, error) {
		return desktopHost.RequestQuit("update-"+requestID, store.Get().Revision)
	})
	if resourceErr != nil {
		store.Commit(func(snapshot *dto.DesktopSnapshot) {
			snapshot.Host.Recovery = dto.Error{Code: dto.ErrorRecoveryRequired, Message: "packaged resources require recovery"}
		})
	}
	if settingsRecovery {
		store.Commit(func(snapshot *dto.DesktopSnapshot) {
			snapshot.Host.Recovery = dto.Error{Code: dto.ErrorRecoveryRequired, Message: "Desktop settings require recovery"}
		})
	}
	if configRecovery {
		store.Commit(func(snapshot *dto.DesktopSnapshot) {
			snapshot.Host.Recovery = dto.Error{Code: dto.ErrorRecoveryRequired, Message: "runtime configuration journal requires recovery"}
			snapshot.Runtime.PendingConfigValidation = true
			snapshot.Runtime.RecoveryCause = dto.ErrorRecoveryRequired
		})
	}
	if lumenRecovery {
		store.Commit(func(snapshot *dto.DesktopSnapshot) {
			snapshot.Host.Recovery = dto.Error{Code: dto.ErrorRecoveryRequired, Message: "Lumen installation requires recovery"}
		})
	}
	desktopService := &control.DesktopService{Store: store, Host: desktopHost, Preferences: preferencesController}
	runtimeService := &control.RuntimeService{Controller: runtimeController, Config: configController}
	storageService := &control.StorageService{Adapter: storageController}
	lumenService := &control.LumenService{Controller: lumenController}
	updateService := &control.UpdateService{Store: store, Adapter: updateController}
	diagnosticsService := &control.DiagnosticsService{Store: store}

	app := application.New(application.Options{
		Name:        "Lumilio Photos",
		Description: "Local-first photo management",
		Assets: application.AssetOptions{
			Handler: frontendHandler(frontendAssets),
		},
		Services: []application.Service{
			application.NewService(desktopService),
			application.NewService(runtimeService),
			application.NewService(storageService),
			application.NewService(lumenService),
			application.NewService(updateService),
			application.NewService(diagnosticsService),
		},
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: applicationID,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				desktopHost.HandleSecondInstance(data.Args)
			},
		},
		Mac: application.MacOptions{
			ActivationPolicy: application.ActivationPolicyAccessory,
			ApplicationShouldTerminateAfterLastWindowClosed: false,
		},
		Windows: application.WindowsOptions{
			DisableQuitOnLastWindowClosed: true,
		},
		ShouldQuit: desktopHost.ShouldQuit,
		OnShutdown: desktopHost.Close,
	})
	desktopHost.AttachApplication(app)
	desktopHost.SetOpenURL(app.Browser.OpenURL)
	desktopService.OpenRuntimeManifestAction = func() error {
		return app.Env.OpenFileManager(paths.RuntimeIntents, false)
	}
	storageController.SetOpenFileManager(app.Env.OpenFileManager)
	storageController.SetPickDirectory(func(title string) (string, error) {
		dialog := app.Dialog.OpenFile().
			CanChooseDirectories(true).
			CanChooseFiles(false).
			CanCreateDirectories(true).
			SetTitle(title)
		return dialog.PromptForSingleSelection()
	})
	desktopHost.SetQuitArmed(app.Quit)

	settingsWindow := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Name:      "settings",
		Title:     "Lumilio Photos Settings",
		Width:     1080,
		Height:    760,
		MinWidth:  760,
		MinHeight: 520,
		Hidden:    true,
		URL:       "/",
	})
	settingsWindow.RegisterHook(events.Common.WindowClosing, func(event *application.WindowEvent) {
		event.Cancel()
		desktopHost.HideSettings()
	})
	desktopHost.AttachSettingsWindow(settingsWindow)
	_ = host.NewTray(app, desktopHost, trayIcon, trayIconDark)

	app.Event.OnApplicationEvent(events.Common.ApplicationStarted, func(*application.ApplicationEvent) {
		desktopHost.Boot(app.Context())
		desktopHost.EmitSnapshotChanges(app.Context())
		if settingsRecovery || resourceErr != nil || configRecovery || lumenRecovery {
			_ = desktopHost.ShowSettings("/recovery")
		} else if !configured {
			_ = desktopHost.ShowSettings("/onboarding")
		}
	})

	if err := app.Run(); err != nil {
		log.Printf("Desktop application exited with error: %v", err)
		os.Exit(1)
	}
}

func frontendHandler(assets embed.FS) http.Handler {
	root, err := fs.Sub(assets, "frontend")
	if err != nil {
		return http.NotFoundHandler()
	}
	if dist, err := fs.Sub(assets, "frontend/dist"); err == nil {
		if _, err := fs.Stat(dist, "index.html"); err == nil {
			root = dist
		}
	}
	return http.FileServer(http.FS(root))
}
