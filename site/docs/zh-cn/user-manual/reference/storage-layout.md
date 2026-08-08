---
title: "资源库存储目录"
description: "列出当前资源库创建的标准目录和用途。"
page_id: "reference/storage-layout"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/directory_manager.go
- server/internal/service/asset_service.go
-->

# 资源库存储目录

```text
repository/
├── .lumiliorepo
├── inbox/
└── .lumilio/
    ├── assets/
    │   ├── thumbnails/{small,medium,large}/
    │   ├── videos/web/
    │   ├── audios/web/
    │   └── faces/
    ├── sidecars/
    ├── staging/{incoming,failed}/
    ├── temp/
    ├── trash/
    └── logs/{app.log,error.log,operations.log}
```

`.lumilio/staging` 和 `.lumilio/temp` 以 `0700` 创建；其他标准目录以 `0755` 创建。实际可访问性仍受父目录、宿主文件系统、ACL 和容器挂载影响。

`assets` 是派生媒体，`sidecars` 是工作室编辑描述，`staging` 是受控提交暂存，`temp` 是瞬时处理，`logs` 是资源库级日志目标。

虽然存在 `.lumilio/trash` 和文件移动基础设施，当前普通资产删除路径没有接通它。不要根据目录名称推断网页“回收站”已经物理移动文件。
