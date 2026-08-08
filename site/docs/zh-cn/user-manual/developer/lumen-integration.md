---
title: "Lumen 集成边界"
description: "通过锁文件、协议和能力发现维护流明集与 Lumen Intelligence 的连接。"
page_id: "developer/lumen-integration"
audience: "开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- lumen.lock.json
- taskfile.yml
- server/internal/service/lumen_service.go
- desktop/internal/lumen/release_catalog.go
- deploy/compose
-->

# Lumen 集成边界

流明集通过 `lumen.lock.json` 固定消费的 Lumen 发布，并由 Task 工具验证锁、目录、校验和、协议与 Compose 意图。Desktop 的 Lumen 发布目录是生成文件，不应手工维护。

Server 不应假定节点一定存在。能力层按任务报告启用与可用，缺失节点时基本媒体库必须继续启动。新增任务需要同时更新协议、能力映射、队列、设置、错误处理、前端状态和文档。

官方 Lumen Compose 使用主机网络以支持 mDNS；CUDA 声明全部 GPU，Vulkan 映射 `/dev/dri`。改变网络模型需要同步修改验证门禁，而不是只改示例文件。

跨仓库兼容应通过版本和协议握手表达，不依赖开发机器上的相邻源码目录。
