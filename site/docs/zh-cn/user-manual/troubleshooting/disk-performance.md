---
title: "磁盘空间不足或性能下降"
description: "分别观察原件、派生文件、暂存、数据库快照和模型缓存增长。"
page_id: "troubleshooting/disk-performance"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/directory_manager.go
- server/internal/service/settings_service.go
- server/internal/processors/transcode_task.go
- server/internal/service/asset_service.go
-->

# 磁盘空间不足或性能下降

空间增长不只来自原件。检查以下位置：资源库 `inbox/` 和普通媒体、`.lumilio/assets` 派生文件、`.lumilio/staging` 失败或未完成暂存、应用私有 `backups`、云会话，以及 Lumen 模型缓存。

## 空间不足

先停止新增导入和大规模索引。不要直接删除 `.lumilio/` 或 SQLite 文件。确认服务器端数据库快照及其 manifest 已配对复制到独立位置，并确认媒体备份后，再处理可重建派生文件或过期本地快照。

## 性能下降

- 大量视频转码、人物重建或语义索引会竞争 CPU/GPU 和磁盘；
- 网络卷延迟会影响扫描和原件读取；
- `cas` 布局不提升所有文件系统的浏览性能；
- 自动硬件加速可能回退到软件处理，应查看运行时信息和转码日志；
- 失败任务反复退避会增加队列观察噪声。

当前回收站和重复项合并不会物理释放原件空间，不能作为容量回收手段。
