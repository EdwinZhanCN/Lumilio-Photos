package panel

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"desktop/internal/control/dto"
	"desktop/internal/locales"
	"desktop/internal/operation"

	"github.com/egoist/mygo/ui"
)

// lumenLogPoll is how often the Control logs are read while the Lumen page
// shows and the Hub is connected.
const lumenLogPoll = 5 * time.Second

const lumenLogBacklog = 200

type lumenState struct {
	operation  tracker
	inputError string

	profile  string
	preset   string
	cacheDir string
	// seen is the setup of the snapshot the selections were last taken
	// from. They follow the snapshot when it changes, not on every frame.
	seen string

	logLevel    string
	logs        []dto.LumenLogEntry
	logsLoading bool
	logsError   string
	lastLogPoll time.Time
	logsFor     string
}

func (p *Panel) lumenAction(action string) {
	l := &p.lumen
	l.operation.begin()
	l.inputError = ""
	id, version := p.requestID("lumen"), p.snapshot.Lumen.Version
	profile, preset, cacheDir := l.profile, l.preset, l.cacheDir
	p.async(func() func() {
		var receipt dto.OperationReceipt
		var err error
		switch action {
		case "install":
			receipt, err = p.services.Lumen.Install(id, version, profile, preset, cacheDir)
		case "start":
			receipt, err = p.services.Lumen.Start(id, version)
		case "stop":
			receipt, err = p.services.Lumen.Stop(id, version)
		case "restart":
			receipt, err = p.services.Lumen.Restart(id, version)
		default:
			receipt, err = p.services.Lumen.RetryCleanup(id, version)
		}
		return func() { l.operation.resolve(receipt, err, p.snapshot.Operations, p.now()) }
	})
}

func (p *Panel) chooseLumenCache() {
	l := &p.lumen
	title := locales.LumenChooseCacheDirectory()
	p.async(func() func() {
		path, err := p.services.Lumen.PickCacheDirectory(title)
		return func() {
			if err != nil {
				if operation.ErrorCodeOf(err) == dto.ErrorSelectionCancelled {
					return
				}
				l.inputError = errorText(err)
				p.toastError(locales.LumenCachePickFailed(), l.inputError)
				return
			}
			if path != "" {
				l.cacheDir, l.inputError = path, ""
			}
		}
	})
}

func (p *Panel) loadLumenLogs() {
	l := &p.lumen
	lumen := p.snapshot.Lumen
	if lumen.ProcessPhase != dto.LumenRunning || !lumen.Control.Connected {
		l.logs, l.logsError = nil, ""
		return
	}
	l.logsLoading = true
	level := l.logLevel
	p.async(func() func() {
		entries, err := p.services.Lumen.GetLogs(lumenLogBacklog, level)
		return func() {
			l.logsLoading = false
			l.logsError = errorText(err)
			if err == nil {
				l.logs = entries
			}
		}
	})
}

func backendLabel(profile string) string {
	switch {
	case strings.HasSuffix(profile, "-metal"):
		return locales.LumenBackendMetal()
	case strings.HasSuffix(profile, "-gpu"):
		return locales.LumenBackendGPU()
	}
	return locales.LumenBackendCPU()
}

func first(values []string) string {
	if len(values) > 0 {
		return values[0]
	}
	return ""
}

