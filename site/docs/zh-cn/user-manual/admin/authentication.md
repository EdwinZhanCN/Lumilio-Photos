---
title: "登录安全与应急恢复"
description: "配置密码、TOTP、恢复代码、通行密钥、会话和 break-glass。"
page_id: "admin/authentication"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/config/config.go
- server/internal/service/auth_service.go
- server/internal/service/auth_mfa.go
- server/internal/service/user_service.go
-->

# 登录安全与应急恢复

流明集使用短期访问令牌、轮换刷新令牌和媒体令牌。默认访问令牌有效 15 分钟，刷新令牌 168 小时，媒体令牌 10 分钟。刷新令牌重用会撤销同一令牌族。

## 推荐基线

- 管理员使用唯一强密码；
- 启用 TOTP 并离线保存恢复代码；
- 在 HTTPS 域名或 localhost 下添加通行密钥；
- 至少保留两位活跃管理员，或定期验证宿主机 break-glass；
- 只在受控日志终端执行应急恢复。

## 限流与锁定

默认认证限流按 IP 和主体分别控制，并在连续失败后施加锁定窗口。不要用脚本持续重试密码；这会延长恢复时间并污染诊断日志。

## 凭据变化

修改密码、管理员重置和应急恢复都会撤销部分或全部会话。关闭 TOTP 会清除恢复代码和通行密钥。任何恢复流程完成后都应重新检查全部认证因素。
