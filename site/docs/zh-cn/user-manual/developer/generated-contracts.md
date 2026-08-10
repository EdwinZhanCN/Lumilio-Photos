---
title: "生成代码与契约"
description: "维护 SQL、OpenAPI、配置、i18n、Lumen 和资源锁定的单一来源。"
page_id: "developer/generated-contracts"
audience: "开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- CONTRIBUTING.md
- taskfile.yml
- lumen.lock.json
- assets.lock.json
-->

# 生成代码与契约

不要手工修改生成输出。

- 后端 DTO 或 API 注解变化：`task dto`。
- 配置 profile、schema 注释或示例变化：`task config:examples`。
- SQL schema 或 query 变化：在 `server/` 运行 `sqlc generate`。
- 前端 i18n：先在代码写 key 与默认值，再运行 i18next 提取并补中文。
- Lumen 发布更新：使用 `task lumen:sync`，再运行 `task lumen:check`。
- 测试资源更新：使用资产 reconcile/sync 流程，再运行 `task assets:check`。

生成文件漂移通常说明来源或命令未同步，不应在生成结果上做孤立补丁。
