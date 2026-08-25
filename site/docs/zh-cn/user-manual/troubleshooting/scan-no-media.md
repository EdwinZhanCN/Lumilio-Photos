---
title: "扫描不到媒体"
description: "确认文件位于资源库、运行环境真实可见且观察操作没有部分覆盖错误。"
page_id: "troubleshooting/scan-no-media"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-22"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/roe/controller/controller.go
- server/internal/storage/roe/changefeed
- server/config/config.go
- server/internal/utils/file/validator.go
- server/internal/api/handler/repository_scan_handler.go
-->

# 扫描不到媒体

扫描观察资源库中的普通媒体文件，包括 `inbox/**`，但明确跳过应用私有的 `.lumilio/**`。不要把用户原件放进 `.lumilio/`。

## 检查顺序

1. 在 Server 所在环境中确认文件真实存在；Docker 宿主机可见不代表容器可见。
2. 确认文件位于资源库目录内，但不在 `.lumilio/` 私有工作区。
3. 确认扩展名属于支持列表。
4. 检查资源库在线和可读权限。
5. 按操作编号查看最新扫描的阶段、已观察目录和文件、错误目录、部分覆盖与后台任务深度。
6. 如果请求显示“已合并”，继续查看同一个活动操作；重复点击不会另起一棵并行遍历。
7. 若出现游标缺口、通知溢出或卷身份变化，保持资源库在线并运行完整验证；不要通过删除数据库记录来强制恢复。

观察默认等待文件稳定 5 秒。刚写入的大文件可能先被发现，随后在稳定后完成哈希；如果哈希期间内容变化，旧结果会被丢弃并按新版本重试。不要在文件仍写入时频繁强制扫描。
