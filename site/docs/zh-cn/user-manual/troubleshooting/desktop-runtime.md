---
title: "Desktop 无法启动 Server"
description: "从 Desktop 主机状态、运行时清单、端口、恢复状态和日志定位失败。"
page_id: "troubleshooting/desktop-runtime"
audience: "所有用户"
platform: "macOS、Windows Desktop"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- desktop/main.go
- desktop/internal/runtime
- desktop/internal/platform/paths.go
- desktop/internal/runtime/runtimeconfig
-->

# Desktop 无法启动 Server

Desktop 内部监督 Server 和可选 Lumen 子进程。界面启动但 Server 不可用时，先打开 Desktop 设置查看主机、Server 和恢复状态。

## 排查顺序

1. 确认没有另一个 Desktop 实例或其他程序占用配置端口。
2. 检查 Desktop 设置是否处于仅本机、局域网或保留的自定义网络模式。
3. 查看应用配置目录下 `logs/` 与 `runtime/`，保留最近启动日志和运行时清单。
4. 如果界面显示“需要恢复”，先处理资源包、设置、运行时配置或 Lumen 安装日志，不要手工删除日志所指的 journal。
5. 暂时禁用可选 Lumen 启动，判断失败是否来自 Server 还是 Lumen 子进程。

Desktop 的 Server 与 SQLite 运行在同一产品运行时中。SQLite 通常能保护已经提交的事务，但强制结束仍会中断正在进行的导入、备份、恢复或文件提交；不要把它当作常规修复步骤。连续失败时先复制应用私有状态和媒体，再继续诊断。
