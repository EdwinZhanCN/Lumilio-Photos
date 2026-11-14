package panel

import (
	"fmt"
	"sync/atomic"
	"time"

	"desktop/internal/control"
	"desktop/internal/control/dto"
	"desktop/internal/locales"

	"github.com/egoist/mygo/ui"
)

// Routes the host and the tray can ask the Settings window to show.
const (
	RouteOnboarding = "/onboarding"
	RouteRecovery   = "/recovery"
	RouteGeneral    = "/general"
	RouteServer     = "/server"
	RouteStorage    = "/storage"
	RouteLumen      = "/lumen"
	RouteUpdates    = "/updates"
)

// Services is the control-plane surface the panel calls.
type Services struct {
	Desktop *control.DesktopService
	Runtime *control.RuntimeService
	Storage *control.StorageService
	Lumen   *control.LumenService
	Update  *control.UpdateService
}

// Options configures a Panel.
type Options struct {
	Services Services
	// Post runs fn on the UI thread and draws a new frame. Nil runs it
	// inline, as tests do.
	Post func(fn func())
	// Go starts work off the UI thread. Nil runs it inline, as tests do.
	Go func(fn func())
	// ApplyPreferences previews the language and the appearance on the host
	// as the person edits them, and restores the saved ones on cancel.
	ApplyPreferences func(dto.DesktopPreferences)
	// Now is the clock, time.Now when nil.
	Now func() time.Time
}

// Panel is the state of the Settings window. Every field is read and written
// on the UI thread.
type Panel struct {
	services Services
	post     func(func())
	spawn    func(func())
	apply    func(dto.DesktopPreferences)
	now      func() time.Time
	sequence atomic.Uint64

	snapshot      dto.DesktopSnapshot
	hasSnapshot   bool
	route         string
	navigationSeq uint64
	toasts        []ui.Toast

	draft   draft
	waiters []*waiter

	onboardingStep int

	runtimeOp  tracker
	recoveryOp tracker

	storage storageState
	lumen   lumenState
	update  tracker

	confirmStorageMove bool
	pendingStoragePath string
}

// New returns a Panel that shows nothing until its first SetSnapshot.
func New(options Options) *Panel {
	p := &Panel{
		services: options.Services,
		post:     options.Post,
		spawn:    options.Go,
		apply:    options.ApplyPreferences,
		now:      options.Now,
		route:    RouteGeneral,
	}
	if p.post == nil {
		p.post = func(fn func()) { fn() }
	}
	if p.spawn == nil {
		p.spawn = func(fn func()) { fn() }
	}
	if p.apply == nil {
		p.apply = func(dto.DesktopPreferences) {}
	}
	if p.now == nil {
		p.now = time.Now
	}
	p.draft.phase = phaseLoading
	p.lumen.logLevel = "INFO"
	p.runtimeOp.onFailed = func(message string) { p.toastError(locales.ServerActionFailed(), message) }
	p.recoveryOp.onFailed = func(message string) { p.toastError(locales.RecoveryFailed(), message) }
	p.recoveryOp.onSucceeded = func() { p.navigate(RouteServer) }
	p.lumen.operation.onFailed = func(message string) { p.toastError(locales.LumenActionFailed(), message) }
	p.update.onFailed = func(message string) { p.toastError(locales.UpdatesActionFailed(), message) }
	p.storage.operation.onFailed = func(message string) { p.toastError(locales.StorageHostActionFailed(), message) }
	p.storage.operation.onSucceeded = func() {
		p.refreshShortcuts()
		p.refreshHostActions()
	}
	return p
}

// SetSnapshot installs a revision of the control-plane snapshot. Call it on
// the UI thread; the caller draws the next frame.
func (p *Panel) SetSnapshot(snapshot dto.DesktopSnapshot) {
	first := !p.hasSnapshot
	p.snapshot = control.ProjectCapabilities(snapshot)
	p.hasSnapshot = true
	if navigation := snapshot.Host.SettingsNavigation; first || navigation.Sequence != p.navigationSeq {
		p.navigationSeq = navigation.Sequence
		if navigation.Route != "" {
			p.route = navigation.Route
		}
	}
	if p.draft.loadedInstance != snapshot.InstanceID {
		p.draft.loadedInstance = snapshot.InstanceID
		p.loadDraft()
	}
	p.observeOperations()
}

// Opened tells the panel its window was created again. A draft the person
// did not edit is read anew, so it never applies over a stale base.
func (p *Panel) Opened() {
	if p.hasSnapshot && !p.draft.dirty() && p.draft.phase != phaseSaving && p.draft.phase != phasePreparing {
		p.loadDraft()
	}
}

