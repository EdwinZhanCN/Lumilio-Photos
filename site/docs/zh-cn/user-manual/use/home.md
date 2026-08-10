---
title: "首页与统计"
description: "理解首页画廊、时空轨迹和媒体统计的来源与限制。"
page_id: "use/home"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/home
- server/internal/api/handler/asset_handler.go
- server/internal/service/featured_selector.go
- server/internal/api/handler/stats_handler.go
- server/internal/api/handler/location_handler.go
- web/src/features/repositories/flows/browse-scope/useBrowseScope.ts
- web/src/locales/zh/translation.json
-->

# 首页与统计

首页提供“画廊”和“数据统计”两类入口。画廊展示服务端从照片候选中按质量、时间和多样性约束确定性选出的精选照片；默认候选时间窗是近 3650 天，同一天使用相同种子。统计页根据已经提取的媒体元数据生成相机镜头组合、焦段分布、拍摄活跃热力图和拍摄时段分布。

“时空轨迹”只统计具有可用经纬度的媒体。没有地理信息时，地图为空并不代表资源库或扫描失败；需要先确认原件包含 GPS 信息，或者位置索引已经完成。

首页、地图和统计使用页头的当前浏览范围，可以选择全部资源库或单个资源库。改变浏览范围只改变查询结果，不会改变工作资源库、上传目标或媒体文件位置。

## 数据不完整时

1. 打开单个媒体的信息面板，确认拍摄时间、相机和坐标是否存在。
2. 检查相关资源库是否在线。
3. 等待元数据队列完成。
4. 仅在元数据存在但统计长期缺失时，收集诊断信息。
