---
title: "搜索媒体"
description: "根据当前能力选择元数据、文字或语义搜索，并正确理解空结果。"
page_id: "use/search"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/asset_handler.go
- server/internal/search
- server/internal/service/ocr_service.go
- server/internal/api/handler/capabilities_handler.go
-->

# 搜索媒体

流明集的“搜索”不是单一索引。普通查询和筛选使用数据库中的文件与元数据字段；OCR 查询依赖 OCR 索引；语义查询依赖图像和文本嵌入，并要求相应 Lumen Intelligence 任务可用。

## 选择搜索方式

- 已知文件名、日期、设备、标签或评分：优先使用筛选器。
- 要查画面中可见文字：使用 OCR 能力。
- 用自然语言描述画面或寻找视觉语义：使用语义搜索。
- 已经知道人物、地点、事件或相册：从“合集”对应入口进入，范围更明确。

## 空结果不等于不存在

先确认当前资源库范围和筛选器，再检查索引是否完成。语义或 OCR 能力“已启用但不可用”时，不会因为设置开关打开就自动产生结果。

恢复数据库后，应用会安排 OCR 索引重建；其他智能索引也可能需要管理员手动重建。索引是可再生派生状态，不应被视作原件备份。
