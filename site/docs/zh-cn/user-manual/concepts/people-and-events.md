---
title: "人物与事件的派生和人工修正"
description: "理解算法分组、人工命名以及重建可能带来的变化。"
page_id: "concepts/people-and-events"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/face_service.go
- server/internal/event
- web/src/features/people
- web/src/features/events
-->

# 人物与事件的派生和人工修正

人物由人脸检测、嵌入和聚类形成。用户可以命名、隐藏、设置封面、合并人物，以及移动或移除人脸。这些人工修正使人物不再只是模型输出。

事件根据拍摄时间和位置组织媒体。用户可以重命名、隐藏、合并、拆分和调整成员。把媒体从事件移除不会删除原件。

“重建”会重新运行派生逻辑。它可能补齐新媒体，也可能改变边界或聚类。重建前创建数据库快照，重建后检查人工决策是否仍符合预期。

人物和事件结果都不应作为身份或事实认证依据；它们是帮助整理媒体的可修正视图。
