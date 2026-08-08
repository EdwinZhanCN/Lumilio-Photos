---
title: "分享接收者指南"
description: "说明无需账户的公开分享访问、下载和常见失效原因。"
page_id: "use/share-recipient"
audience: "分享接收者"
platform: "浏览器"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/router.go
- server/internal/service/share_link_service.go
- server/internal/api/handler/share_link_handler.go
- web/src/features/share/routes/PublicShare.tsx
-->

# 分享接收者指南

收到流明集分享链接后，可以直接在浏览器打开，不需要创建流明集账户。你能看到和下载的内容由分享创建者设置。

## 链接无法打开

链接可能已经到期、被撤销、删除，或者复制不完整。公开页面不会向未认证访问者详细区分这些原因；请联系分享者重新确认链接。

## 下载边界

页面可以显示为“仅浏览”或允许下载，但当前实现存在策略缺口：允许下载的 ZIP 会包含原件，即使创建者关闭“包含原件”；视频或音频缺少 Web 派生文件时，播放请求也可能回退到原件。接收者仍只能访问分享快照内的资产，但创建者不应依靠“包含原件”开关保护敏感原始字节。

## 隐私

分享链接本身就是访问凭据。不要把它转发到公开频道；使用完毕后可以要求分享者撤销。下载的原件可能包含拍摄设备、时间和位置等元数据。
