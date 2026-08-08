---
title: "失败数量持续增加"
description: "从重复错误样本识别共同依赖，并在修复后控制重试范围。"
page_id: "troubleshooting/failures-growing"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/queue
- server/internal/logging
- server/internal/api/handler/capabilities_handler.go
- web/src/features/monitor
-->

# 失败数量持续增加

失败数量持续增加通常说明共同依赖未恢复，例如磁盘离线、路径权限改变、工具缺失、模型节点不可用、目标文件损坏或数据库写入受阻。

## 处理方法

1. 按任务类型和错误代码分组，不要逐条随机重试。
2. 选择最早和最新各一个样本，比较是否同一根因。
3. 检查问题开始时间附近的部署、挂载、更新和设置变更。
4. 修复共同依赖。
5. 先重试一个可公开或可替换样本。
6. 观察成功后再扩大范围。

如果失败集中在可选智能任务，基础浏览可以继续。不要为了清空失败数字而删除原件或数据库记录。
