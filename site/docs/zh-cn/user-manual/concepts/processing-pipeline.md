---
title: "后台处理流水线"
description: "理解媒体从登记到可浏览、可播放和可搜索所经历的阶段。"
page_id: "concepts/processing-pipeline"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/sourcing
- server/internal/queue/jobs
- server/internal/processors/transcode_task.go
- server/internal/service/lumen_service.go
-->

# 后台处理流水线

媒体进入数据库后，流明集通过持久队列完成后续工作。

- 照片：元数据、缩略图，以及按设置安排的智能任务。
- 视频：元数据、缩略图、Web 转码和可选视频语义。
- 音频：元数据和 Web 转码。
- 智能能力：图像语义分析、OCR文字识别、人物识别和BioCLIP物种识别等独立任务。

基本可浏览状态与全部派生状态不是同一时刻。原件已经可用时，人物或语义仍可能排队；节点下线只应影响依赖它的任务。

任务使用持久队列和重试策略，因此进程重启不等于任务一定丢失。诊断应查看状态、尝试次数和错误，而不是只观察页面转圈。
