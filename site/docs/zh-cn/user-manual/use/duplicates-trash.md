---
title: "重复项与回收站"
description: "理解精确重复、视觉相似、合并元数据，以及资源库回收站与缺失文件的行为。"
page_id: "use/duplicates-trash"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "9f10907a42cc5d98ba239aee2888ff38c58f3f83"
last_verified: "2026-10-18"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/duplicate_service.go
- server/internal/storage/trash
- server/internal/storage/scan/lifecycle.go
- server/config/config.go
- web/src/features/assets/flows/lifecycle
- web/src/features/collections/flows/utilities/DuplicatesFlow.tsx
-->

# 重复项与回收站

重复项检测把同一资源库中的候选分为完全重复、视觉相似或混合组。管理员触发检测后，用户可以选择保留项、合并，或标记“不是重复”。

## 合并会做什么

默认合并策略把相册和标签并集到保留项，评分取较高值，喜欢取逻辑“或”，描述优先保留项并在为空时使用重复项。人脸默认不迁移；只有完全重复组才允许显式迁移人脸关系。

随后，未保留的重复项会移入所在资源库的回收站：文件离开原目录，保留期满或被永久删除后磁盘空间才真正释放。界面显示的“可回收空间”是这些文件的大小合计。

## 回收站

“移入回收站”会把文件从你的目录移到该资源库自己的回收站（资源库私有目录 `.lumilio/trash`），不是操作系统的回收站。恢复会把文件放回原路径，相册、评分、喜欢、标签、描述和人物都保持不变。

- 管理员可以在**存储**页按资源库打开回收站；所有用户可以在 合集 → 回收站 查看自己移入的项目。
- 回收站中的文件默认保留 30 天后永久删除。Docker 部署通过 `server.toml` 的 `repository_trash.retention_days` 调整，Desktop 固定为 30 天。**永久删除**会立即移除文件。
- 删除之前创建的数据库备份，恢复后会把被删除的文件列在回收站中，而不是报告为缺失。

## 缺失的文件

文件在流明集之外消失时（移动硬盘被拔掉、在访达或资源管理器里被删除），这张照片会从浏览、搜索、相册和“存储”页的计数中隐藏，并出现在该资源库的**缺失**视图中。相同的文件回到原路径后，照片连同元数据一起恢复。**移除缺失项**会忘记这些缺失文件，不会碰磁盘；移除后没有任何文件的照片，其元数据也一并移除。

在其他应用中原地改写文件（包括只改写 XMP 或 EXIF 元数据）不会产生“缺失”：照片保留相册、评分、喜欢、标签、描述和人物，缩略图重新生成。

::: warning 回收站不是备份
回收站用于撤销误操作。RAID、同步目录和云盘各自只覆盖一部分风险，都不能替代独立且经过恢复验证的备份。
:::
