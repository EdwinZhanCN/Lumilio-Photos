---
title: "搜索与索引"
description: "解释数据库筛选、OCR、语义向量和派生分类为何会产生不同结果。"
page_id: "concepts/search-and-indexes"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/search
- server/internal/service/ocr_service.go
- server/internal/service/lumen_service.go
- server/internal/service/face_service.go
- server/internal/event
-->

# 搜索与索引

流明集有多种检索表面：

- 数据库字段筛选：文件名、日期、类型、评分、标签、设备、位置等；
- OCR 索引：画面可见文字；
- 语义向量：画面和文本描述的相似度；
- 人物和事件：由人脸、时间与地点派生的分组；
- BioCLIP：生物主题分类。

它们具有不同更新时机和可用依赖。数据库字段存在不代表语义索引完成；节点在线也不代表 OCR 已经覆盖全部媒体。

索引通常可重建，但人工命名、隐藏、相册和描述位于数据库，不能通过重新推理可靠恢复。恢复顺序应先数据库和原件，再派生索引。
