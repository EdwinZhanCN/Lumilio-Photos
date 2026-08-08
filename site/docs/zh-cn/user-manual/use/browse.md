---
title: "浏览资源库"
description: "在资源库视图中切换布局、范围、排序和媒体查看器。"
page_id: "use/browse"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/assets/flows/browse/AssetBrowser.tsx
- web/src/features/assets/flows/library/AssetsFlow.tsx
- server/internal/service/asset_service.go
-->

# 浏览资源库

“资源库”是浏览全部可见媒体的主入口。页面支持舒展的自适应布局和可调整列数的紧凑布局，并按拍摄时间或最近添加排序。

## 基本浏览

- 使用滚动加载继续获取结果；页面显示“已无更多结果”时才代表当前查询结束。
- 点击缩略图打开媒体查看器；JPEG、RAW、实况照片、视频和音频可能显示不同组件或播放控件。
- 打开信息面板查看文件名、类型、大小、描述、位置、EXIF 和媒体专属字段。
- 使用喜欢、评分、描述、相册和标签为媒体补充图库元数据。

逻辑媒体项目可能包含多个文件组件，例如 JPEG+RAW 或实况照片。界面显示的主组件由服务端选择；把一个组件移入回收站后，剩余组件可能成为新的主组件。

## 当前范围

资源库页面可能带有资源库、相册、人物、标签、事件或实用工具范围。筛选器标题显示“此视图已锁定”时，基础范围来自当前页面，不能通过通用筛选器移除。
