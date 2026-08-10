---
title: "中文文档贡献"
description: "按代码取证、任务模板、TODO 注释和导航门禁维护主文档。"
page_id: "developer/docs-contribution"
audience: "文档贡献者、开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- site/scripts/docs-checks.ts
- site/docs/.vitepress/sidebar/zh-cn.ts
- site/docs/.vitepress/navbar/zh-cn.ts
-->

# 中文文档贡献

中文主文档以代码行为为事实来源。修改页面时：

1. 找到对应路由、服务、配置、测试或 UI 文案。
2. 在页面 `code-evidence` 注释中记录主要路径。
3. 更新 `baseline_commit` 和 `last_verified`。
4. 无法唯一确定时使用 `TODO(category)` HTML 注释，列出现用术语、备选项和决策需求。
5. 一项事实只保留一个权威页面，其他页面链接过去。
6. 用用户会搜索的标题表达任务或症状。
7. 操作页包含前置条件、步骤、成功标志、数据影响和失败处理。
8. 更新中文侧栏，避免主页面成为孤儿。

旧 URL 使用 `search: false` 兼容页，不复制第二份正文。中文落地页和英文文档有独立维护节奏，不应在只修改中文主文档的补丁中顺手改动。
