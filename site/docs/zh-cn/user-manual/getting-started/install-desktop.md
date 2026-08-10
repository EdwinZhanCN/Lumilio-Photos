---
title: "安装 Desktop"
description: "在受支持的 macOS 或 Windows 电脑上安装并启动流明集 Desktop。"
page_id: "getting-started/install-desktop"
audience: "所有用户"
platform: "macOS、Windows Desktop"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- .github/workflows/release-desktop.yml
- desktop/main.go
- desktop/internal/platform/paths.go
- desktop/README.md
-->

# 安装 Desktop

Desktop 是个人电脑上的完整运行方式，不只是指向远程 Server 的壳。它会管理应用私有状态、启动内置 Server，并在浏览器式界面中提供流明集功能。

## macOS

当前发布产物是 Apple Silicon DMG。打开 DMG 后将应用放入“应用程序”，再从系统启动。首次启动时，macOS 可能要求确认来自已下载应用的运行权限；只应使用与项目发布记录对应的构建产物。

## Windows

当前发布产物面向 x64，可选择安装包或便携压缩包。Desktop 界面依赖 Microsoft Edge WebView2 Runtime；如果应用窗口无法显示，应先确认 WebView2 可用，而不是删除媒体或数据库。

## 第一次启动后

1. 打开 Desktop 设置，确认 Server 运行状态。
2. 使用默认仅本机网络模式完成首次设置。
3. 不要在尚未创建管理员和主资源库时添加大量外置路径。
4. 按[第一次设置](./first-setup.md)继续。

## 应用状态位置

Desktop 使用操作系统提供的用户配置目录，并在其下创建 `Lumilio Photos`。其中可能包含设置、运行时清单、密钥、日志、Lumen 文件和更新状态。不要把这个目录误当作媒体原件目录，也不要在应用运行时手工编辑其内容。
