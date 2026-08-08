---
title: "按平台补充排查"
description: "把相同症状映射到 macOS、Windows Desktop 和 Linux Docker 的宿主差异。"
page_id: "troubleshooting/platforms"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- desktop/internal/platform/paths.go
- desktop/internal/storage/controller.go
- server/docker-entrypoint.go
- deploy/compose/compose.yml
-->

# 按平台补充排查

## macOS Desktop

检查 Apple Silicon 架构、应用权限、外置卷是否重新挂载，以及用户配置目录中的 Desktop 日志。系统隐私权限可能影响目录选择和文件访问。

## Windows Desktop

检查 WebView2 Runtime、盘符是否变化、便携版解压目录权限和防火墙。外置磁盘从一个盘符变为另一个盘符时，先按存储身份重连，不要创建新空资源库。

## Linux Docker

从容器内部验证路径和网络。宿主机目录、UID/GID、SELinux/AppArmor 标签、NFS/CIFS 挂载选项和主机网络都会影响可见行为。

## 共同原则

使用同一份媒体副本复现，记录宿主路径与容器路径的映射。只在明确知道数据含义时修改权限或所有者；不要递归放宽整个媒体树来掩盖单个挂载错误。
