---
title: "扫描已有目录"
description: "把资源库可扫描区域中的普通文件登记到流明集，并理解删除协调。"
page_id: "use/scan"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/storage/scanner/scanner.go
- server/internal/storage/repo_reconcile.go
- server/config/config.go
- server/internal/db/repo/queries
-->

# 扫描已有目录

扫描遍历资源库目录中的普通媒体文件，但跳过 `.lumilio/**` 和 `inbox/**`。它适合已经把目录放在资源库可见范围内的场景，不负责把宿主机任意路径自动挂载进 Docker。

## 手动与周期扫描

默认运行时允许周期扫描，默认间隔 300 秒，稳定等待 5 秒；手动或强制扫描不使用这段稳定等待。每个资源库同一时间只运行一个扫描，重复请求可能被跳过或排队。

完整扫描可以发现新增、更新和缺失文件。遍历发生部分错误时，服务不会据此执行全量“缺失即删除”协调，避免因临时挂载问题误判大量文件消失。

## 文件移动与删除

扫描可以通过完整哈希和大小协调部分移动场景。磁盘上缺失的已登记文件会在数据库中软删除；磁盘文件重新出现时，当前 upsert 路径可能使它再次进入资源库。

<!-- TODO(trash-scan-semantics): 普通“移入回收站”只软删除数据库记录，而扫描可重新发现同一路径。可选方向：持久化用户删除意图、扫描忽略 tombstone、或真正移动到资源库回收区。当前文档提醒用户不要用扫描作为回收站清理工具。 -->
