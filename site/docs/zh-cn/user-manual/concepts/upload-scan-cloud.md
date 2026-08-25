---
title: "上传、扫描与云导入为什么不同"
description: "比较三条接入管线的文件所有权、发现方式和失败恢复。"
page_id: "concepts/upload-scan-cloud"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-22"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/asset_handler.go
- server/internal/storage/roe/controller/controller.go
- server/internal/storage/roe/materializer
- server/internal/cloud
- server/internal/sourcing
-->

# 上传、扫描与云导入为什么不同

## 上传

流明集接收文件字节、写入资源库暂存、计算完整哈希，再按布局提交到 `inbox/`。暂存提交带有可恢复记录；进程中断后会继续完成已归属的提交，无法确认归属的字节会保留在隔离状态，不会被删除。提交完成后，文件通过统一的内容、资产和位置边界登记。

## 扫描

文件已经存在于资源库中。流明集的资源库观察引擎会分批发现普通媒体文件、验证稳定性并逐步计算完整哈希；它不负责从外部复制文件。观察范围包括 `inbox/`，但永远排除应用私有的 `.lumilio/`。操作请求会立即返回，发现、哈希和派生处理在后台分别推进。

## 云导入

提供方适配器先建立外部会话，再把远程媒体通过同一套可恢复暂存提交写入目标资源库。当前只实现 iCloud，且语义是单向导入，不是正式定义的双向同步。

三者最终都使用相同的精确内容身份：同一所有者的相同字节只对应一个资产，但可以保留多个实际文件位置。三条路径前半段的失败证据和恢复位置仍然不同，排障时必须先说明使用了哪一种方式。
