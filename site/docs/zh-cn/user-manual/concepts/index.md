---
title: "了解原理"
description: "用不依赖实现细节的模型理解原件、资源库、处理、索引和恢复。"
page_id: "concepts/index"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/doc.go
- server/internal/sourcing
- server/internal/db/backup
- server/internal/agent
-->

# 了解原理

这里解释“为什么”，不代替具体操作步骤。理解这些边界可以避免最常见的数据事故：把数据库当作照片备份、把扫描当作上传、把回收站当作磁盘清理，或者把 Lumen 节点当作媒体存储。

建议先阅读[流明集心智模型](./mental-model.md)，再按需要进入资源库身份、处理流水线、搜索索引、人物事件、Agent 权限和完整备份。
