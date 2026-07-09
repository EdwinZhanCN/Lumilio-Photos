package main

import "desktop/supervisor"

// nativeStrings localizes the desktop-native chrome (tray menu, startup status,
// failure dialogs) into the two supported languages. It is intentionally tiny and
// independent of the in-browser app translations the web BootstrapWizard owns.
var nativeStrings = map[string]map[string]string{
	"en": {
		"open":            "Open Lumilio Photos",
		"quit":            "Quit Lumilio Photos",
		"updateAvailable": "Update available: %s",
		"starting":        "Starting…",
		"setup":           "Setting up…",
		"running":         "Running — %s",
		"failTitle":       "Lumilio Photos failed to start",
		"alreadyTitle":    "Lumilio Photos is already running",
		"portTitle":       "Port 6680 is already in use",
		"portHint":        "Another Lumilio Photos instance or a local server is using port 6680. Quit that process, then relaunch Lumilio Photos.",
		"failStage":       "Failed while: %s",
		"logHint":         "Logs: %s",
		"aiEnable":        "Enable AI on This Machine…",
		"aiDisable":       "Turn Off Local AI",
		"aiRetry":         "Retry Enabling AI",
		"aiInstalling":    "AI: downloading…",
		"aiStarting":      "AI: starting (first run downloads models)…",
		"aiRunning":       "AI: running on this machine",
		"aiFailed":        "AI: failed to start — see logs",
	},
	"zh": {
		"open":            "打开 Lumilio Photos",
		"quit":            "退出 Lumilio Photos",
		"updateAvailable": "有新版本:%s",
		"starting":        "正在启动…",
		"setup":           "正在设置…",
		"running":         "运行中 — %s",
		"failTitle":       "Lumilio Photos 启动失败",
		"alreadyTitle":    "Lumilio Photos 已在运行",
		"portTitle":       "端口 6680 已被占用",
		"portHint":        "另一个 Lumilio Photos 实例或本机服务正在占用端口 6680。请退出该程序后重新启动 Lumilio Photos。",
		"failStage":       "失败于：%s",
		"logHint":         "日志目录：%s",
		"aiEnable":        "在本机启用 AI…",
		"aiDisable":       "停用本机 AI",
		"aiRetry":         "重试启用 AI",
		"aiInstalling":    "AI：正在下载…",
		"aiStarting":      "AI：正在启动（首次将下载模型）…",
		"aiRunning":       "AI：正在本机运行",
		"aiFailed":        "AI：启动失败——请查看日志",
	},
}

// stageStrings localizes each supervisor startup stage.
var stageStrings = map[string]map[string]string{
	"en": {
		supervisor.StagePreparing:      "Preparing…",
		supervisor.StageInitDB:         "Initializing database…",
		supervisor.StageStartingDB:     "Starting database…",
		supervisor.StageStartingServer: "Starting server…",
		supervisor.StageReady:          "Ready",
	},
	"zh": {
		supervisor.StagePreparing:      "正在准备…",
		supervisor.StageInitDB:         "正在初始化数据库…",
		supervisor.StageStartingDB:     "正在启动数据库…",
		supervisor.StageStartingServer: "正在启动服务…",
		supervisor.StageReady:          "就绪",
	},
}

// tr looks up a native-chrome string in the current language, falling back to
// English and then the key itself.
func (d *desktopApp) tr(key string) string {
	if m, ok := nativeStrings[d.lang]; ok {
		if s, ok := m[key]; ok {
			return s
		}
	}
	if s, ok := nativeStrings["en"][key]; ok {
		return s
	}
	return key
}

// trStage localizes a supervisor stage key.
func (d *desktopApp) trStage(stage string) string {
	if m, ok := stageStrings[d.lang]; ok {
		if s, ok := m[stage]; ok {
			return s
		}
	}
	if s, ok := stageStrings["en"][stage]; ok {
		return s
	}
	return stage
}
