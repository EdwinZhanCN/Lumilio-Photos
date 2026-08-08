---
title: "工作室与非破坏性编辑"
description: "保存裁剪、显影、画框、文字和深度相关编辑，而不改写原件。"
page_id: "use/studio"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/api/handler/asset_handler.go
- server/internal/storage/directory_manager.go
- web/src/features/studio
- server/internal/utils/imaging/process.go
-->

# 工作室与非破坏性编辑

工作室提供裁剪、显影、画框、文字和深度估计等编辑工具。保存编辑时，服务端把版本化 sidecar 写入资源库 `.lumilio/sidecars/<asset-id>.lumilio-sidecar`；原件保持不变。

## 编辑状态

sidecar 记录编辑参数、来源信息和更新时间。它属于资源库可恢复工作区，应随媒体存储备份。数据库仍保存资产身份和关系，因此完整恢复还需要数据库快照。

## 深度估计

深度相关能力需要相应计算服务可用。节点不可用时，其他不依赖该任务的编辑仍可继续；不要把一次深度任务失败理解为原件损坏。

## 保存与导出

“保存”保存编辑描述，不生成替代原件。“导出”把当前编辑渲染为新文件供下载。导出前检查尺寸、格式和元数据保留行为；PNG 不承载常见照片 EXIF，其他格式的元数据保留也应视为尽力而为。

<!-- TODO(studio-history): 当前代码证明存在单份版本化 sidecar，但没有证明为用户提供完整多版本历史和回滚时间线。可选术语：编辑记录、编辑状态；本文不承诺“无限历史”。 -->
