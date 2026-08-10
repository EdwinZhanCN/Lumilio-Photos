---
title: "OCR 文字搜索"
description: "查找照片画面中的可见文字，并理解 OCR 与文件元数据搜索的区别。"
page_id: "use/ocr-search"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/ocr_service.go
- server/internal/queue/jobs/types.go
- server/app/app.go
- web/src/features/settings
-->

# OCR 文字搜索

OCR 识别的是画面中的可见文字，不是文件名、描述或 EXIF。它需要 OCR 能力已启用、节点可用，并且目标媒体已经完成识别和本地索引写入。

适合的查询包括招牌、票据、文档页和屏幕截图中的词语。字体过小、模糊、倾斜、遮挡或语言模型不覆盖时，结果可能缺失。

## 排查顺序

1. 打开目标图片，确认文字在可见画面中。
2. 检查 Lumen Intelligence 的 OCR 任务是否“可用”。
3. 查看 Server Monitor 中 OCR 相关任务是否失败。
4. 对受影响资源库重建 OCR 索引。
5. 使用更短的关键词重试。

OCR 索引位于应用可重建状态中。数据库恢复流程会安排 OCR 重建，因此恢复后短时间内空结果是可解释状态。
