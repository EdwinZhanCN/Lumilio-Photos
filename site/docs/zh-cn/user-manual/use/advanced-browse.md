---
title: "筛选、排序与批量操作"
description: "组合文件名、日期、类型、位置、设备、标签和媒体构成筛选，并安全执行批量操作。"
page_id: "use/advanced-browse"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/assets/components
- web/src/features/assets/flows/browse/AssetBrowser.tsx
- server/internal/api/dto/asset_dto.go
-->

# 筛选、排序与批量操作

资源库筛选器可以组合日期、类型、文件名、喜欢、评分、标签、相机品牌、镜头、位置、分组状态和媒体构成。文件名支持包含、开头、结尾和匹配等模式；位置筛选可以通过边界框或中心点加半径生成。

## 批量选择

进入选择模式后，页头会同时显示已选择的界面项目数和实际受影响的资源数。一个界面项目可能代表堆叠或多组件媒体，因此两者不一定相同。

当前批量操作包括添加到相册、添加标签、喜欢或取消喜欢、评分、下载、分享、手动堆叠和移入回收站。提交前阅读确认框中的受影响数量。

## 使用建议

1. 先应用筛选并抽查结果。
2. 进入选择模式，逐步扩大选择范围。
3. 对不可轻易逆转的下载外操作，先在少量媒体上验证。
4. 批量移入回收站后，到回收站确认结果；当前普通删除不会物理释放原件空间。

搜索、筛选和排序只改变查询结果，不会改写原件。评分、喜欢、描述、标签和相册关系保存在数据库中，必须通过数据库快照保护。
