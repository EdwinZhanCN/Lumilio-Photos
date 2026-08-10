---
title: "获得帮助与提交 Issue"
description: "用完整、脱敏、可复现的材料请求社区或维护者帮助。"
page_id: "troubleshooting/get-help"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- !.github/ISSUE_TEMPLATE
- !SECURITY.md
- CONTRIBUTING.md
- server/internal/logging/logger.go
-->

# 获得帮助与提交 Issue

提交问题前先阅读[当前已知问题](./known-issues.md)，并按[收集诊断信息](./collect-diagnostics.md)准备材料。

一个高质量 Issue 应包含：版本、运行方式、操作系统、复现步骤、预期与实际结果、发生时间、最小日志片段、资源库或任务状态，以及你已经尝试且确认安全的步骤。

不要只写“不能用”，也不要把完整数据库、私密照片、API Key、恢复代码或分享链接上传到公开仓库。当前 baseline 没有可验证的私密安全报告渠道。公开 Issue 中不要包含利用细节；最多只提交已脱敏的最小联系请求，并把缺失报告渠道视为发布治理缺口。

如果问题涉及数据丢失风险，先停止写入，复制现有媒体与应用私有状态，再继续诊断。恢复原件优先于清理界面状态。

<!-- TODO(support-channel): 当前 baseline 没有经验证的统一支持 URL、Issue 模板或私密安全报告渠道。可选方向：新增 `SECURITY.md` 和 Issue 模板，并与文档站共享机器可检查的支持元数据。本文不写未经确认的地址。 -->
