---
title: "术语表"
description: "统一流明集中文文档中的产品、存储、账户和处理术语。"
page_id: "reference/glossary"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/storage/rootcfg/root_config.go
- server/internal/storage/repocfg/repo_config.go
- web/src/locales/zh/translation.json
- server/internal/db/backup/backup.go
-->

# 术语表

| 术语 | 含义 |
| --- | --- |
| 流明集（Lumilio Photos） | 媒体管理产品；首次出现使用中英文全名，后文可简称“流明集” |
| Lumen Intelligence | 独立计算能力层，不设中文产品名 |
| Lumilio Agent | 模型驱动的受控检索与整理能力，不设中文产品名 |
| 存储位置 | 带 `.lumilioroot` 的父位置，可容纳多个资源库 |
| 资源库 | 带 `.lumiliorepo` 的具体媒体单元；界面部分位置仍写“仓库” |
| 主资源库 | 默认存储位置下固定目录 `primary` 的唯一主库 |
| 原件 | 用户提供的普通媒体文件 |
| 派生文件 | 缩略图、Web 转码、人脸裁剪等可重建输出 |
| 应用私有状态 | SQLite、配置、密钥、云会话、数据库快照和应用日志 |
| 提交区 | 资源库 `inbox/`，由上传提交路径管理 |
| 资源库工作区 | `.lumilio/`，保存派生、sidecar、暂存和日志 |
| 扫描 | 发现已位于资源库可扫描区域的文件 |
| 导入 | 从浏览器上传或云提供方把内容接入资源库 |
| 合集 | UI 聚合入口，不是独立数据实体 |
| 相册 | 用户拥有的持久化媒体集合 |
| 通行密钥 | Passkey / WebAuthn 凭据 |
| 数据库快照 | 通过 SQLite Online Backup 创建并验证的目录状态副本 |
| 完整备份 | 至少包含媒体存储与数据库快照 |

<!-- TODO(terminology): UI i18n 仍混用“仓库”“库”“资源库”“媒体库”。替代方案见交付包 `review/terminology-decisions.md`；正文在复述具体按钮时才保留界面原文。 -->
