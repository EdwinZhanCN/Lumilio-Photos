---
title: "Lumen Intelligence 数据流"
description: "理解媒体任务从流明集 Server 到计算节点再回到索引的边界。"
page_id: "concepts/lumen-data-flow"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/lumen_service.go
- server/internal/queue/jobs/types.go
- lumen.lock.json
-->

# Lumen Intelligence 数据流

流明集 Server 负责选择任务和资产、读取必要媒体内容并通过 gRPC 与 Lumen Intelligence 节点通信。节点返回嵌入、文字、人脸或分类结果，Server 再把结果保存到相应数据库或索引。

节点可以与 Server 同机、在可信局域网或由静态地址连接。把它统一称作“设备内 AI”会掩盖网络边界；应根据实际部署说明媒体或预处理数据会经过哪条链路。

Lumen 节点不持有流明集账户和相册真相。节点下线时，原件和数据库仍由流明集管理；任务可以失败、重试或稍后重建。

为敏感媒体部署节点时，应保护 gRPC 网络、节点日志、模型缓存和临时数据，并避免把节点暴露到不可信网络。
