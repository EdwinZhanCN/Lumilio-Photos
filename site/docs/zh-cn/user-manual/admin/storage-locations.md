---
title: "管理存储位置"
description: "创建、登记、断开和重新连接带可携带身份的父位置。"
page_id: "admin/storage-locations"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/rootcfg/root_config.go
- server/internal/storage/repository_roots.go
- server/internal/storage/provisioning.go
- desktop/internal/storage/controller.go
-->

# 管理存储位置

`.lumilioroot` 是 YAML 标记，当前版本 `1.0`，包含 UUID、名称和创建时间。它证明两个路径是否声称是同一个存储位置，但主机是否授权、当前是否可达仍由本地数据库记录决定。

## 默认存储位置

Server 配置中的 `storage.path` 是不可移除的默认存储位置。启动时服务确保目录和标记可用，并在这里建立主资源库。

## 外部存储位置

外部路径必须已经存在，服务不会静默创建缺失的挂载点。路径不能同时是资源库和存储根，也不能与现有根或资源库形成危险重叠。

Desktop 可以使用原生目录选择器添加位置。Server Web API 当前只有 `GET /repository-roots`，因此 Docker 部署应通过固定挂载恢复默认根，而不是期待网页登记宿主机任意路径。

## 备份

复制存储位置时必须保留隐藏标记。丢失 `.lumilioroot` 会使移动和复制判断失去原身份依据；手工重建一个新 UUID 也不会等同于原位置。
