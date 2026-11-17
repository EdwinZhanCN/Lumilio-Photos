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
- server/internal/storage/storage_locations.go
- server/internal/storage/provisioning.go
- desktop/internal/storage/controller.go
-->

# 管理存储位置

`.lumilioroot` 是 YAML 标记，当前版本 `1.0`，包含 UUID、名称和创建时间。它证明两个路径是否声称是同一个存储位置，但主机是否授权、当前是否可达仍由本地数据库记录决定。

## 默认存储位置

Server 配置中的 `storage.path` 是不可移除的默认存储位置。启动时服务确保目录和标记可用，并在这里建立主资源库。

## 外部存储位置

外部路径必须已经存在，服务不会静默创建缺失的挂载点。路径不能同时是资源库和存储位置，也不能与现有存储位置或资源库形成危险重叠。

Desktop 可以使用原生目录选择器添加位置；Web 也可以创建一项短时、持久化的审核任务，由已连接的 Desktop 在本机选择目录。路径和批准凭据不会进入共享 HTTP 合同。Docker 部署不能从 Web 登记任意宿主机路径，应使用固定挂载；Web 只会检查默认存储位置下的一级候选目录。

外部位置离线或移动后，可在 Web 选择“重新连接存储位置”，再由本机 Desktop 批准并定位含同一 `.lumilioroot` 身份的新路径；验证通过后，全部直接子资源库路径会原子更新。每个资源库遵循存放它的那块磁盘的文件名规则（`Photo.jpg` 与 `photo.jpg` 是否同一个文件、两种重音拼写是否同一个名字），该规则在创建、打开或重新连接时在文件夹上实际测量得出。重新连接到规则不同的磁盘（例如 exFAT 或 FAT32 移动硬盘，或区分大小写的卷）时，会先检查没有两个已索引文件会在新磁盘上合并成同一个名字，再在资源库短暂处于维护状态期间重建索引；不会移动任何文件，之后的扫描也不会报告缺失。新磁盘无法区分的两个文件，或无法测量规则的磁盘，都会被拒绝并给出原因。受支持的磁盘以 Lumilio 自身测试在真实镜像上验证过的为准：FAT32、exFAT、ext4（普通及大小写折叠）、APFS（不区分与区分大小写）、HFS+ 以及 macOS ExFAT。没有子资源库和活动生命周期操作的外部位置也可从流明集中移除，目录、标记和磁盘文件都会保留。默认存储位置不能通过这个操作移除或重新定位。

## 备份

复制存储位置时必须保留隐藏标记。丢失 `.lumilioroot` 会使移动和复制判断失去原身份依据；手工重建一个新 UUID 也不会等同于原位置。
