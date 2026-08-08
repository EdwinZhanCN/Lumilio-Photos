---
title: "媒体一直显示处理中"
description: "判断是上传会话、提交、元数据、派生媒体还是可选智能任务未完成。"
page_id: "troubleshooting/processing-stuck"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/queue/jobs
- server/internal/sourcing
- server/internal/processors/transcode_task.go
- web/src/features/manage
-->

# 媒体一直显示处理中

“处理中”覆盖多个阶段。照片通常需要元数据和缩略图；视频还需要缩略图与 Web 转码；音频需要元数据与 Web 转码；智能任务可能在基础媒体可用后继续运行。

## 判断基础处理还是可选处理

- 原件可打开但没有人物或语义结果：基础处理成功，智能索引未完成。
- 原件存在但没有缩略图：检查缩略图任务和图像工具。
- 视频原件存在但网页不能播放：检查 FFprobe、FFmpeg 和转码任务。
- 上传会话仍显示上传中：检查浏览器到 Server 的传输，而不是 Lumen。

## 安全处理

先在 Server Monitor 查看对应任务是否等待、运行、计划重试或失败。不要连续重新处理同一资产；先修复磁盘、工具或节点原因，再对单个样本重试。
