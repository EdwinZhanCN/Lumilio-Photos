---
title: "无法登录或丢失 MFA"
description: "按密码、TOTP、恢复代码、管理员重置和宿主机应急恢复逐级恢复访问。"
page_id: "troubleshooting/login-mfa"
audience: "用户、管理员、宿主机操作者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/cmd/main.go
- server/app/breakglass.go
- server/internal/service/user_service.go
- server/internal/service/auth_mfa.go
-->

# 无法登录或丢失 MFA

先确认用户名使用服务端规范化后的形式，并检查设备时间是否准确。TOTP 挑战有效期为 5 分钟；恢复代码一次只能使用一个。

## 仍有恢复代码

在 MFA 页面切换到恢复代码，输入一条未使用代码。登录后立即重新生成恢复代码，并重新验证 TOTP 和通行密钥。

## 还有另一位管理员

另一位管理员可以在用户设置中“重置访问”。系统会显示一次临时密码、撤销现有会话、清除 TOTP、恢复代码和通行密钥，并要求目标用户下次登录后设置永久密码。

## 所有管理员都被锁定

宿主机操作者可以执行显式的一次性 break-glass 启动控制：设置 `LUMILIO_BREAK_GLASS=1`，可选用 `LUMILIO_BREAK_GLASS_USERNAME` 指定活跃管理员，然后启动 Server。临时密码只在安全日志中显示一次。完成登录和永久密码设置后，必须移除这两个启动控制并正常重启。

::: danger 日志中的临时密码
break-glass 日志含一次性秘密。只在受控终端执行，完成后保护或清理相应日志，不要上传完整日志到公开 Issue。
:::
