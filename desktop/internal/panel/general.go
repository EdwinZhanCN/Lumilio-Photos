package panel

import (
	"desktop/internal/control/dto"
	"desktop/internal/locales"

	"github.com/egoist/mygo/ui"
)

func (p *Panel) generalView(c *ui.Context) {
	d := &p.draft
	if d.preferences == nil {
		loadingRow(c, locales.CommonLoadingPreferences())
		return
	}
	busy := d.busy()
	pageHeading(c, locales.GeneralTitle(), locales.GeneralDescription())
	section(c, locales.GeneralDisplay(), func() {
		settingRow(c, locales.GeneralLanguage(), locales.GeneralLanguageDescription(), func() {
			p.languageSelect(c, "general-language", busy)
		})
		settingRow(c, locales.GeneralRegion(), locales.GeneralRegionDescription(), func() {
			p.regionSelect(c, "general-region", busy)
		})
		settingRow(c, locales.GeneralTheme(), locales.GeneralThemeDescription(), func() {
			theme := d.preferences.Theme
			if theme == "" {
				theme = "system"
			}
			if selectOf(c, "general-theme", &theme, []option[string]{
				{"system", locales.GeneralThemeSystem()},
				{"light", locales.GeneralThemeLight()},
				{"dark", locales.GeneralThemeDark()},
			}, locales.GeneralTheme(), busy) {
				p.updatePreference(func(preferences *dto.DesktopPreferences) { preferences.Theme = theme })
			}
		})
		settingRow(c, locales.GeneralOpenOnLaunch(), locales.GeneralOpenOnLaunchDescription(), func() {
			open := d.preferences.OpenProductOnLaunch
			if ui.Switch(c.Key("general-open-on-launch"), &open).Label(locales.GeneralOpenOnLaunch()).Disabled(busy).Changed() {
				p.updatePreference(func(preferences *dto.DesktopPreferences) { preferences.OpenProductOnLaunch = open })
			}
		})
	})
}

func (p *Panel) languageSelect(c *ui.Context, key string, disabled bool) {
	locale := p.draft.preferences.Locale
	// Each language is named in itself, so a person who cannot read the
	// current one still finds theirs.
	if selectOf(c, key, &locale, []option[string]{
		{"en", "English"},
		{"zh-CN", "简体中文"},
	}, locales.GeneralLanguage(), disabled) {
		p.updatePreference(func(preferences *dto.DesktopPreferences) { preferences.Locale = locale })
	}
}

func (p *Panel) regionSelect(c *ui.Context, key string, disabled bool) {
	region := p.draft.preferences.Region
	if selectOf(c, key, &region, []option[string]{
		{"global", locales.RegionGlobal()},
		{"china", locales.RegionChina()},
	}, locales.GeneralRegion(), disabled) {
		p.updatePreference(func(preferences *dto.DesktopPreferences) { preferences.Region = region })
	}
}

func (p *Panel) networkSelect(c *ui.Context, key string, disabled bool) {
	mode := p.draft.runtime.NetworkMode
	options := []option[string]{
		{"local", locales.NetworkLocal()},
		{"lan", locales.NetworkLan()},
	}
	// A listener written by hand in the manifest stays selectable; it is
	// never offered to a configuration that does not have one.
	if mode == "custom" {
		options = append(options, option[string]{"custom", locales.NetworkCustom()})
	}
	if selectOf(c, key, &mode, options, locales.NetworkAccess(), disabled) {
		p.updateRuntime(func(settings *dto.RuntimeConfigSettings) { settings.NetworkMode = mode })
	}
}

const onboardingSteps = 4

