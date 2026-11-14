package host

import (
	"errors"
	"fmt"
	"testing"

	"desktop/internal/control/dto"
	"desktop/internal/operation"
	"desktop/internal/panel"
	"desktop/internal/state"
	"desktop/internal/update"
)

type fakeWindow struct {
	shown  int
	hidden int
}

func (w *fakeWindow) Show() { w.shown++ }
func (w *fakeWindow) Hide() { w.hidden++ }

func newTestHost(t *testing.T, prepare func(*dto.DesktopSnapshot)) (*Host, *fakeWindow) {
	t.Helper()
	store := state.NewWithInstanceID("host-test")
	operations := operation.New()
	t.Cleanup(func() {
		operations.Close()
		store.Close()
	})
	if prepare != nil {
		store.Commit(prepare)
	}
	host := New(Options{Store: store, Operations: operations})
	window := &fakeWindow{}
	host.attachWindow(window)
	return host, window
}

func configured(snapshot *dto.DesktopSnapshot) { snapshot.Runtime.Configured = true }

// Every way into Settings (the tray, a second launch, the window's own
// sidebar) goes through this guard. If one could leave the wizard early, the
// app would show a runtime with no configuration and no path back.
func TestShowSettingsKeepsAnUnconfiguredRuntimeInTheWizard(t *testing.T) {
	host, window := newTestHost(t, nil)
	for _, route := range []string{panel.RouteGeneral, panel.RouteStorage, panel.RouteUpdates} {
		if err := host.ShowSettings(route); err != nil {
			t.Fatalf("ShowSettings(%q): %v", route, err)
		}
		if got := host.Store().Get().Host.SettingsNavigation.Route; got != panel.RouteOnboarding {
			t.Fatalf("ShowSettings(%q) navigated to %q, want the wizard", route, got)
		}
	}
	if window.shown != 3 {
		t.Fatalf("the window was shown %d times, want once per request", window.shown)
	}
}

func TestShowSettingsKeepsRecoveryUntilItIsResolved(t *testing.T) {
	host, _ := newTestHost(t, func(snapshot *dto.DesktopSnapshot) {
		configured(snapshot)
		snapshot.Host.Recovery = dto.Error{Code: dto.ErrorRecoveryRequired, Message: "settings require recovery"}
	})
	if err := host.ShowSettings(panel.RouteLumen); err != nil {
		t.Fatal(err)
	}
	if got := host.Store().Get().Host.SettingsNavigation.Route; got != panel.RouteRecovery {
		t.Fatalf("navigated to %q during recovery, want the recovery page", got)
	}
	// The wizard stays reachable: it is how a fresh configuration is made.
	if err := host.ShowSettings(panel.RouteOnboarding); err != nil {
		t.Fatal(err)
	}
	if got := host.Store().Get().Host.SettingsNavigation.Route; got != panel.RouteOnboarding {
		t.Fatalf("navigated to %q, want the wizard to stay reachable", got)
	}
}

func TestShowSettingsFollowsTheRequestOnceConfigured(t *testing.T) {
	host, window := newTestHost(t, configured)
	before := host.Store().Get().Host.SettingsNavigation.Sequence
	if err := host.ShowSettings(panel.RouteStorage); err != nil {
		t.Fatal(err)
	}
	snapshot := host.Store().Get()
	if snapshot.Host.SettingsNavigation.Route != panel.RouteStorage || !snapshot.Host.SettingsVisible {
		t.Fatalf("navigation = %#v, visible = %v; want the Storage page shown", snapshot.Host.SettingsNavigation, snapshot.Host.SettingsVisible)
	}
	// The sequence is what tells the window a repeated request for the same
	// route apart from no request.
	if snapshot.Host.SettingsNavigation.Sequence != before+1 {
		t.Fatalf("sequence = %d, want %d", snapshot.Host.SettingsNavigation.Sequence, before+1)
	}
	host.HideSettings()
	if host.Store().Get().Host.SettingsVisible || window.hidden != 1 {
		t.Fatal("hiding Settings did not close the window and record it")
	}
}

func TestShowSettingsRefusesAnUnknownRoute(t *testing.T) {
	host, window := newTestHost(t, configured)
	err := host.ShowSettings("/etc/passwd")
	if operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("error = %v, want invalid_argument", err)
	}
	if window.shown != 0 {
		t.Fatal("an unknown route opened the window")
	}
}

func TestSecondInstanceBringsSettingsForward(t *testing.T) {
	host, window := newTestHost(t, configured)
	host.HandleSecondInstance(nil)
	host.HandleSecondInstance([]string{"show-settings"})
	if window.shown != 2 {
		t.Fatalf("the window was shown %d times, want 2", window.shown)
	}
	if got := host.Store().Get().Host.SettingsNavigation.Route; got != panel.RouteGeneral {
		t.Fatalf("a second launch navigated to %q, want General", got)
	}
}

func TestOpenProductNeedsARunningServerAndAWebURL(t *testing.T) {
	var opened []string
	host, _ := newTestHost(t, configured)
	host.SetOpenURL(func(url string) error {
		opened = append(opened, url)
		return nil
	})
	if err := host.OpenProduct(); operation.ErrorCodeOf(err) != dto.ErrorRuntimeNotReady {
		t.Fatalf("error = %v, want runtime_not_ready while the Server is stopped", err)
	}
	host.commit(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Runtime.Phase = dto.RuntimeRunning
		snapshot.Runtime.ProductURL = "file:///etc/passwd"
	})
	if err := host.OpenProduct(); operation.ErrorCodeOf(err) != dto.ErrorInvalidArgument {
		t.Fatalf("error = %v, want a non-web URL refused", err)
	}
	host.commit(func(snapshot *dto.DesktopSnapshot) { snapshot.Runtime.ProductURL = "http://localhost:6680" })
	if err := host.OpenProduct(); err != nil {
		t.Fatal(err)
	}
	if len(opened) != 1 || opened[0] != "http://localhost:6680" {
		t.Fatalf("opened = %q, want the product URL once", opened)
	}
}

func TestLocaleTagNamesTheCatalogOfAPreference(t *testing.T) {
	for preference, want := range map[string]string{
		"en": "en", "zh-CN": "zh-Hans", "zh-Hans": "zh-Hans", "": "en", "fr": "en",
	} {
		if got := localeTag(preference); got != want {
			t.Errorf("localeTag(%q) = %q, want %q", preference, got, want)
		}
	}
}

// The updater has no typed error for an artifact signed with another key, so
// its message is matched. Only that refusal may be reported as a signature
// failure; a dropped connection is retried, not called tampering.
func TestOnlyTheUpdatersKeyRefusalIsASignatureFailure(t *testing.T) {
	refused := classifyUpdateError(fmt.Errorf("mygo: installing: %w", errors.New("the update is not signed with the app's key")))
	if !errors.Is(refused, update.ErrSignature) {
		t.Fatalf("%v is not a signature failure", refused)
	}
	for _, err := range []error{nil, errors.New("connection reset by peer"), errors.New("mygo: updates are not enabled for this build")} {
		if errors.Is(classifyUpdateError(err), update.ErrSignature) {
			t.Fatalf("%v was classified as a signature failure", err)
		}
	}
}
