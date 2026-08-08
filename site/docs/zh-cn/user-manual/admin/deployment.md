---
title: "部署决策"
description: "为个人 Desktop、家庭 Server 和独立 Lumen 节点划分责任。"
page_id: "admin/deployment"
audience: "管理员、所有者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- desktop/README.md
- desktop/main.go
- deploy/compose/compose.yml
- server/internal/service/lumen_service.go
-->

# 部署决策

## 个人 Desktop

适合一位所有者在同一台 Mac 或 Windows 电脑上使用。Desktop 管理 Server、SQLite、存储位置选择和可选本机 Lumen。应用关闭后服务不再长期在线。

## 家庭 Server

适合多位浏览器用户和持续在线。管理员必须维护 Linux/Docker、两个持久挂载、账户暴露、网络、数据库快照和原件备份。

## 独立 Lumen 节点

适合把计算负载与媒体 Server 分开。节点只负责声明和执行支持的智能任务，基本媒体库应在节点下线时继续工作。

## 不要混淆的边界

- Desktop 不是只连接远程 Server 的轻客户端。
- Docker Web 当前不能像 Desktop 那样任意添加存储根。
- Lumen Intelligence 不是原件存储或数据库。
- Lumilio Agent 的模型提供方不等同于 Lumen 节点。

选择最少组件即可满足当前需求的方案，再为未来迁移保留备份和路径记录。
