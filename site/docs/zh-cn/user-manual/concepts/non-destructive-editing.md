---
title: "非破坏性编辑如何工作"
description: "理解 sidecar、原件和导出文件之间的关系。"
page_id: "concepts/non-destructive-editing"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/asset_handler.go
- server/internal/storage/directory_manager.go
- web/src/features/studio
-->

# 非破坏性编辑如何工作

工作室把编辑参数保存为版本化 sidecar，而不覆盖原件。打开媒体时，界面可以重新读取这些参数并重建编辑视图。

sidecar 位于资源库 `.lumilio/sidecars`，文件名基于资产 UUID。它与原件在同一资源库恢复边界中，但仍依赖数据库把资产 UUID、路径和媒体关系连接起来。

导出会把原件和当前参数渲染成一个新文件。新文件不是自动替换原件，也不会自动成为资源库资产；需要再次上传或扫描才会进入目录。

非破坏不等于无限历史。当前代码证明有可更新的版本 1 sidecar，没有证明面向用户保存任意多历史快照。
