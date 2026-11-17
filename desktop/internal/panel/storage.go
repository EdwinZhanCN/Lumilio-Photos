package panel

import (
	"strings"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/locales"
	"desktop/internal/operation"

	"github.com/egoist/mygo/i18n"
	"github.com/egoist/mygo/ui"
)

// hostActionPoll is how often the pending requests from the Web are read
// while the Storage page shows and the Server runs.
const hostActionPoll = 3 * time.Second

type storageState struct {
	shortcuts     []dto.StorageShortcut
	hostActions   []dto.HostActionTicket
	loadErr       string
	hostActionErr string
	declining     string
	operation     tracker

	loaded        bool
	loadedVersion uint64
	loadedPhase   dto.RuntimePhase
	lastPoll      time.Time
}

func (p *Panel) refreshShortcuts() {
	p.async(func() func() {
		items, err := p.services.Storage.ListShortcuts()
		return func() {
			p.storage.loadErr = errorText(err)
			if err == nil {
				p.storage.shortcuts = items
			}
		}
	})
}

func (p *Panel) refreshHostActions() {
	p.async(func() func() {
		items, err := p.services.Storage.ListHostActions()
		return func() {
			// Requests are read through the in-process repository control,
			// which a stopped Server does not have. No request can exist
			// then, so that answer is the empty list, not a failure.
			if operation.ErrorCodeOf(err) == dto.ErrorRepositoryControlUnavailable {
				p.storage.hostActions, p.storage.hostActionErr = nil, ""
				return
			}
			p.storage.hostActionErr = errorText(err)
			if err == nil {
				p.storage.hostActions = items
			}
		}
	})
}

func (p *Panel) approveHostAction(action dto.HostActionTicket) {
	s := &p.storage
	s.operation.begin()
	id, version := p.requestID("host-action"), p.snapshot.Storage.Version
	p.async(func() func() {
		receipt, err := p.services.Storage.ApproveHostAction(id, version, action.ID, action.Nonce)
		return func() {
			// Dismissing the picker is a normal outcome: the request stays
			// pending for another approval or a decline, so nothing failed.
			if operation.ErrorCodeOf(err) == dto.ErrorSelectionCancelled {
				s.operation.reset()
				return
			}
			s.operation.resolve(receipt, err, p.snapshot.Operations, p.now())
		}
	})
}

func (p *Panel) declineHostAction(action dto.HostActionTicket) {
	s := &p.storage
	s.declining = action.ID
	p.async(func() func() {
		err := p.services.Storage.DeclineHostAction(action.ID)
		return func() {
			s.declining = ""
			if err != nil {
				s.hostActionErr = errorText(err)
				p.toastError(locales.StorageHostActionDeclineFailed(), s.hostActionErr)
				return
			}
			p.refreshHostActions()
		}
	})
}

func (p *Panel) openLocation(id string) {
	p.async(func() func() {
		err := p.services.Storage.OpenLocation(id)
		return func() {
			if err != nil {
				p.toastError(locales.ToastOpenFailed(), errorText(err))
			}
		}
	})
}

