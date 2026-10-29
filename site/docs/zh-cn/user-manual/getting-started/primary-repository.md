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

## 如果这里已经有一个主资源库

当默认存储位置的 `primary/` 文件夹里已经有早先安装留下的资源库时，初始化不会显示创建表单，而是显示这个资源库并提供**接管主资源库**：身份、原始文件、回收站和 Studio 编辑都会保留，扫描会重新登记这些文件。相册、人物、评分和分享链接保存在目录中，要找回它们仍需兼容的数据库备份。如果想重新开始，先更换默认存储位置（Desktop：**设置 → 存储**；Docker：`storage.path`），旧文件夹会原样保留。没有标记的文件、无法读取的标记，或由更新版本写入的标记，绝不会被初始化覆盖；初始化会说明是哪一种情况并保持原样。

## 成功标志

创建或接管后，默认存储位置下应出现 `primary/.lumiliorepo`、`primary/inbox/` 和 `primary/.lumilio/`。应用会解除主资源库门禁并进入正常页面。
