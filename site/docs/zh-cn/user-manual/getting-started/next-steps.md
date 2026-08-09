---
title: "接下来可以做什么"
description: "在基础照片管理和备份闭环稳定后，按需要逐步启用高级能力。"
page_id: "getting-started/next-steps"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/collections
- desktop/internal/storage/controller.go
- server/internal/api/handler/capabilities_handler.go
- server/internal/service/settings_service.go
-->

# 接下来可以做什么

完成首次验收和备份后，再按实际需求增加复杂度。

## 整理已有媒体

阅读[组织媒体](../use/organize.md)，建立相册、标签、喜欢、人物和事件工作流。相册是持久化对象；“合集”只是把相册、人物、地点、文件夹、标签和实用工具集中显示的导航区。

## 接入更多存储

Desktop 可以通过本机目录选择器注册额外存储位置；Docker 当前应通过宿主机挂载规划把物理位置呈现在默认存储位置下。先阅读[存储位置与资源库](../admin/storage-and-repositories.md)，避免混淆 `.lumilioroot` 和 `.lumiliorepo`。

## 启用智能能力

需要图像语义分析、OCR文字识别、人物识别或BioCLIP物种识别时，再部署[Lumen Intelligence](../use/lumen-intelligence.md)。需要自然语言检索和有确认的整理操作时，再配置[Lumilio Agent](../use/agent.md)。两者都不是基本浏览和原件保存的前提。

## 建立维护节奏

至少定期检查磁盘空间、资源库在线状态、失败任务、数据库快照和异地媒体备份；升级前先创建恢复点，并阅读对应版本的升级说明。
