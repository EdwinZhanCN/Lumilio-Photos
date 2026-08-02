// Package preferences owns the small set of Desktop host preferences stored in
// settings.v1.json. It never mutates the Server manifest.
package preferences

import (
	"strings"
	"sync"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/runtime/runtimeconfig"
	"desktop/internal/state"
)

type UpdateChannelController interface {
	SetChannel(string) error
}

type Options struct {
	Path     string
	Settings runtimeconfig.Settings
	Store    *state.Store
	Updates  UpdateChannelController
}

type Controller struct {
	path    string
	store   *state.Store
	updates UpdateChannelController
	mu      sync.Mutex
}

func NewController(options Options) *Controller {
	if options.Store == nil {
		options.Store = state.New()
	}
	controller := &Controller{path: options.Path, store: options.Store, updates: options.Updates}
	controller.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		version := snapshot.Host.Preferences.Version
		if version == 0 {
			version = 1
		}
		snapshot.Host.Preferences = project(options.Settings, version)
	})
	return controller
}

func (c *Controller) Save(candidate dto.DesktopPreferences) (dto.DesktopPreferences, error) {
	if c == nil || c.store == nil || strings.TrimSpace(c.path) == "" {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorRuntimeNotReady, "Desktop preferences are unavailable")
	}
	c.mu.Lock()
	defer c.mu.Unlock()

	current := c.store.Get().Host.Preferences
	if candidate.Version != 0 && candidate.Version != current.Version {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorStaleVersion, "Desktop preferences changed; reload and try again")
	}
	locale := strings.TrimSpace(candidate.Locale)
	region := strings.TrimSpace(candidate.Region)
	channel := strings.TrimSpace(candidate.UpdateChannel)
	theme := strings.TrimSpace(candidate.Theme)
	if locale == "" {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorInvalidArgument, "Desktop language is required")
	}
	if region == "" {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorInvalidArgument, "Desktop region is required")
	}
	if channel != "stable" && channel != "beta" {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorInvalidArgument, "update channel must be stable or beta")
	}
	if theme != "light" && theme != "dark" && theme != "system" {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorInvalidArgument, "theme must be light, dark, or system")
	}

	settings, err := runtimeconfig.LoadSettings(c.path)
	if err != nil {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorRecoveryRequired, "Desktop preferences could not be loaded")
	}
	settings.Locale = locale
	settings.Region = region
	settings.UpdateChannel = channel
	settings.Theme = theme
	settings.OpenProductOnLaunch = candidate.OpenProductOnLaunch
	if err := runtimeconfig.SaveSettings(c.path, settings); err != nil {
		return dto.DesktopPreferences{}, operation.NewError(dto.ErrorRecoveryRequired, "Desktop preferences could not be saved")
	}
	if c.updates != nil {
		if err := c.updates.SetChannel(channel); err != nil {
			return dto.DesktopPreferences{}, err
		}
	}

	saved := project(settings, current.Version+1)
	c.store.Commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Host.Preferences = saved
	})
	return saved, nil
}

func project(settings runtimeconfig.Settings, version uint64) dto.DesktopPreferences {
	return dto.DesktopPreferences{
		Version:             version,
		Locale:              settings.Locale,
		Region:              settings.Region,
		UpdateChannel:       settings.UpdateChannel,
		Theme:               settings.Theme,
		OpenProductOnLaunch: settings.OpenProductOnLaunch,
	}
}