func (p *Panel) lumenView(c *ui.Context) {
	l := &p.lumen
	lumen := p.snapshot.Lumen
	releaseReady := lumen.InstallerAvailable && lumen.ProcessAvailable
	installed := lumen.InstallPhase == dto.LumenInstalled

	seen := strings.Join([]string{lumen.Profile, lumen.Preset, lumen.CacheDir,
		strings.Join(lumen.AvailableProfiles, ","), strings.Join(lumen.AvailablePresets, ",")}, "\x00")
	if seen != l.seen {
		l.seen = seen
		l.profile, l.preset, l.cacheDir = lumen.Profile, lumen.Preset, lumen.CacheDir
		if l.profile == "" {
			l.profile = first(lumen.AvailableProfiles)
		}
		if l.preset == "" {
			l.preset = first(lumen.AvailablePresets)
		}
	}

	pageHeading(c, locales.DockLumen(), locales.LumenDescription())
	section(c, locales.LumenAvailability(), func() {
		description := locales.LumenNotIncluded()
		if releaseReady {
			description = locales.LumenReleaseReady()
		}
		settingRow(c, "Lumen Hub", description, func() {
			if releaseReady {
				statusBadge(c, lumen.Presentation)
			} else {
				pill(c, locales.OnboardingUnavailable(), dto.DotGray)
			}
		})
	})
	if !releaseReady {
		return
	}

	busy := l.operation.loading()
	section(c, locales.LumenService(), func() {
		settingRow(c, locales.LumenPreset(), locales.LumenPresetDescription(), func() {
			options := make([]option[string], len(lumen.AvailablePresets))
			for i, preset := range lumen.AvailablePresets {
				options[i] = option[string]{preset, presetLabel(preset)}
			}
			selectOf(c, "lumen-preset", &l.preset, options, locales.LumenPreset(), busy)
		})
		settingRow(c, locales.LumenBackend(), locales.LumenBackendDescription(), func() {
			options := make([]option[string], len(lumen.AvailableProfiles))
			for i, profile := range lumen.AvailableProfiles {
				options[i] = option[string]{profile, backendLabel(profile) + " · " + profile}
			}
			// The backend names the package that was downloaded: it is
			// chosen once, at installation.
			selectOf(c, "lumen-backend", &l.profile, options, locales.LumenBackend(), busy || installed)
		})
		cache := l.cacheDir
		if cache == "" {
			cache = locales.LumenCacheDirectoryDescription()
		}
		settingRow(c, locales.LumenCacheDirectory(), cache, func() {
			if ui.Button(c.Key("lumen-cache"), locales.CommonChoose()).Disabled(busy).Clicked() {
				p.chooseLumenCache()
			}
		})

		title, description := locales.LumenInstallation(), locales.LumenNoProfile()
		label, loading, success := locales.LumenInstall(), locales.LumenInstalling(), locales.LumenInstalled()
		switch {
		case installed:
			title, description = locales.LumenReconfigure(), locales.LumenReconfigureStopped()
			if lumen.ProcessPhase == dto.LumenRunning {
				description = locales.LumenReconfigureRunning()
			}
			label, loading, success = locales.LumenApplyConfiguration(), locales.LumenApplyingConfiguration(), locales.LumenConfigurationApplied()
		case l.profile != "":
			description = locales.LumenProfile() + ": " + l.profile
		}
		changed := l.preset != lumen.Preset || l.cacheDir != lumen.CacheDir
		ready := lumen.InstallerAvailable && l.profile != "" && l.preset != "" && l.cacheDir != "" && (!installed || changed)
		settingRow(c, title, description, func() {
			if trackedButton(c, &l.operation, label, loading, success, !installed, !ready).Clicked() {
				p.lumenAction("install")
			}
		})

		desired := string(lumen.DesiredState)
		if desired == "" {
			desired = string(dto.DesiredDisabled)
		}
		settingRow(c, locales.LumenProcessStatus(), locales.LumenDesiredState()+": "+desired, func() {
			statusBadge(c, lumen.Presentation)
			capabilities := lumen.Capabilities
			if capabilities.CanStartLumen {
				if ui.Button(c, locales.CommonStart()).Disabled(busy).Clicked() {
					p.lumenAction("start")
				}
			}
			switch {
			case capabilities.CanRetryCleanupLumen:
				if ui.Button(c, locales.CommonRetryCleanup()).Disabled(busy).Clicked() {
					p.lumenAction("retry")
				}
			case capabilities.CanStopLumen:
				if ui.Button(c, locales.CommonStop()).Disabled(busy).Clicked() {
					p.lumenAction("stop")
				}
			}
			if capabilities.CanRestartLumen {
				if ui.Button(c, locales.CommonRestart()).Disabled(busy).Clicked() {
					p.lumenAction("restart")
				}
			}
		})
	})

	if installed {
		p.lumenControlView(c, lumen.Control)
		p.lumenLogsView(c, lumen)
	}
	if message := cmpOr(l.operation.err, l.inputError); message != "" {
		notice(c, toneDanger, locales.ActionNoticeNeedsAttention(locales.DockLumen()), message, "", nil)
	}
}

