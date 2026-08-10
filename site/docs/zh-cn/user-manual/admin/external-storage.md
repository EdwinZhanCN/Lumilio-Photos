---
title: "外置磁盘与网络存储"
description: "在断连、盘符、网络延迟和容器挂载条件下安全运行资源库。"
page_id: "admin/external-storage"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/repository_roots.go
- server/internal/storage/scanner/scanner.go
- server/docker-entrypoint.go
- desktop/internal/storage/controller.go
-->

# 外置磁盘与网络存储

外置盘和网络卷必须在流明集启动或扫描前稳定挂载。路径存在但指向空目录，比明确离线更危险，因为应用可能把它当作可访问位置。

## Desktop

使用目录选择器登记外置位置，并让系统保持稳定挂载点。Windows 盘符变化或 macOS 卷名变化时，按 `.lumilioroot` 身份重新定位；不要新建空标记。

## Docker

把宿主机存储显式挂载到 `/data/storage` 或其规划子目录。对 NFS/CIFS 等网络文件系统，确认容器 UID `10001` 的读写语义、原子重命名、文件锁和断连行为。

## 扫描与断连

完整扫描遇到部分遍历错误时会避免大规模缺失协调，但持续断连仍会产生读取、派生和任务失败。维护期间先停止导入和扫描，再卸载磁盘。

## 备份

RAID、镜像或网络存储可提高可用性，但不等于备份。误删、数据库损坏和应用级错误仍会复制到镜像，需要独立版本化备份。
