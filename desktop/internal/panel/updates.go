package panel

import (
	"desktop/internal/control/dto"
	"desktop/internal/locales"

	"github.com/egoist/mygo/ui"
)

func (p *Panel) updateAction(action string) {
	p.update.begin()
	id, version := p.requestID("update"), p.snapshot.Update.Version
	p.async(func() func() {
		var receipt dto.OperationReceipt
		var err error
		switch action {
		case "check":
			receipt, err = p.services.Update.Check(id, version)
		case "download":
			receipt, err = p.services.Update.Download(id, version)
		default:
			receipt, err = p.services.Update.RestartAndApply(id, version)
		}
		return func() { p.update.resolve(receipt, err, p.snapshot.Operations, p.now()) }
	})
}

func (p *Panel) updatesView(c *ui.Context) {
	d := &p.draft
	theme := c.Theme()
	update := p.snapshot.Update
	busy := p.update.loading()
	pageHeading(c, locales.DockUpdates(), locales.UpdatesDescription())
	section(c, locales.DockUpdates(), func() {
		settingRow(c, locales.UpdatesCurrentVersion(), locales.UpdatesCurrentVersionDescription(), func() {
			version := update.CurrentVersion
			if version == "" {
				version = locales.CommonUnknown()
			}
			ui.Text(c, version).TextColor(theme.TextMuted).Selectable()
		})
		settingRow(c, locales.UpdatesChannel(), locales.UpdatesChannelDescription(), func() {
			if d.preferences == nil {
				ui.Text(c, locales.CommonLoadingEllipsis()).TextColor(theme.TextMuted)
				return
			}
			channel := d.preferences.UpdateChannel
			if selectOf(c, "updates-channel", &channel, []option[string]{
				{"stable", locales.UpdatesStable()},
				{"beta", locales.UpdatesBeta()},
			}, locales.UpdatesChannel(), d.phase == phaseSaving) {
				p.updatePreference(func(preferences *dto.DesktopPreferences) { preferences.UpdateChannel = channel })
			}
		})
		description := locales.UpdatesNoProvider()
		switch {
		case update.ProviderAvailable && update.Phase == "current":
			description = locales.UpdatesUpToDate()
		case update.ProviderAvailable:
			description = locales.UpdatesCheckDescription()
		}
		settingRow(c, locales.UpdatesCheckTitle(), description, func() {
			label := locales.OnboardingUnavailable()
			if update.ProviderAvailable {
				label = locales.UpdatesCheckNow()
			}
			if trackedButton(c, &p.update, label, locales.UpdatesChecking(), locales.UpdatesChecked(), false, !update.ProviderAvailable).Clicked() {
				p.updateAction("check")
			}
		})
		if update.AvailableVersion == "" {
			return
		}
		settingRow(c, locales.UpdatesAvailable(), locales.UpdatesAvailableDescription(update.AvailableVersion), func() {
			if update.CanApply {
				if ui.PrimaryButton(c.Key("updates-apply"), locales.UpdatesRestartInstall()).Disabled(busy).Clicked() {
					p.updateAction("apply")
				}
				return
			}
			if ui.Button(c.Key("updates-download"), locales.UpdatesDownload()).Disabled(busy).Clicked() {
				p.updateAction("download")
			}
		})
		if update.Phase == "downloading" {
			ui.Column(c).Padding(12, 16).Children(func() {
				progress := update.Progress
				if progress <= 0 {
					progress = -1
				}
				ui.Progress(c, progress).Label(locales.UpdatesDownload())
			})
		}
	})
	if update.Error.Code != "" {
		label, action := "", func() {}
		if update.ProviderAvailable {
			label, action = locales.CommonRetry(), func() { p.updateAction("check") }
		}
		message := codeText(update.Error.Code)
		if update.Error.Message != "" {
			message = update.Error.Message
		}
		notice(c, toneDanger, locales.ActionNoticeNeedsAttention(locales.DockUpdates()), message, label, action)
	}
}
