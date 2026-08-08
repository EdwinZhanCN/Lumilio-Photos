---
title: "创建主资源库"
description: "理解固定主资源库路径、布局策略和同名冲突策略后完成初始化。"
page_id: "getting-started/primary-repository"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/storage/repo_provisioning.go
- server/internal/storage/repocfg/repo_config.go
- server/internal/storage/directory_manager.go
- web/src/features/auth/flows/bootstrap/BootstrapFlow.tsx
-->

# 创建主资源库

主资源库是实例进入正常应用的第二个门禁。每个实例只能有一个主资源库，并且必须先于普通资源库创建。

## 路径与名称

主资源库位于默认存储位置下的固定目录 `primary`。你在界面输入的名称是显示名称，不会改变这个目录名。默认存储位置来自 Desktop 运行时清单或 Server 配置；Web 初始化不能提交任意宿主机路径。

## 存储布局

- `date`：按日期组织新提交的媒体，适合希望目录可读的人。
- `flat`：媒体直接放在提交区的平面结构中。
- `cas`：按内容寻址组织，适合机器管理，不适合作为人工浏览目录。

## 同名冲突

- `rename`：在保留原始名称意图的同时生成不冲突名称。
- `uuid`：使用 UUID 生成目标名称。

同一资源库内的内容重复会先通过完整哈希和大小检测；同名冲突与内容重复是不同问题。

## 成功标志

创建后，默认存储位置下应出现 `primary/.lumiliorepo`、`primary/inbox/` 和 `primary/.lumilio/`。应用会解除主资源库门禁并进入正常页面。

<!-- TODO(terminology): 当前初始化界面部分文案使用“主仓库”，本文统一称“主资源库”。可选术语：仓库、媒体库；为区分整个产品级媒体库与单个 Repository，暂不采用“媒体库”。 -->
