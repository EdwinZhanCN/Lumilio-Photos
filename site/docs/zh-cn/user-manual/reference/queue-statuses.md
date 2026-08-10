---
title: "队列与操作状态"
description: "解释持久任务、上传和数据库恢复中用户可见的主要状态。"
page_id: "reference/queue-statuses"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/queue/jobs/types.go
- server/internal/api/dto/backup_dto.go
- web/src/features/manage
- web/src/features/monitor
-->

# 队列与操作状态

## 后台任务

底层队列使用可用、待处理、可重试、运行中和已计划等活动状态，并保留完成或失败结果。界面可能把这些状态汇总为等待、运行、重试和失败。

- **可用 / 待处理**：等待工作进程领取；
- **已计划**：等待指定时间；
- **可重试**：上次失败，等待退避后重试；
- **运行中**：工作进程已经领取；
- **完成**：当前任务成功；
- **失败或丢弃**：达到失败条件，需要查看错误和人工处理。

## 上传

浏览器显示等待、上传、处理、完成、重复和失败。完成传输不等于所有派生任务完成。

## 数据库恢复

恢复操作状态为 `staged`、`restart_requested`、`installing`、`verifying`、`completed`、`rolling_back`、`rolled_back` 或 `failed`。

状态名称是观察面，不应直接用于手工修改数据库。排障时同时保存任务类型、尝试次数、下一运行时间和错误代码。
