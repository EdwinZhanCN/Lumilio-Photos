---
title: "界面入口索引"
description: "把常用功能映射到当前 Web 路由和导航名称。"
page_id: "reference/ui-entry-index"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/app/router/routes.tsx
- web/src/app/shell
- server/internal/api/router.go
-->

# 界面入口索引

| 界面入口 | 主要内容 |
| --- | --- |
| 首页 | 画廊、时空轨迹、相机与拍摄统计 |
| 资源库 | 全部媒体、筛选、搜索、批量操作 |
| 合集 | 相册、人物、地点、文件夹、标签和实用工具 |
| 管理 | 上传、资源库、扫描、云导入、索引与检测工具 |
| 工作室 | 非破坏性编辑与导出 |
| 设置 | 账户、外观、云凭据、AI、Server 与用户管理 |
| Server Monitor | 管理员健康、资源库、队列和能力状态 |
| Lumilio Agent | 对话式检索、审阅和有确认的整理操作 |

“管理”页面不是整体路由管理员专用，因为普通上传也在其中；创建资源库、扫描、检测和系统工具仍由 Server API 执行管理员检查。

旧路由 `/upload-photos` 会重定向到 `/manage`。公开分享路由位于登录和初始化门禁之外，只以分享令牌控制访问。
