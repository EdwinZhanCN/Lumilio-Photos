---
title: "添加媒体：上传、扫描或云导入"
description: "根据文件当前位置和管理责任选择正确接入方式。"
page_id: "use/add-media"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-22"
verification_status: "verified"
---

<!--
code-evidence:
- web/src/features/manage
- server/internal/api/router.go
- server/internal/api/handler/asset_handler.go
- server/internal/storage/roe/controller/controller.go
- server/internal/storage/roe/materializer
-->

# 添加媒体：上传、扫描或云导入

“管理”页面把多种接入方式集中在一起，但它们的数据流不同。

| 方式 | 文件从哪里来 | 流明集如何处理 | 适合场景 |
| --- | --- | --- | --- |
| 上传 | 浏览器或本机选择的文件 | 经过暂存、哈希和提交后进入目标资源库 `inbox/` | 少量新增、明确复制 |
| 扫描 | 已经位于资源库内 | 分批观察普通文件，逐步验证内容并登记实际位置 | 已有目录、大批量接入 |
| 云导入 | 已连接的 iCloud 账户 | 按导入任务获取内容并保存到目标资源库 | 从当前支持的云提供方迁入 |

不要把扫描当作“选择任意电脑文件夹”。Server 只能观察它在资源库路径内实际可见的文件；Docker 中还必须先通过挂载把宿主机路径呈现给容器。扫描请求完成只表示后台操作已经持久化，不表示全部文件已经完成哈希或派生处理。

管理员负责创建资源库、触发扫描、重复项检测、堆叠检测和索引工具。普通用户可以使用通用上传，但最终权限仍由 API 强制执行。
