---
title: "创建与管理分享链接"
description: "用带到期时间和下载边界的公开令牌分享媒体快照。"
page_id: "use/sharing"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/service/share_link_service.go
- server/internal/api/router.go
- server/internal/api/handler/share_link_handler.go
- web/src/features/share
- server/internal/db/repo/queries/share_links.sql
-->

# 创建与管理分享链接

分享链接使用随机公开令牌访问，不要求接收者登录。公开路由位于认证和初始化门禁之外，因此任何获得有效链接的人都可能访问允许的内容。

## 创建分享

可以从资产选择、相册、人物或实用工具查询创建分享。服务会把创建时解析出的资产 ID 保存为快照，最多 5000 项。可以设置标题、描述、到期时间、“允许下载”和“包含原件”；到期时间最长 365 天，默认 30 天。

::: danger 当前原件策略执行不一致
单文件原件端点会检查两个开关，但公开 ZIP 下载只检查“允许下载”，并打包原件；视频和音频的公开 Web 播放端点在派生文件缺失时会直接回退到原件，而且不检查这两个开关。修复前，不要把关闭“包含原件”视作已经阻止所有原件字节外发。对敏感分享应关闭下载，并确认所有视频/音频已有可接受的 Web 派生文件，或暂不创建公开分享。
:::

<!-- TODO(share-original-policy): `include_originals` 没有贯穿 ZIP 下载与 Web 媒体 fallback。可选方向：所有原件读取统一执行策略；关闭原件时只返回派生文件且派生缺失即失败；为策略组合增加端到端测试。 -->

## 管理

分享只由创建者管理。可以查看、撤销，并在已到期或已撤销后删除记录。公开访问对未知、到期和撤销令牌返回尽量一致的结果，减少外部枚举信息。

## 安全检查

1. 创建前检查实际媒体数量。
2. 对含位置或敏感 EXIF 的内容，谨慎启用原件。
3. 使用最短合理到期时间。
4. 通过无登录浏览器验证接收者看到的内容。
5. 不再需要时立即撤销。

<!-- TODO(share-snapshot-ux): 从相册创建的分享是资产快照，不是明确的实时相册订阅。可选方向：界面标明“创建时快照”，或提供可选择的实时/快照语义。 -->
