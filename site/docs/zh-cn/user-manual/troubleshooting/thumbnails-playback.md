---
title: "缩略图缺失或媒体无法播放"
description: "分别检查原件读取、图像处理和 Web 兼容派生文件。"
page_id: "troubleshooting/thumbnails-playback"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/directory_manager.go
- server/internal/queue/jobs
- server/internal/utils/imaging
- server/internal/processors/transcode_task.go
- server/config/schema/lumilio-server.schema.json
-->

# 缩略图缺失或媒体无法播放

缩略图和 Web 播放文件位于资源库 `.lumilio/assets/`，属于可再生派生数据。缺失派生文件不应通过替换原件解决。

## 照片没有缩略图

确认原件可读、扩展名受支持，并查看图像解码或缩略图任务错误。RAW 文件依赖运行镜像中的相关解码库；损坏或特殊编码可能需要最小复现文件。

## 视频或音频无法播放

浏览器不一定直接支持原始编码。流明集使用 FFprobe 读取信息，并使用 FFmpeg 生成 Web 派生文件。检查工具路径、硬件加速设置、转码日志和磁盘空间。

## 重建

修复根因后，对少量资产执行重新处理。批量删除 `.lumilio/assets` 会使全部派生文件失效并造成大规模重建，不是首选排障方法。
