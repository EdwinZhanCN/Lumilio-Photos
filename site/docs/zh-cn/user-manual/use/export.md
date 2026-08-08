---
title: "导出与下载"
description: "区分下载原件、下载选择结果和工作室渲染导出。"
page_id: "use/export"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/internal/api/handler/asset_handler.go
- server/internal/api/handler/share_link_handler.go
- server/internal/utils/imaging/process.go
- web/src/features/studio/flows/editor/export/ExportPanel.tsx
-->

# 导出与下载

流明集有多种把媒体取出系统的方式：下载原件、批量下载当前选择，以及在工作室中把编辑结果渲染为新文件。它们不应混为一种操作。

## 原件下载

原件端点返回登记资产对应的原始文件。公开分享的单文件原件端点要求同时开启“允许下载”和“包含原件”；但当前分享 ZIP 下载只检查“允许下载”，视频或音频 Web 播放端点在派生文件缺失时也会回退到原件。原件可能保留完整 EXIF、位置和原始编码数据。

## 重新编码导出

Server 导出端点可以把源图重新编码为 JPEG、PNG、WebP 或 AVIF。工作室界面则根据当前编辑合成结果导出；实际可选格式以界面为准。

## 批量下载

批量下载会对当前实际受影响的资源启动任务。堆叠或多组件项目可能使下载文件数与界面选择数不同，应先检查确认信息和结果压缩包。

导出文件不会自动回到资源库。需要把成品重新纳入流明集时，应作为新文件上传或放入可扫描区域。
