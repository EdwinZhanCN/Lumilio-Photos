package panel

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"slices"
	"testing"
	"time"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/locales"
	"desktop/internal/operation"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// harness drives a Panel as its window would: every control-plane call runs
// inline against fakes, and the view is built by a tester without a window.
type harness struct {
	t        *testing.T
	panel    *Panel
	tt       *ui.Tester
	snapshot dto.DesktopSnapshot

	config      *fakeConfig
	runtime     *fakeRuntime
	host        *fakeHost
	preferences *fakePreferences
	storage     *fakeStorage
	applied     []dto.DesktopPreferences
}

func newHarness(t *testing.T, configure func(*dto.DesktopSnapshot, *fakeConfig)) *harness {
	t.Helper()
	i18n.SetLocaleFunc(func() string { return "en" })
	h := &harness{t: t, snapshot: dto.InitialSnapshot("test-instance")}
	h.snapshot.Runtime.Configured = true
	h.snapshot.Runtime.Phase = dto.RuntimeRunning
	h.snapshot.Runtime.Ownership = dto.OwnershipHeld
	h.snapshot.Runtime.DesiredState = dto.DesiredRunning
	h.snapshot.Runtime.ProductURL = "http://localhost:6680"
	h.snapshot.Host.SettingsNavigation = dto.NavigationIntent{Sequence: 1, Route: RouteGeneral}
	h.config = &fakeConfig{draft: dto.RuntimeConfigDraft{
		TOML: "candidate-0", BaseFingerprint: "base-1", Source: "current",
		Settings: dto.RuntimeConfigSettings{
			NetworkMode: "local", Listen: "127.0.0.1:6680", StoragePath: "/photos",
			LoggingLevel: "info", HardwareAcceleration: "auto",
		},
		Validation: dto.ConfigValidation{Valid: true},
	}}
	h.runtime = &fakeRuntime{harness: h}
	h.host = &fakeHost{harness: h}
	h.preferences = &fakePreferences{harness: h}
	h.storage = &fakeStorage{}
	if configure != nil {
		configure(&h.snapshot, h.config)
	}
	h.panel = New(Options{
		Services: Services{
			Desktop: &control.DesktopService{Host: h.host, Preferences: h.preferences},
			Runtime: &control.RuntimeService{Controller: h.runtime, Config: h.config},
			Storage: &control.StorageService{Adapter: h.storage},
			Lumen:   &control.LumenService{},
			Update:  &control.UpdateService{},
		},
		ApplyPreferences: func(preferences dto.DesktopPreferences) { h.applied = append(h.applied, preferences) },
		Now: func() time.Time {
			if h.tt == nil {
				return time.Unix(1_800_000_000, 0)
			}
			return h.tt.Now()
		},
	})
	h.tt = ui.NewTester(h.panel.View, 940, 680)
	h.panel.SetSnapshot(h.snapshot)
	h.tt.Frame()
	return h
}

func (h *harness) update(mutate func(*dto.DesktopSnapshot)) {
	h.t.Helper()
	mutate(&h.snapshot)
	h.snapshot.Revision++
	h.panel.SetSnapshot(h.snapshot)
	h.tt.Frame()
}

func (h *harness) finish(operationID string, state operation.State) {
	h.t.Helper()
	h.update(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Operations = append(snapshot.Operations, dto.OperationSnapshot{OperationID: operationID, State: string(state)})
	})
}

func (h *harness) click(text string) {
	h.t.Helper()
	if err := h.tt.Click(text); err != nil {
		h.t.Fatalf("click %q: %v\nshowing: %q", text, err, h.tt.Texts())
	}
}

func (h *harness) want(texts ...string) {
	h.t.Helper()
	for _, text := range texts {
		if !h.tt.HasText(text) {
			h.t.Fatalf("missing %q\nshowing: %q", text, h.tt.Texts())
		}
	}
}

