---
title: "分享链接无法打开"
description: "判断令牌复制、到期、撤销、资产快照和公开路由可达性。"
page_id: "troubleshooting/share-unavailable"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/share_link_service.go
- server/internal/api/router.go
- server/internal/api/handler/share_link_handler.go
- web/src/features/share
-->

# 分享链接无法打开

公开访问者看到的失效结果不会详细区分未知、到期和撤销令牌。分享创建者应在登录后的分享管理页检查状态。

## 创建者检查

- 分享记录仍存在且未撤销；
- 到期时间尚未到；
- 链接完整复制，没有被聊天工具截断；
- 分享快照中仍有可访问资产；
- 反向代理允许公开分享路由和媒体子请求；
- “允许下载”和“包含原件”的设置值符合预期，并已了解当前原件策略执行缺口；

相册后来新增媒体不会自动证明旧分享已经包含它，因为分享保存的是创建时解析的资产快照。必要时撤销旧链接并创建新分享。
