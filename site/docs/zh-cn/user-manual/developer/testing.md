---
title: "测试与门禁"
description: "按修改范围选择架构、模块、Desktop、Compose、浏览器和文档检查。"
page_id: "developer/testing"
audience: "开发者、文档贡献者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- taskfile.yml
- web/taskfile.yml
- site/taskfile.yml
- site/scripts/docs-checks.ts
-->

# 测试与门禁

`task test` 包含架构检查、Server 测试和 Web 测试，不包含 Desktop 或浏览器 E2E。交付相关变更还需要对应门禁。

## 常用组合

```bash
task architecture:check
task server:test
task web:test
task desktop:test
task compose:test
task ci:site
```

浏览器回归通过隔离 Compose 环境和 Web Taskfile 运行；认证强化、Agent 信任、视频语义和备份恢复有窄化目标。CI 失败应使用 workflow 里的同名任务复现（例如 `task web:test`、`task server:test:ci`），只有真正跨模块编排才保留 `ci:*`（`ci:architecture`、`ci:site`、`ci:desktop:*`）。

文档变更至少运行站点 `docs-checks.ts` 和 VitePress build。新增中文页面还应保证侧栏可达、旧路由兼容、frontmatter baseline 存在且无孤儿主页面。
