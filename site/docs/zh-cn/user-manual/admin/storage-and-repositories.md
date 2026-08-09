---
title: "存储位置与资源库"
description: "建立 `.lumilioroot`、`.lumiliorepo`、主资源库和应用私有状态的准确心智模型。"
page_id: "admin/storage-and-repositories"
audience: "管理员、高级用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-08"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/rootcfg/root_config.go
- server/internal/storage/repocfg/repo_config.go
- server/internal/storage/doc.go
- desktop/internal/storage/controller.go
- server/internal/storage/host_action.go
- server/internal/storage/repo_open.go
- server/internal/storage/repository_candidates.go
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

Desktop 场景中，Web 持有用户可见的“添加存储位置”和“打开现有资源库”任务。请求会持久化并显示在 **Desktop 设置 → 存储 → 来自 Web 的请求**；只有本机用户点击批准后，Desktop 才会打开系统目录选择器。所选路径和一次性批准 nonce 不进入共享 HTTP API，Web 刷新后仍可通过任务 ID 继续观察进度。

创建资源库的主流程只要求：

1. 可修改的资源库显示名称；
2. 已登记且在线的存储位置；
3. 稳定的直接子目录名称。

显示名称与目录名称彼此独立。布局和重名文件处理使用 Server 的确定性安全默认值；云凭据连接与云导入在目标资源库存在后单独执行。如果目标目录已有有效 `.lumiliorepo`，创建会停止并引导到明确的“打开现有资源库”任务，不会隐式登记身份。

目标摘要会显示在线状态、可写或只读、可用与总容量以及已登记资源库数量。容量按所选路径本身读取，因此 Docker 子挂载不会错误沿用父路径的容量。文件系统类型保留在诊断信息中，不作为主要放置依据。

打开一个曾被移除的资源库前，流明集把旧 `.lumilio/` 私有状态移动到 `.lumilio/recovery/reopened-…`，建立干净工作区并排队进行权威首次扫描；原始媒体与 `.lumiliorepo` 保持原位。若身份已在另一位置登记，必须明确选择“作为移动后的原件使用”或“作为独立资源库添加”。登记路径仍在线时禁止移动；独立副本会隔离复制来的私有状态并写入新 UUID。

## Docker 候选目录与挂载

Docker 与独立 Server 只列出默认存储位置的直接子目录，并分类为：已登记、可打开、空且可写、非空但无标记、标记无效或身份冲突。身份冲突会提供与 Desktop 相同的“作为移动后的原件使用”和“作为独立资源库添加”选择；原登记位置仍在线时不能重新定位。HTTP 只接受单个可移植目录段，不能提交任意 Server 路径。

官方 Compose 使用长语法，并禁止 Docker 自动创建宿主源目录：

```yaml
services:
  lumilio:
    volumes:
      - type: bind
        source: ./lumilio/media
        target: /data/storage
        bind:
          create_host_path: false
      - type: bind
        source: /mnt/archive
        target: /data/storage/archive
        bind:
          create_host_path: false
```

运行 `docker compose up` 前先创建媒体、应用私有状态和 `/mnt/archive`。这样路径拼写错误会直接失败，不会静默生成空目录。若使用一个已存在的空直接子目录创建 Linux 资源库，Server 会从 `/proc/self/mountinfo` 验证它确实是挂载点；由流明集新建的普通直接子目录不要求是挂载点。

## 安全移除

从流明集中移除普通资源库只删除目录登记。确认框先显示目录影响并要求输入准确的资源库名称；主资源库和有活动任务的资源库不能移除。`.lumiliorepo`、`.lumilio/` 和全部媒体文件都保留在磁盘上，因此之后可以再次打开并重建目录。

外部存储位置仅在没有子资源库和活动生命周期操作时可以移除；默认存储位置不能移除。移除存储位置同样保留 `.lumilioroot` 与磁盘中的全部文件。

::: warning 子挂载不是独立存储身份
把物理磁盘挂在默认根的子目录可以提供容量和路径隔离，但移动检测、复制冲突和根级身份仍以默认 `.lumilioroot` 为边界。不要在未验证恢复流程前把它描述为多个独立存储位置。
:::