// Dirty reports whether the person has edits that are not saved.
func (p *Panel) Dirty() bool { return p.draft.dirty() }

func (p *Panel) observeOperations() {
	operations := p.snapshot.Operations
	now := p.now()
	for _, t := range []*tracker{&p.runtimeOp, &p.recoveryOp, &p.lumen.operation, &p.update, &p.storage.operation} {
		t.observe(operations, now)
	}
	p.observeWaiters(now)
}

// async runs work off the UI thread. The function work returns, if any,
// applies its result on the UI thread.
func (p *Panel) async(work func() func()) {
	p.spawn(func() {
		if done := work(); done != nil {
			p.post(done)
		}
	})
}

func (p *Panel) requestID(prefix string) string {
	return fmt.Sprintf("%s-%s-%d-%d", prefix, p.snapshot.InstanceID, p.now().UnixNano(), p.sequence.Add(1))
}

func (p *Panel) toastError(title, description string) {
	p.toasts = append(p.toasts, ui.Toast{Title: title, Description: description, Type: "error", Timeout: 8 * time.Second})
}

// navigate asks the host for a route. The host owns the guard that keeps
// recovery and onboarding from being left early, and answers through the
// snapshot's navigation intent.
func (p *Panel) navigate(route string) {
	p.route = route
	if p.services.Desktop == nil {
		return
	}
	if err := p.services.Desktop.ShowSettings(route); err != nil {
		p.toastError(locales.ToastPageFailed(), errorText(err))
	}
}

func (p *Panel) openProduct() {
	if p.services.Desktop == nil {
		return
	}
	if err := p.services.Desktop.OpenProduct(); err != nil {
		p.toastError(locales.ToastOpenFailed(), errorText(err))
	}
}

// View builds the Settings window for one frame.
func (p *Panel) View(c *ui.Context) {
	for _, toast := range p.toasts {
		c.AddToast(toast)
	}
	p.toasts = nil
	p.tick(c)

	theme := c.Theme()
	root := ui.Column(c).Fill().Background(theme.Background)
	if !p.hasSnapshot {
		root.Center().Gap(12).Children(func() {
			ui.Spinner(c).Label(locales.LoadingLoadingState())
			ui.Text(c, locales.LoadingOpening()).TextColor(theme.TextMuted)
		})
		return
	}

	onboarding := p.route == RouteOnboarding
	recovery := p.route == RouteRecovery
	// Any non-wizard route while the runtime has no saved configuration shows
	// the setup-required guide instead of a bare panel: entries like the
	// tray's storage menu must never strand the person in a dead end.
	setupRequired := !onboarding && !recovery && !p.snapshot.Runtime.Configured

	root.Children(func() {
		switch {
		case onboarding:
			p.onboardingView(c)
		case recovery:
			p.centered(c, func() { p.recoveryView(c) })
		case setupRequired:
			p.centered(c, func() { p.setupRequiredView(c) })
		default:
			p.settingsView(c)
		}
	})
	p.storageMoveConfirmation(c)
}

func (p *Panel) centered(c *ui.Context, fn func()) {
	ui.Column(c).Fill().Center().Padding(32).Children(func() {
		ui.Column(c).Gap(20).Width(520).Children(fn)
	})
}

// tick settles what depends on time alone and asks for the frame it next
// changes on.
func (p *Panel) tick(c *ui.Context) {
	now := p.now()
	next := time.Duration(0)
	wake := func(at time.Time) {
		if at.IsZero() {
			return
		}
		if wait := at.Sub(now); wait > 0 && (next == 0 || wait < next) {
			next = wait
		}
	}
	for _, t := range []*tracker{&p.runtimeOp, &p.recoveryOp, &p.lumen.operation, &p.update, &p.storage.operation} {
		wake(t.settle(now))
	}
	wake(p.draft.settle(now))
	p.observeWaiters(now)
	for _, w := range p.waiters {
		wake(w.deadline)
	}
	if next > 0 {
		c.After(next + time.Millisecond)
	}
}

