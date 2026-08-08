---
title: "上传失败"
description: "从目标资源库、扩展名、暂存空间、网络和后台提交定位失败。"
page_id: "troubleshooting/upload-failed"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/asset_handler.go
- server/internal/utils/file/validator.go
- server/internal/storage/directory_manager.go
- web/src/features/manage
-->

# 上传失败

## 上传开始前失败

确认目标资源库在线且当前用户有上传权限。当前文件类型校验主要依据扩展名；不支持或伪装的扩展名会被拒绝或在后续提取失败。

## 上传中断

检查浏览器网络、Server 可用性和 `.lumilio/staging/incoming` 所在磁盘空间。分块会话可以保存进度，但会话过期或目标资源库离线时应重新开始。

## 上传完成但处理失败

上传数据可能已经安全暂存或提交，失败不等于原件消失。查看任务错误和 `.lumilio/staging/failed`；不要手工删除暂存区。相同内容再次上传时，服务可能通过哈希识别为重复。

## 收集信息

记录文件扩展名、大小、目标资源库、上传会话状态、错误时间和相关队列。不要公开上传原始私密媒体来证明问题；可以使用可分享的最小复现文件。
