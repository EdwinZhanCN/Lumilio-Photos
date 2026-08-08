---
title: "相册与堆叠"
description: "创建相册、维护顺序与封面，并正确使用手动堆叠。"
page_id: "use/albums"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/internal/service/album_service.go
- server/internal/service/stack_service.go
- server/internal/service/share_link_service.go
-->

# 相册与堆叠

相册是用户拥有的持久化媒体集合。可以创建、重命名、填写描述、设置封面、添加或移除媒体，并调整相册内部顺序。删除相册关系不会删除其中原件。

## 创建相册

可以从相册页创建空相册，也可以在资源库批量选择后“添加到相册”。一个媒体可以属于多个相册。

## 堆叠

选择多个媒体后可以创建手动堆叠。堆叠用于在资源网格中把相关项目折叠显示，例如连拍或不同版本；它不合并文件，也不回收空间。

管理员还可以触发自动堆叠检测。自动结果应抽查后再依赖，因为规则依据元数据和检测逻辑，而不是用户意图。

## 相册与分享

从相册创建分享时，分享服务会把当时的资产 ID 解析为快照。之后相册成员变化不会自动保证已有分享同步变化；管理分享前应检查实际公开内容。