func (h *harness) wantNot(texts ...string) {
	h.t.Helper()
	for _, text := range texts {
		if h.tt.HasText(text) {
			h.t.Fatalf("unexpected %q\nshowing: %q", text, h.tt.Texts())
		}
	}
}

type fakeConfig struct {
	draft   dto.RuntimeConfigDraft
	reads   int
	patches []dto.RuntimeConfigSettings
	saves   []string
	applies []string
	bases   []string
}

func (f *fakeConfig) ReadDraft() (dto.RuntimeConfigDraft, error) {
	f.reads++
	return f.draft, nil
}

func (f *fakeConfig) PatchDraft(candidate string, settings dto.RuntimeConfigSettings) (dto.RuntimeConfigDraft, error) {
	f.patches = append(f.patches, settings)
	patched := f.draft
	patched.TOML = fmt.Sprintf("candidate-%d", len(f.patches))
	patched.Settings = settings
	patched.BaseFingerprint = "patched-base"
	return patched, nil
}

func (f *fakeConfig) Validate(string) (dto.ConfigValidation, error) {
	return dto.ConfigValidation{Valid: true}, nil
}

func (f *fakeConfig) Save(_ string, _ uint64, base, candidate string) (dto.OperationReceipt, error) {
	f.saves, f.bases = append(f.saves, candidate), append(f.bases, base)
	return dto.OperationReceipt{OperationID: "config-op"}, nil
}

func (f *fakeConfig) Apply(_ string, _ uint64, base, candidate string) (dto.OperationReceipt, error) {
	f.applies, f.bases = append(f.applies, candidate), append(f.bases, base)
	return dto.OperationReceipt{OperationID: "config-op"}, nil
}

func (f *fakeConfig) RestoreLastKnownGood(string, uint64) (dto.OperationReceipt, error) {
	return dto.OperationReceipt{OperationID: "restore-op"}, nil
}

type fakeRuntime struct {
	harness *harness
	actions []string
	err     error
}

func (f *fakeRuntime) Snapshot() dto.DesktopSnapshot { return f.harness.snapshot }

func (f *fakeRuntime) act(action string) (dto.OperationReceipt, error) {
	f.actions = append(f.actions, action)
	return dto.OperationReceipt{OperationID: "runtime-op"}, f.err
}

func (f *fakeRuntime) Start(string, uint64) (dto.OperationReceipt, error)   { return f.act("start") }
func (f *fakeRuntime) Stop(string, uint64) (dto.OperationReceipt, error)    { return f.act("stop") }
func (f *fakeRuntime) Restart(string, uint64) (dto.OperationReceipt, error) { return f.act("restart") }
func (f *fakeRuntime) RetryCleanup(string, uint64) (dto.OperationReceipt, error) {
	return f.act("retry")
}

// fakeHost answers navigation as the real host does, through the snapshot's
// navigation intent, without its recovery and onboarding guard.
type fakeHost struct {
	harness *harness
	routes  []string
	opened  int
}

func (f *fakeHost) ShowSettings(route string) error {
	f.routes = append(f.routes, route)
	navigation := &f.harness.snapshot.Host.SettingsNavigation
	navigation.Sequence++
	navigation.Route = route
	f.harness.panel.SetSnapshot(f.harness.snapshot)
	return nil
}

func (f *fakeHost) OpenProduct() error { f.opened++; return nil }

func (f *fakeHost) RequestQuit(string, uint64) (dto.OperationReceipt, error) {
	return dto.OperationReceipt{}, nil
}

func (f *fakeHost) ResumeAfterFailedShutdown(string, uint64) (dto.OperationReceipt, error) {
	return dto.OperationReceipt{}, nil
}

func (f *fakeHost) ForceQuit(string, uint64, string) (dto.OperationReceipt, error) {
	return dto.OperationReceipt{}, nil
}

type fakePreferences struct {
	harness *harness
	saved   []dto.DesktopPreferences
}

