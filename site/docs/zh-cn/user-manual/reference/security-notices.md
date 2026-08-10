---
title: "安全公告与报告边界"
description: "说明哪些现象属于公开文档，哪些细节应通过私密渠道报告。"
page_id: "reference/security-notices"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- !SECURITY.md
- server/internal/logging/logger.go
- server/internal/service/auth_service.go
-->

# 安全公告与报告边界

当前文档公开说明部署者必须知道的产品边界，例如默认 HTTP、公开注册、分享令牌、外部模型提供方和数据库快照不含媒体。这些不是替代漏洞报告的完整清单。

发现可能导致未授权访问、秘密泄露、远程执行、绕过角色、任意文件访问或不可逆数据损坏的问题时，不要在公开 Issue 提供利用细节、真实令牌或受影响实例地址。当前 baseline 没有 `SECURITY.md`，也没有经过代码包验证的私密报告地址。不要因此把利用细节改发到公开 Issue；可以先发布一条不含漏洞细节、令牌、实例地址或复现载荷的最小联系请求，但这并不能替代正式的私密报告流程。

修复前应保存日志和版本证据，撤销相关密钥，限制服务可达范围，并避免破坏后续取证。

<!-- TODO(security-contact): 当前 baseline 明确缺少 `SECURITY.md` 和可验证的私密报告渠道。可选方向：启用平台私密漏洞报告、增加 `SECURITY.md`，并把联系入口纳入发布门禁；本文不硬编码不存在的邮箱或 URL。 -->
