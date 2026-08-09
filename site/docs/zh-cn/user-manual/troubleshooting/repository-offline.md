---
title: "资源库或磁盘显示离线"
description: "安全恢复挂载和可携带身份，不把移动误判为复制。"
page_id: "troubleshooting/repository-offline"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/repository_roots.go
- server/internal/storage/repo_relocate.go
- server/app/repository_control.go
- desktop/internal/runtime/repository.go
- desktop/internal/storage/controller.go
- server/internal/api/router.go
-->

# 资源库或磁盘显示离线

资源库在线状态取决于登记路径、存储位置身份和资源库身份是否一致。外置磁盘断开、盘符变化、网络卷未挂载或权限变化都可能使资源库离线。

## 先检查文件系统

1. 确认磁盘或网络卷在操作系统中真实可读写。
2. 确认原路径仍存在，且没有变成空目录或指向另一块盘。
3. 检查 `.lumilioroot` 和目标资源库 `.lumiliorepo` 仍存在。
4. 不要新建同名空标记来“修复”身份。

## Desktop

Desktop 可以通过目录选择器添加存储位置或打开已有资源库。也可以先在 Web 发起任务，再到 Desktop 的“设置 → 存储”中审核并在本机选择目录。遇到同一身份出现在不同路径时，界面会要求明确选择“作为移动后的原位置”或“作为独立资源库添加”；不要手工改写标记。

## Docker

Web 不能登记任意宿主机路径，但可以检查默认存储位置下的一级子目录，并明确打开带有效标记的资源库候选。应先恢复宿主机挂载到容器可见的预期位置；已有空目录只有在 Linux 中确认为真实挂载点时才可作为创建目标。不要把未挂载的空目录当作原存储启动大规模扫描。
