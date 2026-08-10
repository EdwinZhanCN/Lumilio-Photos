---
title: "扫描不到媒体"
description: "确认文件位于可扫描区域、容器真实可见且扩展名受支持。"
page_id: "troubleshooting/scan-no-media"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/scanner/scanner.go
- server/config/config.go
- server/internal/utils/file/validator.go
- server/internal/api/handler/repository_scan_handler.go
-->

# 扫描不到媒体

扫描只遍历资源库可扫描区域，并明确跳过 `.lumilio/**` 和 `inbox/**`。把文件放进这两个目录不会让扫描发现它们。

## 检查顺序

1. 在 Server 所在环境中确认文件真实存在；Docker 宿主机可见不代表容器可见。
2. 确认文件位于资源库目录内，但不在隐藏工作区或提交区。
3. 确认扩展名属于支持列表。
4. 检查资源库在线和可读权限。
5. 查看最新扫描的发现、更新、删除、跳过和错误统计。
6. 确认没有同一资源库扫描正在运行。

周期扫描默认等待文件稳定 5 秒。刚写入的大文件可能不会立即进入完整处理；手动扫描可以绕过这段等待，但不应在文件仍写入时频繁强制扫描。
