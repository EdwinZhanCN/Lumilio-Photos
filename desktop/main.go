package main

import (
	"context"
	"embed"
	"errors"
	"io/fs"
	"log"
	"os"
	"path/filepath"

	"desktop/internal/buildinfo"
	"desktop/internal/control/dto"
	"desktop/internal/host"
	"desktop/internal/lumen"
	"desktop/internal/operation"
	"desktop/internal/panel"
	"desktop/internal/platform"
	"desktop/internal/preferences"
	"desktop/internal/resources"
	"desktop/internal/runtime"
	"desktop/internal/runtime/runtimeconfig"
	"desktop/internal/state"
	"desktop/internal/storage"
	"desktop/internal/update"
)

//go:embed resources/manifest.json all:resources/payload
var packagedResources embed.FS

// The tray icon is a black-outline template glyph: macOS tints it for the menu
// bar, and Windows shows it white on a dark taskbar. It is baked into the
// binary rather than staged, because the tray is not a payload item.
//
//go:embed build/tray.png
var trayIcon []byte

func main() {
	if handled, err := lumen.RunSupervisorMode(os.Args[1:]); handled {
		if err != nil {
			log.Printf("Lumen supervisor exited with error: %v", err)
			os.Exit(1)
		}
		return
	}
	paths, err := platform.ResolvePaths()
	if err != nil {
		log.Fatalf("resolve Desktop app-data: %v", err)
	}
	if err := paths.Ensure(); err != nil {
		log.Fatalf("prepare Desktop app-data: %v", err)
	}
	if err := platform.ConfigureBundledVipsModules(); err != nil {
		log.Printf("configure bundled libvips modules: %v", err)
	}
	if err := platform.ClearBundledResourcesQuarantine(); err != nil {
		log.Printf("bundled media resources quarantine cleanup (non-fatal): %v", err)
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
	lumenCacheDir := settings.LumenCacheDir
	if lumenCacheDir == "" {
		lumenCacheDir = paths.LumenModels
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
	// An open Apply journal blocks the start only when the current pointer
	// names a candidate that was never proven ready; a journal left at a
	// phase where the previous generation, or a candidate the journal
	// recorded as ready, is current starts normally and is closed by the
	// readiness promotion (OnReady below).
	configRecovery := configLoadRecovery || configReconcileErr != nil || configReconcile.NeedsRollback ||
		(configReconcile.NeedsResume && !configReconcile.SafeToStart)
	if configReconcileErr != nil {
		log.Printf("Desktop runtime journal requires recovery: %v", configReconcileErr)
	} else if configReconcile.NeedsResume && configReconcile.SafeToStart {
		log.Printf("Desktop runtime journal left at %q resumes through the readiness promotion", configReconcile.Journal.Phase)
	}
	configured = configured && resourceErr == nil && !configRecovery
	lumenInstalled := false
	lumenVersion := ""
	lumenProfile := ""
	if profile, available := lumen.CurrentReleaseProfile(); available {
		lumenProfile = profile
	}
	lumenRecovery := false
	if reconcileErr := lumen.ReconcileOfficialReleaseInstall(paths.LumenDir); reconcileErr != nil {
		log.Printf("Desktop Lumen installation journal requires recovery: %v", reconcileErr)
		lumenRecovery = true
	}
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
	var lumenInstaller lumen.Installer
	var lumenFactory lumen.Factory
	if _, available := lumen.CurrentReleaseProfile(); available {
		lumenInstaller = lumen.NewOfficialReleaseInstaller(paths.LumenDir)
		lumenFactory = lumen.CurrentFactory{
			Root: paths.LumenDir, ConfigPath: paths.LumenConfig,
			OwnerLock: paths.LumenOwner, Endpoint: lumen.DefaultEndpoint,
			Prepare: func(ctx context.Context, _ string, hubBinary string) error {
				currentSettings, err := runtimeconfig.LoadSettings(paths.SettingsFile)
				if err != nil {
					return err
				}
				cacheDir := currentSettings.LumenCacheDir
				if cacheDir == "" {
					cacheDir = paths.LumenModels
				}
				intent, err := lumen.NewSetupIntent(paths.LumenConfig, cacheDir, currentSettings.Region, currentSettings.LumenPreset)
				if err != nil {
					return err
				}
				return lumen.ReconcileSetupConfig(ctx, hubBinary, intent)
			},
		}
	}
	lumenController := lumen.NewController(lumen.Options{
		Store: store, Operations: operations,
		Desired:    runtimeconfig.NewLumenDesiredStateStore(paths.SettingsFile),
		SetupStore: runtimeconfig.NewLumenSetupStore(paths.SettingsFile),
		ValidateSetup: func(ctx context.Context, preset, cacheDir string) error {
			currentSettings, err := runtimeconfig.LoadSettings(paths.SettingsFile)
			if err != nil {
				return err
			}
			return lumen.ValidateInstalledSetup(ctx, paths.LumenDir, paths.LumenConfig, cacheDir, currentSettings.Region, preset)
		},
		Installer: lumenInstaller, Factory: lumenFactory,
		Installed: lumenInstalled, InstalledVer: lumenVersion, Profile: lumenProfile,
		Preset: settings.LumenPreset, CacheDir: lumenCacheDir,
		Profiles: lumen.CurrentReleaseProfiles(), Presets: lumen.SetupPresetNames(),
		Endpoint: lumen.DefaultEndpoint,
	})
	storageController := storage.NewController(storage.Options{
		Paths: paths, Runtime: runtimeController, Store: store,
	})
	updateController := update.NewController(update.Options{
		Store: store, Operations: operations,
		Channel: settings.UpdateChannel, CurrentVersion: buildinfo.Version,
	})
	preferencesController := preferences.NewController(preferences.Options{
		Path: paths.SettingsFile, Settings: settings, Store: store, Updates: updateController,
	})

	desktopHost := host.New(host.Options{
		Store: store, Operations: operations, Runtime: runtimeController, RuntimeConfig: configController,
		Lumen: lumenController, Storage: storageController, Update: updateController,
		Preferences:         preferencesController,
		OpenRuntimeManifest: func() error { return host.OpenFolder(paths.RuntimeIntents) },
		OpenProductOnLaunch: settings.OpenProductOnLaunch,
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
	showOnLaunch := ""
	switch {
	case settingsRecovery || resourceErr != nil || configRecovery || lumenRecovery:
		showOnLaunch = panel.RouteRecovery
	case !configured:
		showOnLaunch = panel.RouteOnboarding
	}
	// The framework keeps its own small state (the window's place, the chosen
	// language) apart from the files the Desktop controllers own.
	frameworkData := filepath.Join(paths.Root, "host")
	if err := platform.EnsurePrivateDirectory(frameworkData); err != nil {
		log.Printf("prepare host state directory: %v", err)
		frameworkData = ""
	}
	if err := desktopHost.Run(host.RunOptions{
		TrayIcon: trayIcon, FrameworkData: frameworkData, ShowOnLaunch: showOnLaunch,
	}); err != nil {
		log.Printf("Desktop application exited with error: %v", err)
		os.Exit(1)
	}
}