func (p *Panel) settingsView(c *ui.Context) {
	theme := c.Theme()
	active := normalizeRoute(p.route)
	ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
		selected := active
		sidebar := ui.Sidebar(c, &selected, func() {
			ui.SidebarItem(c, RouteGeneral, iconGeneral, locales.DockGeneral())
			ui.SidebarItem(c, RouteServer, iconServer, locales.DockServer())
			ui.SidebarItem(c, RouteStorage, iconStorage, locales.DockStorage())
			ui.SidebarItem(c, RouteLumen, iconLumen, locales.DockLumen())
			ui.SidebarItem(c, RouteUpdates, iconUpdates, locales.DockUpdates())
		}).Width(208).Label(locales.DockLabel())
		if sidebar.Changed() && selected != active {
			p.navigate(selected)
		}
		ui.Column(c).Grow(1).Children(func() {
			ui.Row(c).Padding(14, 24).Gap(12).Children(func() {
				ui.Text(c, locales.ProductCompactName()).FontSize(15).Bold().Grow(1)
				statusBadge(c, p.snapshot.Runtime.Presentation)
				if ui.PrimaryButton(c, locales.HeaderOpenProduct()).
					Disabled(!p.snapshot.Runtime.Capabilities.CanOpenProduct).Clicked() {
					p.openProduct()
				}
			})
			ui.Divider(c)
			ui.Scroll(c.Key(active)).Grow(1).Children(func() {
				ui.Column(c).Padding(24, 32, 32, 32).Gap(20).MaxWidth(760).Children(func() {
					switch active {
					case RouteServer:
						p.serverView(c)
					case RouteStorage:
						p.storageView(c)
					case RouteLumen:
						p.lumenView(c)
					case RouteUpdates:
						p.updatesView(c)
					default:
						p.generalView(c)
					}
				})
			})
			p.saveBar(c, theme)
		})
	})
}

// saveBar is the one save boundary of the draft: it shows while the person
// has edits, with what keeps them from being saved.
func (p *Panel) saveBar(c *ui.Context, theme *ui.Theme) {
	d := &p.draft
	if !d.dirty() && d.phase != phaseSuccess && d.phase != phaseSaving {
		return
	}
	ui.Divider(c)
	ui.Row(c).Padding(12, 24).Gap(12).Children(func() {
		message, color := locales.SettingsUnsaved(), theme.TextMuted
		switch {
		case d.phase == phaseSaving:
			ui.Spinner(c).Label(locales.SettingsSaving())
			message = locales.SettingsSaving()
		case d.phase == phaseSuccess:
			message, color = locales.SettingsSaved(), theme.Success
		case d.phase == phaseError && d.err != "":
			message, color = d.err, theme.Danger
		case d.validation != nil && !d.validation.Valid:
			message, color = firstIssue(d.validation), theme.Danger
		}
		ui.Text(c, message).TextColor(color).Grow(1)
		if d.phase == phaseSuccess {
			return
		}
		if ui.Button(c, locales.CommonCancel()).Disabled(d.phase == phaseSaving).Clicked() {
			p.cancelDraft()
		}
		blocked := d.phase == phasePreparing || d.phase == phaseSaving || (d.validation != nil && !d.validation.Valid)
		if ui.PrimaryButton(c, locales.CommonSave()).Disabled(blocked).Clicked() {
			p.saveDraft(nil)
		}
	})
}

func (p *Panel) setupRequiredView(c *ui.Context) {
	pageHeading(c, locales.SetupRequiredTitle(), locales.SetupRequiredDescription())
	section(c, locales.SetupRequiredNextStep(), func() {
		settingRow(c, locales.SetupRequiredFinishSetup(), locales.SetupRequiredMinutes(), func() {
			if ui.PrimaryButton(c, locales.SetupRequiredBackToSetup()).Clicked() {
				p.navigate(RouteOnboarding)
			}
		})
	})
}

func (p *Panel) recoveryView(c *ui.Context) {
	description := p.snapshot.Host.Recovery.Message
	if description == "" {
		description = locales.RecoveryDescription()
	}
	pageHeading(c, locales.RecoveryTitle(), description)
	ui.Row(c).Gap(8).Children(func() {
		if trackedButton(c, &p.recoveryOp, locales.RecoveryRestore(), locales.RecoveryRestoring(), locales.RecoveryRestored(), true, false).Clicked() {
			p.restoreLastKnownGood()
		}
		if ui.Button(c, locales.RecoveryReviewSettings()).Clicked() {
			p.navigate(RouteServer)
		}
	})
}

func (p *Panel) restoreLastKnownGood() {
	if p.services.Runtime == nil {
		return
	}
	p.recoveryOp.begin()
	id, version := p.requestID("recovery"), p.snapshot.Runtime.Version
	p.async(func() func() {
		receipt, err := p.services.Runtime.RestoreLastKnownGood(id, version)
		return func() { p.recoveryOp.resolve(receipt, err, p.snapshot.Operations, p.now()) }
	})
}

func normalizeRoute(route string) string {
	switch route {
	case RouteGeneral, RouteServer, RouteStorage, RouteLumen, RouteUpdates:
		return route
	case "/runtime":
		return RouteServer
	default:
		return RouteGeneral
	}
}
