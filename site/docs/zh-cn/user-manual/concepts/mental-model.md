---
title: "流明集的心智模型"
description: "用四层模型理解文件、目录身份、数据库状态和可选计算。"
page_id: "concepts/mental-model"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/rootcfg/root_config.go
- server/internal/storage/repocfg/repo_config.go
- server/internal/storage/scanner/scanner.go
- server/internal/sourcing/materializer.go
-->

# 流明集的心智模型

可以把流明集理解为四层：

1. **原件层**：普通照片、RAW、视频和音频文件。
2. **资源库层**：存储位置和资源库身份，以及 `inbox/`、`.lumilio/` 等目录约定。
3. **目录层**：SQLite 保存账户、资产记录、相册、人物、事件、分享和任务状态。
4. **计算层**：缩略图、转码、OCR、嵌入、人物聚类和 Agent 工具结果。

原件层是内容本体；目录层让产品知道它们是什么、属于谁、如何组织；计算层可以在条件允许时重建。资源库层把普通文件和目录状态连接起来。

## 两个常见误解

`.lumilioroot` 不是一座资源库，它标识可以容纳多个资源库的存储位置；`.lumiliorepo` 才标识具体资源库。

`inbox/` 不是任意“待扫描文件夹”。它由上传提交路径管理，而扫描明确忽略它。已有目录应放在资源库可扫描区域，而不是塞进 `inbox/` 等待发现。
