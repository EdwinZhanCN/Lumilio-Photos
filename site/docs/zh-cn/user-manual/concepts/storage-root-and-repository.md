---
title: "存储位置身份与资源库身份"
description: "解释两层 UUID 如何支持移动、复制和多资源库。"
page_id: "concepts/storage-root-and-repository"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/rootcfg/root_config.go
- server/internal/storage/repocfg/repo_config.go
- server/internal/storage/repository_roots.go
- server/internal/storage/repo_relocate.go
-->

# 存储位置身份与资源库身份

`.lumilioroot` 和 `.lumiliorepo` 都包含 UUID，但用途不同。

- 根 UUID 回答“这是哪一个存储位置”；
- 资源库 UUID 回答“这是哪一座资源库”；
- 数据库保存“这台流明集实例是否授权并知道它位于哪里”。

因此，把一块磁盘移动到新挂载点时，标记仍能证明它是同一位置；复制一份时，两个路径会声称相同身份，系统必须要求操作者决定新副本是否获得新身份。

主资源库的目录固定为默认存储位置下的 `primary`。普通资源库可以位于同一存储位置的其他目录。Desktop 还能登记多个独立存储位置；Docker 默认部署通常只登记一个默认存储位置。

路径不是身份，名称也不是身份。不要通过改目录名或显示名称推断 UUID 已变化。
