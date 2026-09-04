---
title: "外置磁盘与网络存储"
description: "在断连、盘符、网络延迟和容器挂载条件下安全运行资源库。"
page_id: "admin/external-storage"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-22"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/repository_roots.go
- server/internal/storage/roe/controller/controller.go
- server/internal/storage/roe/changefeed
- server/docker-entrypoint.go
- desktop/internal/storage/controller.go
-->

# 外置磁盘与网络存储

外置盘和网络卷应在流明集启动或扫描前稳定挂载。流明集会同时检查资源库标记和卷身份；仅有同名空目录不会被解释为所有文件都已删除。

## Desktop

使用目录选择器登记外置位置，并让系统保持稳定挂载点。Windows 盘符变化或 macOS 卷名变化时，按 `.lumilioroot` 身份重新定位；不要新建空标记。

## Docker

把宿主机存储显式挂载到 `/data/storage` 或其规划子目录。对 NFS/CIFS 等网络文件系统，确认容器 UID `10001` 的读写语义、原子重命名、文件锁和断连行为。

## 扫描与断连

网络卷、可移动卷和不支持可靠日志的文件系统会保留受限的实时提示，并依赖周期完整验证。离线、权限错误、通知溢出或卷身份不匹配时，观察会暂停安全缺失确认，保留先前有效位置；已确认的新文件仍可逐步出现。可移动卷上的缺失还需要经过持久稳定等待。

持续断连仍会产生读取、派生和任务重试。维护期间先停止导入并取消或等待活动观察，再卸载磁盘；重新连接后保持原有 `.lumilioroot` 和 `.lumiliorepo` 身份，让恢复操作重新验证变化。

## 备份

RAID、镜像或网络存储可提高可用性，但不等于备份。误删、数据库损坏和应用级错误仍会复制到镜像，需要独立版本化备份。
