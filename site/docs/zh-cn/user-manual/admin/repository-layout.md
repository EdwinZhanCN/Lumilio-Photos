---
title: "资源库目录与布局策略"
description: "理解 `.lumiliorepo`、`inbox/`、`.lumilio/` 和 date、flat、cas 三种布局。"
page_id: "admin/repository-layout"
audience: "管理员、高级用户"
platform: "Desktop、Server"
baseline_commit: "c2fd359863d31d0f533dd38eddeecb6a997c3416"
last_verified: "2026-11-16"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/repocfg/repo_config.go
- server/internal/storage/layout/classes.go
- server/internal/storage/repo_provisioning.go
- server/internal/ingest/sourcing/materializer.go
-->

# 资源库目录与布局策略

每个资源库至少包含以下内容：

```text
repository/
├── .lumiliorepo
├── inbox/
└── .lumilio/
```

`.lumiliorepo` 保存资源库 UUID、名称、创建时间、存储策略和本地同名处理。`inbox/` 是通过流明集提交新媒体的受管理区域；`.lumilio/` 按类别保存人工创作的内容、回收站、可重建的缓存、进行中的工作、隔离内容和资源库日志，详见[资源库存储目录](../reference/storage-layout)。

## 布局策略

- `date` 按日期生成层级，便于人工理解。
- `flat` 把提交文件放在较平的结构中，同名处理更重要。
- `cas` 按内容标识组织，更适合由软件管理。

策略在创建资源库时写入标记。当前没有安全的资源库重命名或在线布局切换；不要通过手工编辑 `.lumiliorepo` 改变既有库。“从流明集中移除”会清理目录和应用私有状态，但保留 `.lumiliorepo`、原始媒体以及资源库拥有的磁盘文件。

## 同名处理

有效值只有 `rename` 和 `uuid`。内容重复先由完整哈希和大小检测；同名处理只解决不同内容映射到同一目标名称的问题。
