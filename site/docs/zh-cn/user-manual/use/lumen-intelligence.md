---
title: "Lumen Intelligence"
description: "理解智能任务、节点发现和“已启用”与“可用”的区别。"
page_id: "use/lumen-intelligence"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/service/lumen_service.go
- server/internal/api/handler/capabilities_handler.go
- desktop/internal/lumen
- deploy/compose/lumen-cpu.compose.yml
-->

# Lumen Intelligence

Lumen Intelligence 是流明集之外的计算层。流明集 Server 可以在没有节点时正常启动和浏览基本媒体；依赖节点的任务会退化为不可用或进入可重试失败状态。

## 当前任务

- 图像和文本语义嵌入；
- OCR 文字识别；
- 人脸检测、嵌入和人物聚类输入；
- BioCLIP 生物主题分类；
- 视频采样帧语义处理。

能力页分别报告任务是否在设置中启用，以及当前是否有健康节点提供它。只有两者都满足，任务才可实际执行。

## 节点来源

Desktop 可以安装和监督本机 Lumen Intelligence。Server 可以通过静态地址、mDNS 发现或 Lumen Hub 相关机制连接节点；Docker 示例还提供 CPU、CUDA 和 Vulkan 叠加配置。

智能索引属于可重建派生数据。不要因为它可重建而忽略原件和数据库备份，也不要在节点暂时不可用时删除资源库状态。
