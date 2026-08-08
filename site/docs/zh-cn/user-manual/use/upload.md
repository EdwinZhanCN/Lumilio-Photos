---
title: "上传媒体"
description: "理解浏览器上传、暂存、内容重复检测和后台处理。"
page_id: "use/upload"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/asset_handler.go
- server/internal/sourcing/materializer.go
- server/internal/utils/file/validator.go
- server/internal/storage/directory_manager.go
-->

# 上传媒体

上传支持照片、RAW、视频和音频扩展名。浏览器可以使用单文件或分块会话，并在后台显示等待、上传、处理、完成、重复或失败状态。

## 数据路径

1. 服务端先检查目标资源库在线。
2. 数据写入资源库 `.lumilio/staging/incoming`。
3. 完成后计算完整 BLAKE3 哈希和文件大小。
4. 同一资源库中已有相同内容且现有文件可验证时，上传被标记为重复，暂存副本被移除。
5. 非重复内容由提交任务按资源库布局放入 `inbox/`，再进入元数据和派生处理队列。

上传失败时，服务会尽力把暂存内容隔离到 `.lumilio/staging/failed`；在无法安全隔离时，不会为了“清理”而盲目删除唯一暂存副本。

## 注意事项

文件类型校验当前以扩展名白名单为主，而不是通过完整内容嗅探决定类型。错误扩展名或伪装文件可能在后续提取阶段失败。上传完成前不要断开目标外置磁盘。
