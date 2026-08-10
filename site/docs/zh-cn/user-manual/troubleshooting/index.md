---
title: "解决问题：从症状开始"
description: "先保护数据和证据，再按安装、存储、处理、能力或访问层定位问题。"
page_id: "troubleshooting/index"
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
- server/internal/logging
- web/src/features/monitor
-->

# 解决问题：从症状开始

排障的第一目标不是让错误暂时消失，而是判断哪一层失败，并保留恢复所需证据。

## 先做三件事

1. 停止重复提交同一操作，尤其是导入、恢复、索引重建和磁盘移动。
2. 记录当前版本、平台、页面、时间、资源库、任务名和可见错误。
3. 不要删除数据库、`.lumiliorepo`、`.lumilioroot`、`.lumilio/` 或应用私有状态。

## 按症状进入

| 症状 | 页面 |
| --- | --- |
| 安装包、容器或权限失败 | [安装失败](./installation.md) |
| Desktop Server 起不来 | [Desktop 运行时](./desktop-runtime.md) |
| 浏览器打不开或健康检查失败 | [无法打开流明集](./cannot-open.md) |
| 无法登录、丢失 TOTP 或恢复代码 | [登录与 MFA](./login-mfa.md) |
| 通行密钥不可用 | [通行密钥](./passkey.md) |
| 磁盘或资源库离线 | [资源库离线](./repository-offline.md) |
| 上传或扫描失败 | [上传失败](./upload-failed.md)、[扫描不到媒体](./scan-no-media.md) |
| 处理或队列不前进 | [处理停滞](./processing-stuck.md)、[队列停滞](./queue-stuck.md) |
| 搜索、人物或 OCR 缺失 | [搜索缺失](./search-missing.md)、[智能结果缺失](./ai-no-results.md) |
| 恢复时页面断开 | [恢复期间断开](./restore-disconnect.md) |

无法定位时，按[收集诊断信息](./collect-diagnostics.md)准备最小证据集。
