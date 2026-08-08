---
title: "地点与地图"
description: "根据媒体坐标浏览地图，并区分原始坐标与反向地理编码。"
page_id: "use/places"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/location_handler.go
- server/internal/service/location_service.go
- server/internal/api/handler/share_link_handler.go
- server/config/schema/lumilio-server.schema.json
- web/src/features/assets/map
-->

# 地点与地图

地图使用媒体记录中的经纬度生成点位和范围查询。原文件没有坐标、元数据尚未提取，或当前范围没有带坐标媒体时，地图会为空。

反向地理编码是可选能力，默认运行时配置可以保持关闭。启用时可使用配置的 Nominatim 服务把坐标转换为可读地点；关闭它不会删除坐标，也不会阻止地图按经纬度显示。

## 隐私提醒

位置是敏感元数据。创建公开分享前，应确认分享范围、下载策略和媒体派生状态；当前“包含原件”开关没有覆盖所有 ZIP 与 Web 媒体回退路径，原件本身可能包含比网页展示更多的 EXIF 信息。

## 排查

先在单个媒体信息中确认经纬度，再检查位置重建或地理编码任务。不要通过修改原件来“刷新”地图，除非你确实需要修正文件元数据并已保留备份。
