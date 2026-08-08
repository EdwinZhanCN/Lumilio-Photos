---
title: "开始使用"
description: "从选择运行方式到完成第一份完整备份的最短成功路径。"
page_id: "getting-started/index"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/auth/flows/bootstrap/BootstrapFlow.tsx
- server/internal/service/bootstrap_service.go
- server/internal/storage/repo_provisioning.go
-->

# 开始使用

第一次使用流明集时，不要先启用所有功能，也不要一次接入全部原件。推荐顺序是：选择运行方式、完成安装、创建管理员和主资源库、导入少量测试媒体、确认处理完成，最后创建第一份数据库快照并把可恢复的快照与清单副本保存到独立位置。

## 推荐路径

1. 阅读[认识流明集](./about.md)和[选择运行方式](./choose-runtime.md)。
2. 按平台完成[Desktop 安装](./install-desktop.md)或[Docker Server 安装](./install-docker.md)。
3. 依次完成[第一次设置](./first-setup.md)、[账户保护](./secure-account.md)和[主资源库](./primary-repository.md)。
4. 使用副本完成[第一次导入](./import-first-media.md)。
5. 按[首次运行验收](./verify-first-run.md)确认原件、缩略图和队列状态。
6. 建立[第一份完整备份](./first-backup.md)。

完成这些步骤以后，再接入外置磁盘、云导入、Lumen Intelligence 或 Lumilio Agent。这样一旦出现问题，你能够明确判断问题发生在基本媒体库、存储、后台处理，还是可选能力层。