func (f *fakePreferences) Save(candidate dto.DesktopPreferences) (dto.DesktopPreferences, error) {
	candidate.Version++
	f.saved = append(f.saved, candidate)
	f.harness.snapshot.Host.Preferences = candidate
	return candidate, nil
}

type fakeStorage struct {
	shortcuts   []dto.StorageShortcut
	hostActions []dto.HostActionTicket
	actionsErr  error
	pick        string
	approveErr  error
	approved    []string
	declined    []string
}

func (f *fakeStorage) ListShortcuts(context.Context) ([]dto.StorageShortcut, error) {
	return f.shortcuts, nil
}
func (f *fakeStorage) PickLocation(string) (string, error)        { return f.pick, nil }
func (f *fakeStorage) OpenLocation(context.Context, string) error { return nil }
func (f *fakeStorage) ListHostActions(context.Context) ([]dto.HostActionTicket, error) {
	return f.hostActions, f.actionsErr
}

func (f *fakeStorage) ApproveHostAction(_ string, _ uint64, actionID, nonce string) (dto.OperationReceipt, error) {
	f.approved = append(f.approved, actionID+":"+nonce)
	return dto.OperationReceipt{OperationID: "host-action-op"}, f.approveErr
}

func (f *fakeStorage) DeclineHostAction(actionID string) error {
	f.declined = append(f.declined, actionID)
	f.hostActions = nil
	return nil
}

func TestOnboardingSavesTheFirstConfigurationAndLeavesTheWizard(t *testing.T) {
	h := newHarness(t, func(snapshot *dto.DesktopSnapshot, config *fakeConfig) {
		snapshot.Runtime = dto.RuntimeSnapshot{Phase: dto.RuntimeStopped}
		snapshot.Host.SettingsNavigation.Route = RouteOnboarding
		config.draft.Source = "default"
		config.draft.BaseFingerprint = ""
	})
	h.want(locales.OnboardingGeneralTitle(), locales.OnboardingStep(1, onboardingSteps))
	// The wizard owns the window: no sidebar, and no way to open a product
	// that does not run yet.
	h.wantNot(locales.DockUpdates(), locales.HeaderOpenProduct())

	h.click(locales.CommonContinue())
	h.want(locales.OnboardingStorageTitle(), "/photos")
	h.click(locales.CommonContinue())
	h.want(locales.OnboardingNetworkTitle())
	h.click(locales.CommonContinue())
	h.want(locales.OnboardingReadyTitle(), locales.NetworkLocal())

	h.click(locales.OnboardingFinishSetup())
	if !slices.Equal(h.config.saves, []string{"candidate-0"}) || len(h.config.applies) != 0 {
		t.Fatalf("first save: saves = %q, applies = %q; want one plain save of the proposed candidate", h.config.saves, h.config.applies)
	}
	if len(h.host.routes) != 0 {
		t.Fatalf("the wizard left before the transaction ended: %q", h.host.routes)
	}

	h.config.draft.Source = "current"
	h.finish("config-op", operation.Succeeded)
	if !slices.Equal(h.host.routes, []string{RouteGeneral}) {
		t.Fatalf("routes after setup = %q, want the wizard to hand over to General", h.host.routes)
	}
	if h.panel.Dirty() {
		t.Fatal("the draft is still dirty after the first save succeeded")
	}
}

func TestOnboardingStaysWhenTheFirstSaveFails(t *testing.T) {
	h := newHarness(t, func(snapshot *dto.DesktopSnapshot, config *fakeConfig) {
		snapshot.Runtime = dto.RuntimeSnapshot{Phase: dto.RuntimeStopped}
		snapshot.Host.SettingsNavigation.Route = RouteOnboarding
		config.draft.Source = "default"
	})
	for range onboardingSteps - 1 {
		h.click(locales.CommonContinue())
	}
	h.click(locales.OnboardingFinishSetup())
	h.update(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Operations = append(snapshot.Operations, dto.OperationSnapshot{
			OperationID: "config-op", State: string(operation.Failed),
			Error: dto.Error{Code: dto.ErrorStorageUnavailable},
		})
	})
	h.want(locales.OnboardingSetupFailed(), locales.ErrorsStorageUnavailable())
	if len(h.host.routes) != 0 {
		t.Fatalf("a failed setup left the wizard: %q", h.host.routes)
	}
}

