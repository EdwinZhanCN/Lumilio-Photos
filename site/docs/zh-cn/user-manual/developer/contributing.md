---
title: "提交变更"
description: "遵循仓库权威指南、生成流程和按范围验证。"
page_id: "developer/contributing"
audience: "开发者、贡献者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- CONTRIBUTING.md
- AGENTS.md
- web/ARCHITECTURE.md
- taskfile.yml
-->

# 提交变更

根目录 `CONTRIBUTING.md` 和 `AGENTS.md` 是贡献入口。开始前检查活动执行计划，避免与长期任务冲突。

提交前：

- 运行与变更范围匹配的 Task 门禁；
- 确认生成文件没有意外漂移；
- 不提交秘密、数据库、缓存或本地媒体；
- 保持完整 TOML manifest 原则，不添加隐式配置搜索或普通环境变量覆盖；
- 遵循前端功能边界和 Go 格式；
- 使用仓库约定的 Conventional Commits 风格。

涉及用户行为的代码变更必须同步中文权威文档或明确记录文档债；涉及数据删除、备份、恢复、账户暴露或外部模型数据的变更还应提供测试和决策记录（资源库 `.agents/decisions/`）。