func cmpOr(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

var lumenPhaseOrder = []dto.LumenControlPhase{
	dto.LumenControlStarting, dto.LumenControlDownloading, dto.LumenControlLoading,
	dto.LumenControlWarmup, dto.LumenControlReady,
}

// lumenControlView is the live lifecycle and model state the installed Hub
// reports through its versioned Control service.
func (p *Panel) lumenControlView(c *ui.Context, control dto.LumenControlStatus) {
	theme := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, locales.LumenControlStatus()).FontSize(13).Bold().TextColor(theme.TextMuted)
		ui.Text(c, locales.LumenControlDescription()).FontSize(12).TextColor(theme.TextMuted)
		ui.Column(c).Radius(10).Border(1, theme.Border).Background(theme.Surface).Gap(1).Dividers(1, theme.Border).Children(func() {
			inference := locales.LumenDisconnected()
			switch {
			case control.InferenceReady:
				inference = locales.LumenReady()
			case control.Connected:
				inference = locales.LumenPreparing()
			}
			settingRow(c, locales.LumenInference(), inference, func() {
				pill(c, controlPhaseLabel(control.Phase), controlPhaseColor(control.Phase))
			})
			ui.Row(c).Padding(12, 16).Gap(24).Children(func() {
				meta := func(label, value string) {
					if value == "" {
						value = "—"
					}
					ui.Column(c).Gap(2).Children(func() {
						ui.Text(c, label).FontSize(12).TextColor(theme.TextMuted)
						ui.Text(c, value).SingleLine()
					})
				}
				meta(locales.LumenVersion(), control.Version)
				meta(locales.LumenBackend(), control.Backend)
				sequence := ""
				if control.Sequence != 0 {
					sequence = fmt.Sprint(control.Sequence)
				}
				meta(locales.LumenSequence(), sequence)
			})

			if control.Connected {
				current := slices.Index(lumenPhaseOrder, control.Phase)
				ui.Row(c).Padding(12, 16).Gap(8).Label(locales.LumenLifecycle()).Children(func() {
					for i, phase := range lumenPhaseOrder {
						color := theme.Border
						text := theme.TextMuted
						switch {
						case i < current:
							color = theme.Success
						case i == current:
							color, text = theme.Accent, theme.Text
						}
						ui.Column(c.Key(string(phase))).Grow(1).Basis(0).Gap(6).Children(func() {
							ui.Box(c).Height(4).Radius(2).Background(color)
							ui.Text(c, controlPhaseLabel(phase)).FontSize(11).TextColor(text).SingleLine()
						})
					}
				})
			} else {
				emptyRow(c, locales.LumenControlWaiting(), locales.LumenControlWaitingDescription())
			}

			if download := control.Download; download != nil {
				ui.Column(c).Padding(12, 16).Gap(8).Children(func() {
					model := download.Model
					if model == "" {
						model = locales.LumenModel()
					}
					file := download.File
					if file == "" {
						file = locales.LumenPreparingDownload()
					}
					fraction, percent := -1.0, locales.LumenDownloading()
					if download.BytesTotal > 0 {
						fraction = min(1, float64(download.BytesDone)/float64(download.BytesTotal))
						percent = fmt.Sprintf("%.1f%%", fraction*100)
					}
					ui.Row(c).Gap(12).Children(func() {
						ui.Column(c).Grow(1).Basis(0).Gap(2).Children(func() {
							ui.Text(c, model)
							ui.Text(c, file).FontSize(12).TextColor(theme.TextMuted).SingleLine().TruncateMiddle()
						})
						ui.Text(c, percent)
					})
					ui.Progress(c, fraction).Label(locales.LumenDownloadProgress())
					ui.Row(c).Children(func() {
						done := formatBytes(download.BytesDone)
						if download.BytesTotal > 0 {
							done += " / " + formatBytes(download.BytesTotal)
						}
						ui.Text(c, done).FontSize(12).TextColor(theme.TextMuted).Grow(1)
						ui.Text(c, locales.LumenFilesProgress(int(download.FilesDone), int(download.FilesTotal))).FontSize(12).TextColor(theme.TextMuted)
					})
				})
			}

			if control.Error != nil {
				ui.Column(c).Padding(12, 16).Gap(2).Children(func() {
					ui.Text(c, locales.LumenControlFailed()).TextColor(theme.Danger)
					ui.Text(c, control.Error.Message).FontSize(12).TextColor(theme.TextMuted)
				})
			}

			ui.Row(c).Padding(12, 16).Children(func() {
				ui.Text(c, locales.LumenServices()).Grow(1)
				ui.Text(c, locales.LumenServicesReported(len(control.Services))).FontSize(12).TextColor(theme.TextMuted)
			})
			if len(control.Services) == 0 {
				hint := locales.LumenServicesOffline()
				if control.Connected {
					hint = locales.LumenServicesPending()
				}
				emptyRow(c, hint, "")
			}
			for _, service := range control.Services {
				detail := ""
				if service.Error != nil {
					detail = service.Error.Message
				}
				settingRow(c.Key("service:"+service.Service), serviceDisplayName(service.Service), detail, func() {
					pill(c, controlPhaseLabel(service.Phase), controlPhaseColor(service.Phase))
				})
			}
		})
	})
}

