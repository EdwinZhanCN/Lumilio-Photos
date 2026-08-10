---
title: "人物"
description: "查看、命名、隐藏、合并人物聚类，并修正错误人脸关联。"
page_id: "use/people"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/face_service.go
- server/internal/api/router.go
- web/src/features/people
- web/src/locales/zh/translation.json
-->

# 人物

人物页面依赖 Lumen Intelligence 的人脸检测与嵌入，以及服务端的聚类结果。没有可用人脸能力或索引尚未完成时，页面可以为空。

## 可执行操作

- 为未命名人物设置名称；
- 选择人物封面；
- 从默认网格隐藏或恢复人物；
- 把多个人物合并到目标人物；
- 把选中人脸移动到另一个人物；
- 从人物中移除错误人脸；
- 重新构建人物聚类。

隐藏只改变默认显示，不删除照片或人脸关联。移除人脸只解除人物关系，原始照片不变。合并人物会改变人物筛选结果，因此应先确认目标人物。

人物名称、隐藏状态、封面和人工修正保存在数据库中；重建聚类前应有近期数据库快照。
