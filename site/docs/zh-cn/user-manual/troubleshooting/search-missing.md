---
title: "搜索结果缺失"
description: "排除范围、筛选、元数据和索引类型差异。"
page_id: "troubleshooting/search-missing"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/assets
- server/internal/search
- server/internal/service/ocr_service.go
- server/internal/api/handler/capabilities_handler.go
-->

# 搜索结果缺失

先清除当前搜索和筛选，并切换到包含目标媒体的资源库范围。相册、人物、事件、回收站和其他页面可能锁定基础范围。

## 文件名或元数据查询

打开目标媒体信息，确认字段已经提取。拍摄时间、相机、镜头、位置或标签为空时，对应筛选不会命中。

## OCR 或语义查询

检查任务是否启用且可用，并确认目标媒体已经索引。恢复数据库、变更能力或节点长期离线后，结果可能暂时不完整。

## 仍然缺失

记录目标资产 ID、预期字段、实际查询、范围和索引状态。不要只提供“搜索坏了”的截图；可复现查询和目标资产信息更有诊断价值。