// On the first launch the wizard's storage path is only a proposal: choosing
// another folder is a plain choice, with nothing to move and nothing to
// confirm.
func TestChoosingStorageInTheWizardNeedsNoMoveConfirmation(t *testing.T) {
	h := newHarness(t, func(snapshot *dto.DesktopSnapshot, config *fakeConfig) {
		snapshot.Runtime = dto.RuntimeSnapshot{Phase: dto.RuntimeStopped}
		snapshot.Host.SettingsNavigation.Route = RouteOnboarding
		config.draft.Source = "default"
	})
	h.storage.pick = "/chosen"
	h.click(locales.CommonContinue())
	h.click(locales.CommonChoose())
	h.wantNot(locales.StorageDefaultLocationMigrationConfirmation())
	if len(h.config.patches) != 1 || h.config.patches[0].StoragePath != "/chosen" {
		t.Fatalf("patches = %#v, want the chosen folder patched in at once", h.config.patches)
	}
	h.want("/chosen")
}

// A candidate the strict Server loader rejects must say why on the last
// step, and must not be savable.
func TestWizardNamesAnInvalidConfigurationAndDoesNotSaveIt(t *testing.T) {
	h := newHarness(t, func(snapshot *dto.DesktopSnapshot, config *fakeConfig) {
		snapshot.Runtime = dto.RuntimeSnapshot{Phase: dto.RuntimeStopped}
		snapshot.Host.SettingsNavigation.Route = RouteOnboarding
		config.draft.Source = "default"
		config.draft.Validation = dto.ConfigValidation{
			Issues: []dto.ConfigIssue{{Path: "storage.path", Message: "storage.path must be writable"}},
		}
	})
	for range onboardingSteps - 1 {
		h.click(locales.CommonContinue())
	}
	h.want(locales.OnboardingSettingsInvalid(), "storage.path must be writable")
	if err := h.tt.Click(locales.OnboardingFinishSetup()); err == nil && len(h.config.saves) != 0 {
		t.Fatalf("an invalid configuration was saved: %q", h.config.saves)
	}
	if len(h.config.saves) != 0 {
		t.Fatalf("an invalid configuration was saved: %q", h.config.saves)
	}
}

// A stopped Server has no repository control, so no request from the Web can
// be pending. That is the empty state, not something needing attention.
func TestStoppedServerShowsNoWebRequestsInsteadOfAnAlert(t *testing.T) {
	h := newHarness(t, func(snapshot *dto.DesktopSnapshot, _ *fakeConfig) {
		snapshot.Runtime.Phase = dto.RuntimeStopped
		snapshot.Runtime.Ownership = dto.OwnershipNone
		snapshot.Runtime.DesiredState = dto.DesiredStopped
	})
	h.storage.actionsErr = operation.NewError(dto.ErrorRepositoryControlUnavailable, "storage control is unavailable")
	h.click(locales.DockStorage())
	h.want(locales.StorageNoWebRequests())
	h.wantNot(locales.ErrorsRepositoryControlUnavailable(), locales.ActionNoticeNeedsAttention(locales.StorageWebRequests()))
}

// Any other failure to read the requests is still shown, with a way to retry.
func TestFailureToReadWebRequestsIsShown(t *testing.T) {
	h := newHarness(t, nil)
	h.storage.actionsErr = operation.NewError(dto.ErrorControllerBusy, "busy")
	h.click(locales.DockStorage())
	h.want(locales.ErrorsControllerBusy(), locales.ActionNoticeNeedsAttention(locales.StorageWebRequests()))
}