// onboardingView is the first-run wizard. It edits the same draft the
// Settings pages do, and its last step saves it, which starts the Server.
func (p *Panel) onboardingView(c *ui.Context) {
	theme := c.Theme()
	d := &p.draft
	if d.runtime == nil || d.preferences == nil {
		ui.Column(c).Fill().Center().Gap(12).Padding(32).Children(func() {
			if d.phase == phaseError {
				ui.Text(c, locales.OnboardingSetupFailed()).Bold()
				ui.Text(c, d.err).TextColor(theme.TextMuted)
				if ui.Button(c, locales.CommonRetry()).Clicked() {
					p.loadDraft()
				}
				return
			}
			ui.Spinner(c).Label(locales.OnboardingPreparing())
			ui.Text(c, locales.OnboardingPreparingDescription()).TextColor(theme.TextMuted)
		})
		return
	}
	busy := d.busy()
	step := p.onboardingStep
	ui.Column(c).Fill().Padding(28, 48, 32, 48).Gap(20).Label(locales.OnboardingAriaLabel()).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, locales.ProductCompactName()).FontSize(15).Bold().Grow(1)
			ui.Text(c, locales.OnboardingStep(step+1, onboardingSteps)).TextColor(theme.TextMuted)
		})
		ui.Progress(c, float64(step+1)/onboardingSteps).Label(locales.OnboardingStep(step+1, onboardingSteps))
		ui.Column(c.Key(step)).Grow(1).Justify(ui.Center).Children(func() {
			ui.Column(c).Gap(20).WidthPercent(100).MaxWidth(680).AlignSelf(ui.Center).Children(func() {
				switch step {
				case 0:
					pageHeading(c, locales.OnboardingGeneralTitle(), locales.OnboardingGeneralDescription())
					section(c, locales.OnboardingDesktop(), func() {
						settingRow(c, locales.OnboardingLanguage(), locales.OnboardingLanguageDescription(), func() {
							p.languageSelect(c, "onboarding-language", busy)
						})
						settingRow(c, locales.OnboardingRegion(), locales.OnboardingRegionDescription(), func() {
							p.regionSelect(c, "onboarding-region", busy)
						})
					})
				case 1:
					pageHeading(c, locales.OnboardingStorageTitle(), locales.OnboardingStorageDescription())
					section(c, locales.OnboardingStorage(), func() {
						settingRow(c, locales.OnboardingStorageLocation(), d.runtime.StoragePath, func() {
							if ui.Button(c, locales.CommonChoose()).Disabled(busy).Clicked() {
								p.chooseDefaultStorage()
							}
						})
					})
				case 2:
					pageHeading(c, locales.OnboardingNetworkTitle(), locales.OnboardingNetworkDescription())
					section(c, locales.OnboardingServer(), func() {
						settingRow(c, locales.OnboardingNetworkAccess(), locales.OnboardingNetworkRecommended(), func() {
							p.networkSelect(c, "onboarding-network", busy)
						})
					})
				default:
					p.onboardingSummary(c)
				}
			})
		})
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, locales.CommonBack()).Disabled(step == 0 || busy).Clicked() {
				p.onboardingStep--
			}
			ui.Spacer(c)
			if step < onboardingSteps-1 {
				blocked := busy || (step == 1 && d.runtime.StoragePath == "")
				if ui.PrimaryButton(c, locales.CommonContinue()).Disabled(blocked).Clicked() {
					p.onboardingStep++
				}
				return
			}
			label := locales.OnboardingFinishSetup()
			if d.phase == phaseSaving {
				label = locales.CommonFinishing()
			}
			blocked := busy || (d.validation != nil && !d.validation.Valid)
			if ui.PrimaryButton(c.Key("onboarding-finish"), label).Disabled(blocked).Clicked() {
				p.saveDraft(func(ok bool) {
					if ok {
						p.onboardingStep = 0
						p.navigate(RouteGeneral)
					}
				})
			}
		})
	})
}

func (p *Panel) onboardingSummary(c *ui.Context) {
	d := &p.draft
	theme := c.Theme()
	lumen := p.snapshot.Lumen
	available := lumen.InstallerAvailable && lumen.ProcessAvailable
	pageHeading(c, locales.OnboardingReadyTitle(), locales.OnboardingReadyDescription())
	section(c, locales.OnboardingSummary(), func() {
		settingRow(c, locales.OnboardingStorageLocation(), d.runtime.StoragePath, func() {
			ui.Text(c, locales.OnboardingSelected()).TextColor(theme.TextMuted)
		})
		settingRow(c, locales.OnboardingNetworkAccess(), networkLabel(d.runtime.NetworkMode), func() {
			ui.Text(c, locales.OnboardingConfigured()).TextColor(theme.TextMuted)
		})
		description, badge := locales.OnboardingLumenNotIncluded(), locales.OnboardingUnavailable()
		if available {
			description, badge = locales.OnboardingLumenAvailable(), locales.OnboardingOptional()
		}
		settingRow(c, locales.OnboardingLumenAI(), description, func() {
			pill(c, badge, dto.DotGray)
		})
	})
	switch {
	case d.phase == phaseError && d.err != "":
		notice(c, toneDanger, locales.OnboardingSetupFailed(), d.err, "", nil)
	case d.validation != nil && !d.validation.Valid:
		notice(c, toneDanger, locales.OnboardingSettingsInvalid(), firstIssue(d.validation), "", nil)
	}
}
