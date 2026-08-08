---
title: "用户与角色"
description: "管理管理员、普通用户、停用、降级和访问重置。"
page_id: "admin/users-and-roles"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/user_service.go
- server/internal/db/repo/queries/users.sql
- server/internal/db/repo/queries/repositories.sql
- server/internal/api/router.go
-->

# 用户与角色

当前角色只有 `admin` 和 `user`。管理员可以管理用户、系统设置、资源库操作和全局媒体范围；普通用户主要操作自己的资料和被授权媒体范围。

## 安全约束

系统阻止停用或降级最后一位活跃管理员。管理员不能通过界面对自己执行访问重置，以减少当前会话自锁风险。

## 重置访问

为无法登录的用户执行“重置访问”会：

- 生成随机临时密码并只显示一次；
- 要求下次登录后修改密码；
- 使现有会话失效；
- 清除通行密钥、TOTP 和恢复代码。

应通过安全渠道传递临时密码，并要求用户重新建立 MFA。不要把临时密码写入工单或聊天群。

## 主机所有者

第一位账户成为主机所有者，并为主资源库提供兜底所有者关系。不要随意停用或删除这一身份；当前公开 API 没有完整的主机所有权转移流程。
