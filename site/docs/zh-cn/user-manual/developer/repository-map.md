---
title: "仓库与组件地图"
description: "快速定位产品代码、交付配置、文档和跨仓库锁定。"
page_id: "developer/repository-map"
audience: "开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- taskfile.yml
- assets.lock.json
- lumen.lock.json
- AGENTS.md
-->

# 仓库与组件地图

| 路径 | 责任 |
| --- | --- |
| `server/` | Go Server、配置、API、数据库、队列、存储和工具 |
| `web/` | React Web、功能模块、i18n、E2E 与生成类型 |
| `desktop/` | Wails Desktop、主机控制面、资源与打包 |
| `deploy/compose/` | Server、反向代理和 Lumen Compose 交付配置 |
| `wasm/` | 浏览器 WASM 包 |
| `site/` | VitePress 文档、媒体和 OpenAPI 站点 |
| `.github/workflows/` | 触发、Runner、缓存、制品与发布聚合 |
| `assets.lock.json` | Lumilio-Assets 版本锁定 |
| `lumen.lock.json` | Lumen 发布与契约锁定 |

baseline 包还包含相邻的 Lumen-Hub、Lumen-SDK 和 Lumilio-Assets 快照，用于联合审查；主仓库通过锁文件和生成目录消费它们，不应把本地相邻目录当作隐式运行依赖。
