---
title: "Taskfile 工作流"
description: "使用跨平台任务入口保持本地与 CI 命令语义一致。"
page_id: "developer/taskfile"
audience: "开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- taskfile.yml
- server/taskfile.yml
- web/taskfile.yml
- desktop/taskfile.yml
- site/taskfile.yml
-->

# Taskfile 工作流

根 `taskfile.yml` 负责编排模块任务；GitHub Actions 负责 Runner、原生依赖、缓存、凭据和制品。优先使用 Task 入口，不要为同一流程新增只在某个 shell 可用的脚本。

| 命令 | 用途 |
| --- | --- |
| `task setup` | 安装依赖和开发工具 |
| `task dev` | 启动 Server 与 Web |
| `task test` | 架构、Server 与 Web 测试 |
| `task server:test` | Server 测试 |
| `task web:test` | Web 类型、lint、边界与单元测试 |
| `task desktop:test` | Desktop race-test 门禁 |
| `task compose:test` | 验证所有 Compose |
| `task ci:site` | 安装并构建文档站 |
| `task dto` | 生成 OpenAPI、前端类型和 API 文档 |
| `task config:examples` | 生成配置 schema 与示例 |
| `task lumen:check` | 离线验证 Lumen 锁、目录和协议意图 |
| `task assets:check` | 离线验证资源锁 |

模块目录内也可以使用各自 Taskfile 的普通任务名。新增命令时保持根编排边界和模块所有权。