func TestUnconfiguredRuntimeShowsTheSetupGuideInsteadOfAPanel(t *testing.T) {
	h := newHarness(t, func(snapshot *dto.DesktopSnapshot, _ *fakeConfig) {
		snapshot.Runtime = dto.RuntimeSnapshot{Phase: dto.RuntimeStopped}
		snapshot.Host.SettingsNavigation.Route = RouteStorage
	})
	h.want(locales.SetupRequiredTitle())
	h.wantNot(locales.DockUpdates(), locales.HeaderOpenProduct())
	h.click(locales.SetupRequiredBackToSetup())
	if !slices.Equal(h.host.routes, []string{RouteOnboarding}) {
		t.Fatalf("routes = %q, want the guide to lead back to the wizard", h.host.routes)
	}
}

func TestPreferenceEditPreviewsAtOnceAndSavesOnce(t *testing.T) {
	h := newHarness(t, nil)
	h.want(locales.GeneralTitle(), locales.GeneralThemeSystem())
	h.wantNot(locales.SettingsUnsaved())

	h.click(locales.GeneralThemeSystem())
	h.click(locales.GeneralThemeDark())
	if last := h.applied[len(h.applied)-1]; last.Theme != "dark" {
		t.Fatalf("previewed preferences = %#v, want the dark appearance at once", last)
	}
	if len(h.preferences.saved) != 0 {
		t.Fatal("an edit was persisted before Save")
	}
	h.want(locales.SettingsUnsaved())

	h.click(locales.CommonSave())
	if len(h.preferences.saved) != 1 || h.preferences.saved[0].Theme != "dark" {
		t.Fatalf("saved preferences = %#v, want one save of the dark appearance", h.preferences.saved)
	}
	if len(h.config.saves)+len(h.config.applies) != 0 {
		t.Fatal("a preference-only save ran a configuration transaction")
	}
	h.want(locales.SettingsSaved())
	h.tt.Advance(savedHold + time.Second)
	h.wantNot(locales.SettingsSaved(), locales.SettingsUnsaved())
}

func TestCancelRestoresTheSavedPreferences(t *testing.T) {
	h := newHarness(t, nil)
	h.click(locales.GeneralThemeSystem())
	h.click(locales.GeneralThemeLight())
	h.click(locales.CommonCancel())
	if last := h.applied[len(h.applied)-1]; last.Theme != "system" {
		t.Fatalf("appearance after cancel = %q, want the saved one back", last.Theme)
	}
	if h.panel.Dirty() || len(h.preferences.saved) != 0 {
		t.Fatal("cancel left a dirty draft or persisted it")
	}
	h.wantNot(locales.SettingsUnsaved())
}

func TestRuntimeEditAppliesWithARestartWhileTheServerRuns(t *testing.T) {
	h := newHarness(t, nil)
	h.click(locales.DockServer())
	h.want(locales.ServerTitle(), locales.RuntimeWorkspaceInfo())
	h.click(locales.RuntimeWorkspaceInfo())
	h.click(locales.RuntimeWorkspaceDebug())
	if len(h.config.patches) != 1 || h.config.patches[0].LoggingLevel != "debug" {
		t.Fatalf("patches = %#v, want the logging level patched into the candidate", h.config.patches)
	}
	h.click(locales.CommonSave())
	if !slices.Equal(h.config.applies, []string{"candidate-1"}) || len(h.config.saves) != 0 {
		t.Fatalf("applies = %q, saves = %q; want a controlled apply of the patched candidate", h.config.applies, h.config.saves)
	}
	// A patch must not replace the fingerprint the edit started from.
	if !slices.Equal(h.config.bases, []string{"base-1"}) {
		t.Fatalf("base fingerprints = %q, want the draft's own base", h.config.bases)
	}
	h.want(locales.SettingsSaving())
	h.finish("config-op", operation.Succeeded)
	h.want(locales.SettingsSaved())
}

