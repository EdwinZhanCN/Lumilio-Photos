---
title: "OpenAPI 与 API 文档"
description: "从 Go DTO 和注解生成契约，避免手工维护漂移。"
page_id: "developer/openapi"
audience: "开发者"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- taskfile.yml
- server/internal/api/handler
- server/internal/api/dto
- server/internal/api/router.go
- web/src/lib/http-commons/schema.d.ts
-->

# OpenAPI 与 API 文档

Server handler、DTO 与注解是 OpenAPI 生成输入。修改请求、响应、状态或路由后运行：

```bash
task dto
```

该任务依次生成 Server OpenAPI、Web TypeScript 类型和站点 API 文档。前端不应通过强制类型断言掩盖契约漂移；先检查后端 DTO、`@Success` 注解和生成 schema。

公共 API 的存在必须与路由实际注册一致。资源库重命名仍未开放；安全移除路由只清理流明集目录和应用私有状态，并明确保留磁盘文件，不能把它描述成物理删除。
