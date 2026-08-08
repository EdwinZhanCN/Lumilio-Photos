---
title: "第一次设置"
description: "按实际初始化门禁完成语言、管理员、账户保护和主资源库。"
page_id: "getting-started/first-setup"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/bootstrap_service.go
- web/src/features/auth/flows/bootstrap/BootstrapFlow.tsx
- server/internal/service/auth_passkeys.go
- server/internal/service/auth_mfa.go
-->

# 第一次设置

全新实例只有在至少存在一个管理员，并且恰好有一个可用主资源库后，才进入正常应用。Web 初始化流程依次引导语言与地区、管理员账户、可选 TOTP、通行密钥、恢复代码和主资源库。

## 推荐顺序

1. 选择界面语言和地区。
2. 创建第一位管理员。
3. 启用 TOTP，并保存恢复代码。
4. 在安全来源下添加通行密钥；也可以稍后完成。
5. 创建主资源库。
6. 进入应用后先检查 Server 状态和资源库状态。

第一位账户会成为管理员和主机所有者；后续通过公开注册创建的账户是普通用户。当前实现没有邀请码或“仅管理员创建账户”的开关，因此部署者必须单独控制服务可达范围。

::: warning 不要跳过恢复准备
TOTP 在流程中可以跳过，但通行密钥要求账户已经启用 TOTP。恢复代码只在生成时以明文显示；保存完成后，服务端只保留不可逆摘要。
:::
