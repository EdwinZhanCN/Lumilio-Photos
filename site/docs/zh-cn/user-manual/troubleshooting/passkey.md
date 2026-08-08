---
title: "通行密钥不可用"
description: "检查安全来源、TOTP 前提、域名、设备凭据和挑战有效期。"
page_id: "troubleshooting/passkey"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/auth_passkeys.go
- server/internal/httporigin/policy.go
- server/internal/service/auth_mfa.go
-->

# 通行密钥不可用

通行密钥只在安全来源中工作。`localhost` 可用于本机；远程访问需要 HTTPS 域名，当前实现不接受裸 IP 作为 HTTPS 通行密钥来源。

## 无法添加

1. 确认账户已经启用 TOTP。
2. 确认浏览器地址是 `localhost` 或有效 HTTPS 域名。
3. 确认设备支持驻留凭据和用户验证。
4. 在 10 分钟挑战有效期内完成。
5. 检查该凭据是否已经登记。

## 无法登录

先使用密码加 TOTP 或恢复代码登录，再查看已登记通行密钥。删除失效凭据后重新添加。不要为了修复通行密钥而关闭 TOTP：关闭 TOTP 会同时删除全部通行密钥和恢复代码。

反向代理场景还要确保浏览器看到的外部来源与 Server 用于 WebAuthn 验证的来源一致。
