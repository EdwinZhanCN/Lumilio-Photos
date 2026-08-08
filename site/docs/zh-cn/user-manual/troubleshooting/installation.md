---
title: "安装失败"
description: "按 macOS、Windows 和 Docker 的实际交付边界定位安装问题。"
page_id: "troubleshooting/installation"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- .github/workflows/release-desktop.yml
- server/docker-entrypoint.go
- server/Dockerfile
-->

# 安装失败

## macOS Desktop

确认设备是 Apple Silicon。当前发布工作流不产出 Intel macOS DMG。若系统阻止打开，记录系统提示和构建来源，不要通过关闭整个系统安全机制来绕过。

## Windows Desktop

确认系统为 x64，并安装可用的 Microsoft Edge WebView2 Runtime。应用窗口空白或无法创建时，先检查 WebView2 和 Desktop 日志，而不是删除应用状态。

## Docker Server

依次确认：镜像架构匹配、`/data/storage` 和 `/data/app-state` 都已挂载、两个路径对容器 UID/GID `10001` 可写、端口未冲突。入口脚本会在挂载不可写时主动停止，并打印宿主机路径与权限建议。

## 成功标志

服务健康检查通过，应用能返回初始化状态。此时再处理浏览器、账户和主资源库问题；容器“正在运行”本身不足以证明应用就绪。
