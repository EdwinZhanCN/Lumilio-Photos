---
title: "公开注册与访问边界"
description: "处理初始化后仍开放注册入口带来的账户暴露风险。"
page_id: "admin/registration-exposure"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/internal/service/auth_passkeys.go
- web/src/app/router/routes.tsx
- web/src/features/auth/flows/sign-in/LoginFlow.tsx
-->

# 公开注册与访问边界

当前 `POST /api/v1/auth/register` 和 `/register` 页面在初始化完成后仍可用。第一位账户是管理员，后续注册账户是普通用户；代码没有邀请码、管理员审批或关闭注册的设置。

这意味着任何能访问服务登录页的人都可能创建普通账户。角色限制可以阻止其执行管理员 API，但不能把“能自助获得账户”视作安全默认。

## 当前缓解措施

- 不要把默认 HTTP 实例直接暴露到互联网。
- 在反向代理、VPN、零信任网关或可信局域网边界限制访问者。
- 定期检查用户列表并停用未知账户。
- 为管理员启用 TOTP、恢复代码和通行密钥。
- 把公开注册视为发布前已知问题，而不是邀请功能。

::: danger 与默认 Docker 组合
默认 Compose 使用主机网络和 HTTP。若宿主机端口可被不可信网络访问，公开注册与未加密凭据传输会叠加风险。
:::

<!-- TODO(security): 可选产品方案包括默认关闭注册、一次性邀请码、管理员创建账户、首位管理员后自动关闭、反向代理可信身份。账户获取模型的取舍需要在 `.agents/decisions/` 留下决策记录。 -->
