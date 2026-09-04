---
title: "开发者入口"
description: "用最少的公开工程文档进入架构、开发、测试、契约和文档贡献。"
page_id: "developer/index"
audience: "开发者、贡献者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- AGENTS.md
- CONTRIBUTING.md
- taskfile.yml
- docs
-->

# 开发者入口

开发者内容是用户文档的最后一层。开始前先阅读仓库根目录 `AGENTS.md` 和 `CONTRIBUTING.md`；内部架构、执行计划和技术债以 `docs/` 中的工程文档为权威，不在公开帮助中心复制。

最短路径：安装前置工具，运行 `task setup`，使用 `task dev` 启动 Web 与 Server，按变更范围运行模块测试，契约变化后运行 `task dto`。
