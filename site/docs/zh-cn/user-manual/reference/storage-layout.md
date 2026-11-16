---
title: "资源库存储目录"
description: "列出资源库的标准目录、各自的用途、能否删除以及是否需要备份。"
page_id: "reference/storage-layout"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "c2fd359863d31d0f533dd38eddeecb6a997c3416"
last_verified: "2026-11-16"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/layout/classes.go
- server/internal/storage/layout/paths.go
- server/internal/storage/layout/tree.go
- server/internal/storage/recovery/startup.go
- server/internal/foundation/logging/logger.go
-->

# 资源库存储目录

每个资源库都是一个普通文件夹。原始文件留在你放置的位置；Lumilio 自己的数据只放在一个隐藏文件夹 `.lumilio/` 中，并按数据的性质和处理方式分类。

```text
<资源库>/
├── .lumiliorepo          资源库的标识与设置
├── .lumiliorepo.lock     服务器使用该资源库期间持有
├── inbox/                上传和云导入的落地位置
├── <你的文件夹>/          原始文件
└── .lumilio/
    ├── user/             人工创作的内容：Studio 编辑、资产 sidecar
    ├── trash/            已删除的原始文件，保留到回收站过期
    ├── cache/            缩略图、网页版本、波形、人脸裁切
    ├── work/             进行中的上传和写入
    ├── quarantine/       等待你决定的字节
    └── logs/             该资源库的操作日志和错误日志
```

Lumilio 还可能在 `.lumiliorepo` 旁边创建名为 `.lumilio-tmp-…` 的短暂文件；它们会自行消失，也从不被当作媒体。

## 各目录的内容

| 目录 | 内容 | 可以删除吗？ | 需要备份吗？ |
| --- | --- | --- | --- |
| `user/` | Studio 编辑和资产 sidecar。由人创作，无法重建。 | 否 | 是 |
| `trash/` | 你删除的原始文件，直到回收站的保留期使其过期。 | 只能通过回收站 | 是 |
| `cache/` | 缩略图、可在网页播放的版本、波形和人脸裁切。缺失的内容会被重建。 | 可以，在服务器停止时整个删除 | 否（带有 `CACHEDIR.TAG`） |
| `work/` | 进行中的上传和写入。 | 否 | 否 |
| `quarantine/` | 失败的上传，以及启动或重新打开资源库时无法归位的内容。 | 只能在恢复清单中删除 | 是 |
| `logs/` | 按大小轮转的操作日志和错误日志。 | 可以 | 可选 |

`.lumilio/` 下的其他内容（例如更早或更新的版本写入的文件夹）会原样保留，并显示为未知内容。Lumilio 从不删除它们。

## Lumilio 自动执行的操作

- **每次启动时**，Lumilio 会重建上表中缺失的目录。中断的运行留在 `work/` 中的内容会移到 `quarantine/recovery/`；仍在等待导入的上传和浏览器可以续传的上传除外。不会删除任何内容。移动的文件会出现在“存储”页该资源库的**恢复清单**中，你可以逐项导出或删除。
- **运行期间**，没有任何操作会完成的上传在闲置一小时后被移除；无法拼合的上传则移到 `quarantine/uploads/`。
- **缓存**会清理不再被任何资产引用的文件。
- **日志**在 50 MB 时轮转，最多保留五个压缩副本，保存 14 天。

## 备份

备份整个资源库文件夹；如需节省空间，可以排除 `.lumilio/cache/` 和 `.lumilio/work/`。支持 `CACHEDIR.TAG` 的备份工具会自动跳过 `cache/`。不要在服务器运行时只把 `.lumilio/` 的一部分恢复回去。

不要把文件移入 `.lumilio/`，也不要手工编辑其中的文件。
