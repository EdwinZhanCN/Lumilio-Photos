---
title: "账户、用户与偏好"
description: "管理个人资料、密码、MFA、外观、语言和管理员用户操作。"
page_id: "use/account"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/user_service.go
- server/internal/service/auth_service.go
- web/src/features/settings
- server/internal/api/router.go
-->

# 账户、用户与偏好

普通用户可以修改自己的显示名称和密码，并管理 TOTP、恢复代码与通行密钥。管理员还能列出用户、调整可用状态和角色，以及为无法登录的用户重置访问。

## 密码与会话

修改密码会使现有刷新令牌失效。管理员“重置访问”会生成一次显示的临时密码，要求用户下次登录后设置永久密码，并清除该用户的通行密钥、TOTP 和恢复代码。

## 外观偏好

设置页可以选择语言、地区、亮暗主题配对、是否跟随系统，以及资源页默认布局与紧凑列数。这些偏好不会改变 Server 的媒体存储或数据库备份策略。

## 云与 Server 设置

云凭据属于当前用户。系统 AI、备份、运行时信息和用户管理属于管理员范围。界面是否显示入口不是唯一安全边界，Server API 仍会执行角色检查。
