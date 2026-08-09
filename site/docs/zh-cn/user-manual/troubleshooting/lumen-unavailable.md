---
title: "Lumen Intelligence 节点不可用"
description: "检查节点进程、发现、网络、能力方案和任务健康。"
page_id: "troubleshooting/lumen-unavailable"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- desktop/internal/lumen
- server/internal/service/lumen_service.go
- deploy/compose/lumen-cpu.compose.yml
-->

# Lumen Intelligence 节点不可用

“已发现节点”表示服务看到了节点身份；“活跃节点”表示当前健康；具体任务“可用”还要求节点声明并能执行该任务。

## Desktop

查看 Lumen 安装、当前版本、进程状态、缓存和日志。主 Server 正常而 Lumen 子进程失败时，基础照片管理应继续可用。

## Docker 或独立节点

确认容器运行、模型缓存可写、CPU/GPU 设备可见、gRPC 地址从流明集 Server 所在网络可达。主机网络、容器网络和浏览器网络是不同视角。

## 发现问题

mDNS 适合可信局域网，但跨网段、VPN 或受限容器网络可能无法发现。此时使用明确静态地址，并记录节点和 Server 之间的实际连接路径。
