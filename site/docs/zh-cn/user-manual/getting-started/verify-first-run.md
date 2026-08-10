---
title: "验收首次运行"
description: "用可观察结果确认资源库、媒体处理和基础恢复准备均正常。"
page_id: "getting-started/verify-first-run"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/health_handler.go
- server/internal/api/handler/queue_handler.go
- server/internal/api/handler/capabilities_handler.go
- web/src/features/monitor
-->

# 验收首次运行

首次成功不是“页面能打开”，而是原件、数据库记录和派生处理都形成了可验证闭环。

## 验收清单

- 主资源库显示在线，路径与预期一致。
- 测试媒体出现在资源库视图中。
- 照片有缩略图，视频或音频可以通过 Web 兼容派生文件播放。
- 媒体信息中的文件名、大小和拍摄时间符合预期。
- Server Monitor 中没有持续增长的失败任务。
- 重新启动 Desktop 运行时或 Docker 容器后，账户和媒体仍然存在。
- 数据库快照可以创建、进入已验证列表并下载；同时已为服务器端快照与 manifest 配对文件建立独立复制方案。

Lumen Intelligence 未连接时，图像语义分析、OCR文字识别、人物识别等结果可以缺失；这不代表基础照片管理失败。能力页面会分别报告“已启用”和“当前可用”，排障时不要混为一谈。

## 失败处理

先记录版本、平台、资源库 ID、任务名称和错误样本。不要删除 `.lumilio/`、数据库或挂载来尝试“重置”；这些位置包含恢复和诊断所需状态。
