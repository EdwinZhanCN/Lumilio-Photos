---
title: "重复项与回收站"
description: "理解精确重复、视觉相似、合并元数据和当前软删除边界。"
page_id: "use/duplicates-trash"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/service/duplicate_service.go
- server/internal/service/asset_service.go
- server/internal/storage/directory_manager.go
- web/src/features/collections/flows/utilities/DuplicatesFlow.tsx
-->

# 重复项与回收站

重复项检测把同一资源库中的候选分为完全重复、视觉相似或混合组。管理员触发检测后，用户可以选择保留项、合并，或标记“不是重复”。

## 合并会做什么

默认合并策略把相册和标签并集到保留项，评分取较高值，喜欢取逻辑“或”，描述优先保留项并在为空时使用重复项。人脸默认不迁移；只有完全重复组才允许显式迁移人脸关系。

随后，重复项通过普通资产删除路径软删除。当前实现不会物理移动或删除原件，因此界面显示的“释放空间”“回收空间”或服务返回的 `recovered_bytes` 只是候选文件大小统计，并不代表磁盘已经释放。

## 回收站当前行为

普通“移入回收站”只在数据库中标记资产，原文件仍在原位置。恢复会清除软删除标记。当前公开 API 和界面没有经过验证的永久清空流程；资源库扫描还可能重新发现同一路径文件。

::: danger 不要把回收站当作磁盘清理工具
在产品修正生命周期语义前，必须通过文件系统和备份策略单独管理物理空间。手工删除原件前先确认数据库、相册、堆叠和组件关系，并保留可恢复副本。
:::

<!-- TODO(product-defect): duplicate_service 注释声称可移动到资源库回收区，但 AssetDeleter 实际调用的 DeleteAsset 只执行数据库软删除。可选方向：接通 storage TrashManager、提供永久清空与撤销，或删除所有“释放空间”文案。 -->
