---
title: "逻辑媒体项目、组件与堆叠"
description: "区分一张媒体的多个文件组件和多个媒体项目的浏览分组。"
page_id: "concepts/media-items-and-stacks"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/asset_service.go
- server/internal/service/stack_service.go
- web/src/features/assets
-->

# 逻辑媒体项目、组件与堆叠

一个逻辑媒体项目可以包含 JPEG、RAW、实况照片视频或编辑版本等资产组件。服务会选择一个主资产用于浏览，并在组件被软删除或恢复后重新规范主资产。

堆叠则把多个逻辑媒体项目分为一组。手动堆叠来自用户选择，自动堆叠来自检测。堆叠不合并文件、不改变哈希，也不等同于重复项合并。

这解释了为什么界面选择一个卡片时，实际受影响资产数可能大于一；也解释了为什么把 JPEG 组件移入回收站后，RAW 可能成为主显示组件。

批量下载、分享和删除前应查看“项目数”和“资产数”，避免按缩略图数量推断文件数量。