func TestMovingASavedDefaultStorageLocationIsConfirmedFirst(t *testing.T) {
	h := newHarness(t, nil)
	h.storage.pick = "/elsewhere"
	h.click(locales.DockStorage())
	h.click(locales.StorageLocateDefaultLocation())
	if len(h.config.patches) != 0 {
		t.Fatal("the saved location was changed without confirmation")
	}
	h.want(locales.StorageDefaultLocationMigrationConfirmation())
	h.click(locales.CommonContinue())
	if len(h.config.patches) != 1 || h.config.patches[0].StoragePath != "/elsewhere" {
		t.Fatalf("patches = %#v, want the confirmed folder", h.config.patches)
	}
}

func TestDecliningTheMoveKeepsTheSavedLocation(t *testing.T) {
	h := newHarness(t, nil)
	h.storage.pick = "/elsewhere"
	h.click(locales.DockStorage())
	h.click(locales.StorageLocateDefaultLocation())
	h.click(locales.CommonCancel())
	if len(h.config.patches) != 0 || h.panel.Dirty() {
		t.Fatal("a declined move changed the draft")
	}
}

func TestHostActionApprovalFollowsItsOperation(t *testing.T) {
	h := newHarness(t, nil)
	h.storage.hostActions = []dto.HostActionTicket{{
		ID: "action-1", Kind: "authorize_storage_location", Actor: "admin", Nonce: "nonce-1",
		Status: "pending", ExpiresAt: time.Unix(1_800_000_600, 0),
	}}
	h.click(locales.DockStorage())
	h.want(locales.StorageHostActionAddLocation(), locales.StorageReviewAndChoose())
	h.click(locales.StorageReviewAndChoose())
	if !slices.Equal(h.storage.approved, []string{"action-1:nonce-1"}) {
		t.Fatalf("approved = %q, want the ticket's own nonce", h.storage.approved)
	}
	h.want(locales.StorageHostActionChoosing())
	h.finish("host-action-op", operation.Succeeded)
	h.want(locales.StorageHostActionApproved())
}

func TestDismissingTheFolderPickerLeavesTheRequestPending(t *testing.T) {
	h := newHarness(t, nil)
	h.storage.approveErr = operation.NewError(dto.ErrorSelectionCancelled, "directory selection was cancelled")
	h.storage.hostActions = []dto.HostActionTicket{{
		ID: "action-1", Kind: "open_repository", Actor: "admin", Nonce: "nonce-1",
		Status: "pending", ExpiresAt: time.Unix(1_800_000_600, 0),
	}}
	h.click(locales.DockStorage())
	h.click(locales.StorageReviewAndChoose())
	// Nothing failed: the same request can be approved again or declined.
	h.want(locales.StorageReviewAndChoose())
	h.wantNot(locales.ErrorsSelectionCancelled())
	h.click(locales.CommonDecline())
	if !slices.Equal(h.storage.declined, []string{"action-1"}) {
		t.Fatalf("declined = %q", h.storage.declined)
	}
	h.want(locales.StorageNoWebRequests())
}

func TestRiskyHostActionNamesEveryRiskBeforeItCanContinue(t *testing.T) {
	h := newHarness(t, nil)
	h.storage.hostActions = []dto.HostActionTicket{{
		ID: "action-1", Kind: "authorize_storage_location", Actor: "admin", Nonce: "nonce-1",
		Status: "needs_decision", RiskWarnings: []string{"removable_storage", "network_filesystem"},
		ExpiresAt: time.Unix(1_800_000_600, 0),
	}}
	h.click(locales.DockStorage())
	h.want(locales.StorageHostActionRiskTitle(), "• "+locales.StorageRiskRemovable(), "• "+locales.StorageRiskNetwork(),
		locales.StorageConfirmRiskAndContinue())
}

