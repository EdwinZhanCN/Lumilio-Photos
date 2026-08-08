---
title: "保护管理员账户"
description: "正确组合密码、TOTP、通行密钥和一次性恢复代码。"
page_id: "getting-started/secure-account"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/auth_mfa.go
- server/internal/service/auth_passkeys.go
- server/internal/httporigin/policy.go
- web/src/features/auth/components/ui/SecurityPanels.tsx
-->

# 保护管理员账户

流明集的通行密钥不是独立替代所有恢复方式的单一因素。代码要求先启用 TOTP，才能添加通行密钥；关闭 TOTP 时会同时删除恢复代码和全部通行密钥，以保留“密码加 TOTP”的可用回退路径。

## 推荐配置

1. 使用密码管理器保存唯一密码。
2. 在身份验证器中扫描 TOTP 设置码并验证。
3. 将生成的恢复代码保存到与 Server 分离的位置。
4. 在 `localhost` 或 HTTPS 域名下添加通行密钥。
5. 使用另一浏览器会话验证登录，再退出初始化会话。

恢复代码每个只能使用一次。重新生成会使旧代码失效；关闭 TOTP 和重新生成恢复代码都要求当前密码。

## 通行密钥来源要求

浏览器只在安全来源中启用通行密钥。`localhost` 可以使用；远程访问必须使用 HTTPS 域名。当前实现明确拒绝把裸 IP 地址作为 HTTPS 通行密钥来源。

::: danger 保存恢复代码
不要只把恢复代码保存在同一座流明集媒体库中。账户锁定或服务不可用时，你可能无法读取它们。
:::
