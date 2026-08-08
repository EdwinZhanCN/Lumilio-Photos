---
title: "存储位置与资源库"
description: "建立 `.lumilioroot`、`.lumiliorepo`、主资源库和应用私有状态的准确心智模型。"
page_id: "admin/storage-and-repositories"
audience: "管理员、高级用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/storage/rootcfg/root_config.go
- server/internal/storage/repocfg/repo_config.go
- server/internal/storage/doc.go
- desktop/internal/storage/controller.go
- deploy/compose/compose.yml
-->

# 存储位置与资源库

流明集把“存储位置”和“资源库”分成两层。

- **存储位置**是可以携带、断开和重新定位的父位置，由根目录中的 `.lumilioroot` 标识。
- **资源库**是一个具体媒体单元，由资源库目录中的 `.lumiliorepo` 标识。
- **应用私有状态**位于存储位置之外，保存 SQLite、配置、密钥、云会话、应用日志和数据库快照。

一个存储位置可以容纳多个资源库。每个实例只有一个主资源库，位于默认存储位置的 `primary` 目录；普通资源库只能在主资源库之后创建。

## Inbox 与文件整理

上传和云导入完成后，原始文件默认落在 `inbox/`。`inbox/` 只是落点，不是锁定区：确认上传已经完成后，用户可以用 Finder、资源管理器或其他文件系统工具，在同一资源库内移动或重命名文件。资源库扫描会包含 `inbox/`；只有在完整内容哈希形成唯一的一一对应关系时，才会把新路径关联回原资源，并保留资源 ID 及其相册、评分、人物等目录关系。

扫描不会替用户整理、移动或复制原始文件。如果一个缺失路径对应多个内容相同的新路径，扫描会报告“存在歧义”并拒绝猜测；移除多余副本或恢复为一一对应布局后，再次扫描即可继续处理。仍在写入的文件或无法完整读取的目录会让本轮扫描成为部分扫描，不能作为删除原始文件的证据。

资源库内只有 `.lumilio/` 是应用私有子树，其中保存暂存文件和派生资源，请勿手动修改。`.lumiliorepo` 是资源库身份标记；包括 `inbox/` 在内的其他普通目录都属于用户可见的媒体树。

## Desktop 与 Docker

Desktop 可以登记多个带不同 `.lumilioroot` 身份的存储位置。Docker 默认只有配置中的 `/data/storage` 作为已登记默认存储位置；管理员可以把多块宿主磁盘分别挂载为这个根下的不同子目录，并在其上创建资源库，但这些子挂载不会自动变成独立 `.lumilioroot` 身份。

::: warning 子挂载不是独立存储身份
把物理磁盘挂在默认根的子目录可以提供容量和路径隔离，但移动检测、复制冲突和根级身份仍以默认 `.lumilioroot` 为边界。不要在未验证恢复流程前把它描述为多个独立存储位置。
:::

<!-- TODO(storage-model): 用户给出的目标模型是 Desktop 可有多个 `.lumilioroot`，Docker 通常只有一个并可含多个物理子挂载。代码的 Web API 只列根、Desktop 才有添加控制面。可选方向：Server 管理 API 添加受控根注册，或正式把 Docker 子挂载定义为 Repository mount。 -->
