---
title: "组织媒体"
description: "用相册、堆叠、标签、喜欢、人物、地点和事件建立可恢复的组织结构。"
page_id: "use/organize"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- web/src/features/collections
- server/internal/db/repo/queries/albums.sql
- server/internal/service/asset_service.go
-->

# 组织媒体

流明集同时提供人工组织和派生组织。相册、标签、喜欢、评分和描述是用户明确维护的数据库关系；人物、地点、事件、重复项和分类视图部分依赖元数据或计算结果。

## “合集”不是一种对象

导航中的“合集”是聚合入口，包含相册、人物、地点、文件夹、标签和实用工具。代码中持久化的主要容器对象是相册，不存在一个独立可创建的 Collection 实体。

## 堆叠与媒体组件

手动堆叠把多个媒体项目作为一组浏览；自动堆叠检测由管理员触发。JPEG+RAW 或实况照片的组件关系属于逻辑媒体项目内部，不等同于手动堆叠。

## 备份影响

这些组织关系主要保存在 SQLite 数据库。只复制原件目录不能恢复相册、喜欢、人物命名、事件编辑和其他图库状态；应定期创建数据库快照。

<!-- TODO(terminology): 中文导航使用“合集”，部分历史文档使用 Collection。可选方向：改为“整理”或“浏览分类”。本文把“合集”限定为界面入口，不把它描述为数据模型。 -->
