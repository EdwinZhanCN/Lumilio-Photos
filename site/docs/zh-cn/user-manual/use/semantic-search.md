---
title: "语义搜索"
description: "使用自然语言查找画面内容，并判断语义索引是否完整。"
page_id: "use/semantic-search"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/queue/jobs/types.go
- server/internal/service/settings_service.go
- server/internal/api/dto/settings_dto.go
- server/internal/service/embedding_service.go
-->

# 语义搜索

语义搜索把文字查询转换为向量，并与已经为媒体生成的图像或视频语义向量比较。它适合“海边日落”“红色汽车”这类画面描述，不保证像文件名或标签筛选一样精确。

## 使用前提

- 系统设置中图像语义和文本语义能力已启用；
- 至少有一个健康节点提供相应任务；
- 目标媒体已经完成语义索引；
- 当前资源库范围包含这些媒体。

视频语义会从视频采样有限帧，默认最多 8 帧；因此它是内容近似检索，不是逐帧完整理解。长视频阈值和场景阈值由管理员设置。

## 改善结果

尝试具体、可观察的名词和场景，而不是文件管理意图。结果不完整时，先检查任务可用性和失败队列，再由管理员对受影响范围重建索引。
