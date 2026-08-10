---
title: "媒体信息与组件"
description: "区分原件字段、提取元数据、资源库元数据和逻辑媒体组件。"
page_id: "use/media-details"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/assets/flows/viewer/info
- server/internal/service/asset_service.go
- server/internal/db/repo/queries
-->

# 媒体信息与组件

媒体信息面板同时展示来自原文件的提取结果和流明集数据库中的可编辑字段。文件名、大小、编码、拍摄参数和原始 EXIF 通常来自文件；描述、喜欢、评分、相册和标签关系由流明集保存。

## 组件模型

一个逻辑媒体项目可以由多个资产组件组成。当前界面明确处理 JPEG、RAW 和实况照片等构成；查看器可以在可用组件间切换。堆叠则是更高一层的用户或系统分组，不等同于 JPEG+RAW 组件关系。

## EXIF

“查看 EXIF”读取服务已保存或提取的原始 EXIF 数据。显示“尚未保存原始 EXIF 数据”时，不应据此断言原文件没有任何元数据；应结合原文件和提取任务状态判断。

## 修改描述

描述是数据库字段，修改不会写回原件。备份原件但不备份数据库，会丢失这些资源库级修改。
