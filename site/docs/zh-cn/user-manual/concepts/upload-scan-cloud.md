---
title: "上传、扫描与云导入为什么不同"
description: "比较三条接入管线的文件所有权、发现方式和失败恢复。"
page_id: "concepts/upload-scan-cloud"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/asset_handler.go
- server/internal/storage/scanner/scanner.go
- server/internal/cloud
- server/internal/sourcing
-->

# 上传、扫描与云导入为什么不同

## 上传

流明集接收文件字节、写入资源库暂存、计算完整哈希、检查同库内容重复，再按布局提交到 `inbox/`。它控制从传输到登记的全过程。

## 扫描

文件已经存在于资源库可扫描目录。流明集遍历、验证扩展名、读取元数据并把路径登记到数据库。它不负责把文件从外部复制进来，也不扫描 `.lumilio/` 或 `inbox/`。

## 云导入

提供方适配器先建立外部会话，再把远程媒体作为导入任务写入目标资源库。当前只实现 iCloud，且语义是单向导入，不是正式定义的双向同步。

三者最终都进入相似的元数据和派生处理，但前半段的失败证据和恢复位置不同。排障时必须先说清使用了哪一条路径。
