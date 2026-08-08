---
title: "从 iCloud 导入"
description: "连接当前支持的云提供方、完成验证并把媒体导入指定资源库。"
page_id: "use/cloud-import"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/cloud/icloud
- server/internal/cloud/sync_service.go
- server/internal/api/router.go
- web/src/features/settings/flows/cloud
-->

# 从 iCloud 导入

当前 baseline 只提供 iCloud 云提供方。云凭据归创建它的用户所有；管理员可以把已连接凭据用于目标资源库的导入。

## 连接与验证

1. 在设置的“云导入”中添加凭据和标签。
2. 选择 Apple 区域并提交身份信息。
3. 如服务要求，完成发送到设备的验证码挑战。
4. 确认状态为“已连接”。
5. 在“管理”中选择目标资源库并启动导入。

密码用于认证，认证会话保存在隔离的应用私有云状态目录。移除凭据会永久删除凭据和会话数据，但已经导入到流明集的媒体仍保留。

## 当前边界

提供方适配器当前只可靠处理内置相册范围，不支持把用户创建的 iCloud 文件夹结构完整映射为流明集结构。界面和 API 的主要语义是“导入”，不要把它当作持续双向同步。

<!-- TODO(cloud-contract): 代码中仍有 sync 命名和兼容端点，而产品界面使用“导入”。可选术语：同步、迁移；本文统一使用“导入”，直到增量、删除和冲突语义被正式定义。 -->
