---
title: "创建第一位管理员"
description: "使用服务端真实凭据策略创建首个账户，并理解其不可替代职责。"
page_id: "getting-started/create-admin"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/service/credential_policy.go
- server/internal/service/auth_passkeys.go
- server/internal/service/user_service.go
- server/internal/db/repo/queries/repositories.sql
-->

# 创建第一位管理员

第一位注册成功的账户自动获得 `admin` 角色，并成为主机所有者。这个身份会作为主资源库和遗留无所有者资源库的兜底所有者，因此应由真正负责部署和恢复的人创建。

## 用户名规则

服务端会去除首尾空白并转为小写。用户名长度为 3–32 个 Unicode 字符，必须以 ASCII 字母开头；其余位置只能使用小写字母、数字、点、下划线或连字符。用户名不能以分隔符结尾，也不能连续或混合使用分隔符。

## 密码规则

密码按字节计为 10–72 字节，并且至少包含一个小写字母、一个大写字母和一个 Unicode 数字。界面或 API 声明中出现的更宽松提示不代表服务端会接受。

## 管理员职责

管理员可以管理用户、系统设置、资源库操作、队列与全局媒体范围。系统阻止停用或降级最后一位活跃管理员，但这不能替代恢复代码、数据库快照和宿主机应急恢复能力。

<!-- TODO(auth-contract): RegistrationStartRequest 仍声明用户名最大 50、密码最小 6，而 credential_policy.go 实际执行用户名最大 32、密码最小 10，且要求复杂度。可选方向：统一 DTO、OpenAPI、前端提示与服务策略。本文以服务策略为准。 -->
