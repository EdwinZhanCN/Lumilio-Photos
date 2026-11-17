package panel

import (
	"desktop/internal/control/dto"
	"desktop/internal/locales"

	"github.com/egoist/mygo/ui"
)

func (p *Panel) serverView(c *ui.Context) {
	runtime := p.snapshot.Runtime
	capabilities := runtime.Capabilities
	busy := p.runtimeOp.loading()
	pageHeading(c, locales.ServerTitle(), locales.ServerDescription())
	section(c, locales.ServerRuntime(), func() {
		settingRow(c, locales.ServerStatus(), locales.ServerStatusDescription(), func() {
			statusBadge(c, runtime.Presentation)
			if capabilities.CanStartRuntime {
				if ui.Button(c, locales.CommonStart()).Disabled(busy).Clicked() {
					p.runtimeAction("start")
				}
			}
			switch {
			case capabilities.CanRetryCleanupRuntime:
				if ui.Button(c, locales.CommonRetryCleanup()).Disabled(busy).Clicked() {
					p.runtimeAction("retry")
				}
			case capabilities.CanStopRuntime:
				if ui.Button(c, locales.CommonStop()).Disabled(busy).Clicked() {
					p.runtimeAction("stop")
				}
			}
			if capabilities.CanRestartRuntime {
				if trackedButton(c, &p.runtimeOp, locales.CommonRestart(), locales.ServerRestarting(), locales.ServerRestarted(), false, false).Clicked() {
					p.runtimeAction("restart")
				}
			}
		})
	})

	switch {
	case p.runtimeOp.err != "":
		notice(c, toneDanger, locales.ActionNoticeNeedsAttention(locales.DockServer()), p.runtimeOp.err, "", nil)
	case runtime.Phase == dto.RuntimeFailed:
		label, action := "", func() {}
		if capabilities.CanRetryCleanupRuntime {
			label, action = locales.CommonRetryCleanup(), func() { p.runtimeAction("retry") }
		}
		notice(c, toneDanger, locales.ActionNoticeNeedsAttention(locales.DockServer()), locales.ServerFailedMessage(), label, action)
	}

	if p.draft.runtime == nil {
		loadingRow(c, locales.ServerLoadingSettings())
		return
	}
	p.runtimeConfigView(c)
}

func (p *Panel) runtimeAction(action string) {
	p.runtimeOp.begin()
	id, version := p.requestID("runtime"), p.snapshot.Runtime.Version
	p.async(func() func() {
		var receipt dto.OperationReceipt
		var err error
		switch action {
		case "start":
			receipt, err = p.services.Runtime.Start(id, version)
		case "stop":
			receipt, err = p.services.Runtime.Stop(id, version)
		case "restart":
			receipt, err = p.services.Runtime.Restart(id, version)
		default:
			receipt, err = p.services.Runtime.RetryCleanup(id, version)
		}
		return func() { p.runtimeOp.resolve(receipt, err, p.snapshot.Operations, p.now()) }
	})
}

// runtimeConfigView edits the small projection of the Server manifest the
// Desktop owns. The complete manifest stays a file the person can open.
func (p *Panel) runtimeConfigView(c *ui.Context) {
	d := &p.draft
	busy := d.busy()
	section(c, locales.RuntimeWorkspaceConfiguration(), func() {
		settingRow(c, locales.RuntimeWorkspaceNetworkAccess(), locales.RuntimeWorkspaceNetworkDescription(), func() {
			p.networkSelect(c, "server-network", busy)
		})
	})
	section(c, locales.RuntimeWorkspaceProcessing(), func() {
		settingRow(c, locales.RuntimeWorkspaceHardwareAcceleration(), locales.RuntimeWorkspaceHardwareAccelerationDescription(), func() {
			acceleration := d.runtime.HardwareAcceleration
			if selectOf(c, "server-acceleration", &acceleration, []option[string]{
				{"auto", locales.RuntimeWorkspaceAuto()},
				{"videotoolbox", locales.RuntimeWorkspaceVideoToolbox()},
				{"vaapi", locales.RuntimeWorkspaceVaapi()},
				{"nvenc", locales.RuntimeWorkspaceNvenc()},
				{"qsv", locales.RuntimeWorkspaceQsv()},
				{"none", locales.RuntimeWorkspaceSoftwareOnly()},
			}, locales.RuntimeWorkspaceAcceleration(), busy) {
				p.updateRuntime(func(settings *dto.RuntimeConfigSettings) { settings.HardwareAcceleration = acceleration })
			}
		})
		settingRow(c, locales.RuntimeWorkspaceLoggingLevel(), locales.RuntimeWorkspaceLoggingLevelDescription(), func() {
			level := d.runtime.LoggingLevel
			if selectOf(c, "server-logging", &level, []option[string]{
				{"debug", locales.RuntimeWorkspaceDebug()},
				{"info", locales.RuntimeWorkspaceInfo()},
				{"warn", locales.RuntimeWorkspaceWarnings()},
				{"error", locales.RuntimeWorkspaceErrors()},
			}, locales.RuntimeWorkspaceLoggingLevel(), busy) {
				p.updateRuntime(func(settings *dto.RuntimeConfigSettings) { settings.LoggingLevel = level })
			}
		})
	})
	section(c, locales.RuntimeWorkspaceAdvanced(), func() {
		settingRow(c, locales.RuntimeWorkspaceManifest(), locales.RuntimeWorkspaceManifestDescription(), func() {
			if ui.Button(c.Key("server-manifest"), locales.CommonOpen()).Clicked() {
				p.async(func() func() {
					err := p.services.Desktop.OpenRuntimeManifest()
					return func() {
						if err != nil {
							p.toastError(locales.RuntimeWorkspaceManifestFailed(), errorText(err))
						}
					}
				})
			}
		})
	})
}
