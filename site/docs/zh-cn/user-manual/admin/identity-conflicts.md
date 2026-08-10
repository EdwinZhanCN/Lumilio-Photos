---
title: "移动、复制与身份冲突"
description: "使用可携带 UUID 区分同一存储或资源库的重定位与克隆。"
page_id: "admin/identity-conflicts"
audience: "管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/storage/repository_roots.go
- server/internal/storage/repo_relocate.go
- server/internal/storage/repo_manager.go
- server/app/repository_control.go
- desktop/internal/runtime/repository.go
- desktop/internal/storage/controller.go
-->

# 移动、复制与身份冲突

当相同 `.lumilioroot` 或 `.lumiliorepo` UUID 在新路径出现时，系统不能自动知道你是移动了原位置，还是复制了一份仍与旧位置同时存在的克隆。

## 移动

原位置已经不再使用，身份应继续指向新路径。重定位保留数据库关系和资源库身份。

## 复制

旧位置和新位置都会继续存在。新副本必须显式注册为新的独立资源库身份，避免两个可写路径声称是同一资源库。

## 安全流程

1. 停止对源位置写入。
2. 创建数据库快照和媒体备份。
3. 完整复制，包括隐藏标记与 `.lumilio/`。
4. 决定旧位置是否继续存在。
5. 尝试附加新路径并保留身份冲突信息。
6. 只有在产品提供明确的“重定位”或“注册副本”控制后再选择对应动作。
7. 验证单个资源库后再恢复扫描。

底层控制面已经实现两种动作，但当前 Desktop 适配器没有把冲突解决方法暴露给 UI，Web 也没有相应公共路由。手工删除或改写 UUID 可能绕过冲突检测并制造不可恢复的数据库映射，不能作为常规修复方法。

<!-- TODO(storage-conflict-ui): Server 内部 RepositoryControl 支持重定位与注册副本，但 Desktop 的 StorageControl/StorageService 只暴露添加与附加。可选方向：把结构化冲突和两种动作贯通到 Desktop UI；或提供仅管理员可用的受审计恢复入口。 -->
