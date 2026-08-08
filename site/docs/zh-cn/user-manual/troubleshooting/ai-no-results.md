---
title: "人物、OCR 或语义没有结果"
description: "区分设置已启用、节点可用、任务成功和派生结果可查询四个阶段。"
page_id: "troubleshooting/ai-no-results"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/capabilities_handler.go
- server/internal/queue/jobs/types.go
- server/internal/service/face_service.go
- server/internal/event
-->

# 人物、OCR 或语义没有结果

智能结果需要四个条件同时成立：功能开关开启、健康节点提供任务、队列成功处理目标媒体、索引或聚类结果已经可查询。

## 排查顺序

1. 在设置中确认目标任务已启用。
2. 刷新能力状态，确认任务显示可用。
3. 在 Server Monitor 查对应任务失败。
4. 对一个受支持样本执行重新处理或索引。
5. 人物功能还需完成聚类；事件还需完成事件构建。
6. 检查当前用户和资源库范围。

BioCLIP、OCR、人物和语义使用不同任务。一个任务可用不代表其他任务可用；不要用“节点在线”代替逐任务检查。