func TestServerActionFailureIsShownAndTheButtonRecovers(t *testing.T) {
	h := newHarness(t, nil)
	h.click(locales.DockServer())
	h.click(locales.CommonRestart())
	if !slices.Equal(h.runtime.actions, []string{"restart"}) {
		t.Fatalf("actions = %q", h.runtime.actions)
	}
	h.want(locales.ServerRestarting())
	h.update(func(snapshot *dto.DesktopSnapshot) {
		snapshot.Operations = append(snapshot.Operations, dto.OperationSnapshot{
			OperationID: "runtime-op", State: string(operation.Failed),
			Error: dto.Error{Code: dto.ErrorReadinessTimeout},
		})
	})
	h.want(locales.ErrorsReadinessTimeout(), locales.CommonRestart())
}

func TestEveryPageBuildsInEveryLanguage(t *testing.T) {
	for _, language := range []string{"en", "zh-Hans"} {
		for _, route := range []string{RouteGeneral, RouteServer, RouteStorage, RouteLumen, RouteUpdates, RouteRecovery, RouteOnboarding} {
			t.Run(language+route, func(t *testing.T) {
				h := newHarness(t, func(snapshot *dto.DesktopSnapshot, _ *fakeConfig) {
					snapshot.Host.SettingsNavigation.Route = route
					snapshot.Lumen.InstallerAvailable, snapshot.Lumen.ProcessAvailable = true, true
					snapshot.Lumen.InstallPhase = dto.LumenInstalled
					snapshot.Lumen.AvailablePresets = []string{"minimal", "basic"}
					snapshot.Lumen.AvailableProfiles = []string{"macos-arm64-metal"}
					snapshot.Update.ProviderAvailable = true
					snapshot.Update.AvailableVersion = "26.2.0"
				})
				i18n.SetLocaleFunc(func() string { return language })
				t.Cleanup(func() { i18n.SetLocaleFunc(func() string { return "en" }) })
				h.tt.SetLocale(language)
				h.tt.Frame()
				if len(h.tt.Texts()) == 0 {
					t.Fatal("the page built nothing")
				}
				for _, info := range h.tt.AccessAll() {
					if info.Role == ui.RoleButton && info.Name == "" {
						t.Errorf("a button at %v has no name for assistive technology", info.Bounds)
					}
				}
			})
		}
	}
}

// A control-plane code that lands without copy would reach the person as an
// English internal message. The codes are read from their declaration, so a
// new one fails here until codeText knows it.
func TestEveryErrorCodeHasCopy(t *testing.T) {
	source, err := os.ReadFile("../control/dto/errors.go")
	if err != nil {
		t.Fatal(err)
	}
	codes := regexp.MustCompile(`Error\w+\s+ErrorCode = "([a-z_]+)"`).FindAllStringSubmatch(string(source), -1)
	if len(codes) < 20 {
		t.Fatalf("found only %d error codes; the declaration moved", len(codes))
	}
	for _, match := range codes {
		if codeText(dto.ErrorCode(match[1])) == "" {
			t.Errorf("error code %q has no copy in codeText", match[1])
		}
	}
}

// The tray and the Settings window localize a status by its key. A key the
// presentation gains must gain a label too, or it falls back to English.
func TestEveryPresentationKeyHasALabel(t *testing.T) {
	source, err := os.ReadFile("../control/presentation.go")
	if err != nil {
		t.Fatal(err)
	}
	keys := regexp.MustCompile(`"([a-z]+(?:-[a-z]+)*)", "[A-Z][A-Za-z ]+"|Key: "([a-z-]+)"`).FindAllStringSubmatch(string(source), -1)
	if len(keys) < 15 {
		t.Fatalf("found only %d presentation keys; the declaration moved", len(keys))
	}
	const fallback = "\x00fallback"
	for _, match := range keys {
		key := match[1] + match[2]
		if StatusLabel(dto.ProcessPresentation{Key: key, Label: fallback}) == fallback {
			t.Errorf("presentation key %q has no label in StatusLabel", key)
		}
	}
}
