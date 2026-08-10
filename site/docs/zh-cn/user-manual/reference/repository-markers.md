---
title: "`.lumilioroot` 与 `.lumiliorepo` 参考"
description: "列出两类可携带标记的当前字段和约束。"
page_id: "reference/repository-markers"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/rootcfg/root_config.go
- server/internal/storage/repocfg/repo_config.go
- server/internal/storage/repository_roots.go
-->

# `.lumilioroot` 与 `.lumiliorepo` 参考

## `.lumilioroot`

当前格式版本 `1.0`，字段包括：

- `version`；
- `id`：UUID；
- `name`；
- `created_at`。

它位于存储位置根目录，表示父位置身份。

## `.lumiliorepo`

字段包括：

- `version`；
- `id`：UUID；
- `name`；
- `created_at`；
- `storage_strategy`：`date`、`flat` 或 `cas`；
- `local_settings.handle_duplicate_filenames`：`rename` 或 `uuid`。

## 修改规则

这些文件由应用创建和验证。不要手工生成 UUID、复制后继续双写，或在线修改策略。标记文件不是完整配置：主机授权、路径、所有者和在线状态仍保存在数据库中。