// serviceDisplayName names the model family behind a Hub service. These are
// model names, not the product's capability labels.
func serviceDisplayName(service string) string {
	switch service {
	case "siglip":
		return "SigLIP"
	case "face", "insightface":
		return "InsightFace"
	case "ocr", "ppocr":
		return "PP-OCR"
	case "bioclip":
		return "BioCLIP"
	}
	return service
}

func (p *Panel) lumenLogsView(c *ui.Context, lumen dto.LumenSnapshot) {
	l := &p.lumen
	theme := c.Theme()
	connected := lumen.ProcessPhase == dto.LumenRunning && lumen.Control.Connected
	now := p.now()
	// The logs are read anew when the Hub connects or the level changes,
	// then on a timer while the page shows.
	scope := fmt.Sprint(connected, l.logLevel)
	if scope != l.logsFor {
		l.logsFor, l.lastLogPoll = scope, now
		p.loadLumenLogs()
	}
	if connected {
		if now.Sub(l.lastLogPoll) >= lumenLogPoll {
			l.lastLogPoll = now
			p.loadLumenLogs()
		}
		c.After(lumenLogPoll - now.Sub(l.lastLogPoll))
	}

	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, locales.LumenLogs()).FontSize(13).Bold().TextColor(theme.TextMuted)
		ui.Text(c, locales.LumenLogsDescription()).FontSize(12).TextColor(theme.TextMuted)
		ui.Row(c).Gap(8).Children(func() {
			levels := []string{"TRACE", "DEBUG", "INFO", "WARN", "ERROR"}
			ui.Select(c.Key("lumen-log-level"), &l.logLevel, levels).Label(locales.LumenLogs()).Disabled(!lumen.Control.Connected)
			if ui.Button(c.Key("lumen-log-refresh"), locales.CommonRefresh()).
				Disabled(!lumen.Control.Connected || l.logsLoading).Clicked() {
				l.lastLogPoll = now
				p.loadLumenLogs()
			}
			if l.logsLoading {
				ui.Spinner(c).Label(locales.LumenLogsLoading())
			}
		})
		if l.logsError != "" {
			notice(c, toneDanger, locales.LumenLogsUnavailable(), l.logsError, "", nil)
		}
		logs := ui.Column(c).Radius(10).Border(1, theme.Border).Background(theme.Surface)
		switch {
		case !connected:
			logs.Children(func() { emptyRow(c, locales.LumenLogsOffline(), "") })
		case len(l.logs) == 0 && l.logsLoading:
			logs.Children(func() { emptyRow(c, locales.LumenLogsLoading(), "") })
		case len(l.logs) == 0:
			logs.Children(func() { emptyRow(c, locales.LumenLogsEmpty(), "") })
		default:
			logs.Height(280).Children(func() {
				ui.Scroll(c).Fill().Role(ui.RoleLog).Label(locales.LumenLogs()).Children(func() {
					ui.Column(c).Padding(10, 12).Gap(2).Selectable().Children(func() {
						for i, entry := range l.logs {
							color := theme.Text
							switch entry.Level {
							case "ERROR":
								color = theme.Danger
							case "WARN":
								color = theme.Warning
							case "TRACE", "DEBUG":
								color = theme.TextMuted
							}
							ui.Text(c.Key(i), logLine(entry)).Font("monospace").FontSize(11).TextColor(color)
						}
					})
				})
			})
		}
	})
}

func logLine(entry dto.LumenLogEntry) string {
	stamp := "--:--:--"
	if entry.TimeUnixMS != 0 {
		stamp = time.UnixMilli(entry.TimeUnixMS).Local().Format("15:04:05")
	}
	line := stamp + "  " + entry.Level + "  " + entry.Target + "  " + entry.Message
	if len(entry.Fields) == 0 {
		return line
	}
	keys := make([]string, 0, len(entry.Fields))
	for key := range entry.Fields {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	fields := make([]string, len(keys))
	for i, key := range keys {
		fields[i] = key + "=" + entry.Fields[key]
	}
	return line + " · " + strings.Join(fields, " ")
}