func (p *Panel) storageView(c *ui.Context) {
	s := &p.storage
	runtime := p.snapshot.Runtime
	running := runtime.Phase == dto.RuntimeRunning
	now := p.now()
	if !s.loaded || s.loadedVersion != p.snapshot.Storage.Version || s.loadedPhase != runtime.Phase {
		s.loaded, s.loadedVersion, s.loadedPhase, s.lastPoll = true, p.snapshot.Storage.Version, runtime.Phase, now
		p.refreshShortcuts()
		p.refreshHostActions()
	}
	if running {
		if now.Sub(s.lastPoll) >= hostActionPoll {
			s.lastPoll = now
			p.refreshHostActions()
		}
		c.After(hostActionPoll - now.Sub(s.lastPoll))
	}

	pageHeading(c, locales.StorageTitle(), locales.StorageDescription())
	if s.loadErr != "" {
		notice(c, toneDanger, locales.ActionNoticeNeedsAttention(locales.DockStorage()), s.loadErr, locales.CommonRetry(), p.refreshShortcuts)
	}

	section(c, locales.StorageWebRequests(), func() {
		if s.hostActionErr != "" {
			settingRow(c, locales.ActionNoticeNeedsAttention(locales.StorageWebRequests()), s.hostActionErr, func() {
				if ui.Button(c.Key("host-actions-retry"), locales.CommonRetry()).Clicked() {
					p.refreshHostActions()
				}
			})
		}
		if len(s.hostActions) == 0 {
			emptyRow(c, locales.StorageNoWebRequests(), locales.StorageNoWebRequestsHint())
			return
		}
		for _, action := range s.hostActions {
			p.hostActionRow(c, action)
		}
	})

	configured := ""
	if p.draft.runtime != nil {
		configured = p.draft.runtime.StoragePath
	}
	var fallback *dto.StorageShortcut
	var defaultLocation *dto.StorageShortcut
	for i := range s.shortcuts {
		item := &s.shortcuts[i]
		if item.Kind == "default" {
			defaultLocation = item
			break
		}
		if fallback == nil && configured != "" && item.Path == configured {
			fallback = item
		}
	}
	if defaultLocation == nil {
		defaultLocation = fallback
	}

	section(c, locales.StorageDefaultLocation(), func() {
		description := configured
		if defaultLocation != nil {
			description = locationDescription(*defaultLocation)
		} else if description == "" {
			description = locales.StorageNoDefault()
		}
		settingRow(c, locales.StorageDefaultLocation(), description, func() {
			if ui.Button(c.Key("default-open"), locales.CommonOpen()).
				Disabled(defaultLocation == nil || !defaultLocation.CanOpen).Clicked() {
				p.openLocation(defaultLocation.ID)
			}
			if ui.Button(c.Key("default-locate"), locales.StorageLocateDefaultLocation()).
				Disabled(p.draft.busy()).Clicked() {
				p.chooseDefaultStorage()
			}
		})
	})

	section(c, locales.StorageAdditional(), func() {
		shown := 0
		for _, item := range s.shortcuts {
			if defaultLocation != nil && item.ID == defaultLocation.ID {
				continue
			}
			shown++
			name := item.Name
			if name == "" {
				name = locales.StorageLocation()
			}
			settingRow(c.Key("location:"+item.ID), name, locationDescription(item), func() {
				if ui.Button(c, locales.CommonOpen()).Disabled(!item.CanOpen).Clicked() {
					p.openLocation(item.ID)
				}
			})
		}
		if shown == 0 {
			hint := locales.StorageStartServerHint()
			if running {
				hint = locales.StorageNoAdditionalHint()
			}
			emptyRow(c, locales.StorageNoAdditional(), hint)
		}
	})
}

// hostActionRow is one request the Web made for a folder of this computer.
// The Web drives the durable task; the nonce and the folder the person
// picks stay on this side.
func (p *Panel) hostActionRow(c *ui.Context, action dto.HostActionTicket) {
	s := &p.storage
	theme := c.Theme()
	needsDecision := action.Status == "needs_decision"
	hasRisks := len(action.RiskWarnings) > 0
	purpose := action.Purpose
	if purpose == "" {
		purpose = locales.StorageHostActionDescription()
	}
	ui.Column(c.Key("host-action:"+action.ID)).Padding(12, 16).Gap(10).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			ui.Text(c, hostActionLabel(action.Kind))
			ui.Text(c, locales.StorageHostActionRequestedBy(purpose, action.Actor)).FontSize(12).TextColor(theme.TextMuted)
		})
		if needsDecision && hasRisks {
			ui.Column(c).Padding(10, 12).Gap(4).Radius(8).Border(1, theme.Warning).Children(func() {
				ui.Text(c, locales.StorageHostActionRiskTitle()).FontSize(12).Bold()
				for _, warning := range action.RiskWarnings {
					ui.Text(c, "• "+storageRiskLabel(warning)).FontSize(12)
				}
			})
		}
		ui.Row(c).Gap(8).Children(func() {
			expires := i18n.FormatDate(c.Locale(), action.ExpiresAt.Local(), i18n.DateOptions{Time: i18n.MediumStyle})
			ui.Text(c, locales.StorageHostActionExpires(expires)).FontSize(12).TextColor(theme.TextMuted).Grow(1)
			if ui.Button(c, locales.CommonDecline()).
				Disabled(s.declining == action.ID || s.operation.loading()).Clicked() {
				p.declineHostAction(action)
			}
			label, loading := locales.StorageReviewAndChoose(), locales.StorageHostActionChoosing()
			if needsDecision {
				label, loading = locales.StorageConfirmRiskAndContinue(), locales.StorageHostActionConfirmingRisk()
				if !hasRisks {
					label = locales.StorageHostActionResolveInWeb()
				}
			}
			disabled := s.declining != "" || (needsDecision && !hasRisks)
			if trackedButton(c, &s.operation, label, loading, locales.StorageHostActionApproved(), true, disabled).Clicked() {
				p.approveHostAction(action)
			}
		})
	})
}

func locationDescription(item dto.StorageShortcut) string {
	place := item.Path
	if place == "" {
		place = item.Status
	}
	return strings.Join([]string{place, locales.StorageRepositoryCount(int(item.RepositoryCount))}, " · ")
}
