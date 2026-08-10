---
title: "卸载与保留数据"
description: "在移除 Desktop 或 Docker 服务前区分原件、资源库工作区和应用私有状态。"
page_id: "getting-started/uninstall"
audience: "所有者、管理员"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- desktop/internal/platform/paths.go
- server/internal/storage/doc.go
- server/config/schema/lumilio-server.schema.json
- deploy/compose/compose.yml
-->

# 卸载与保留数据

卸载应用程序本身不会等同于删除媒体，但错误地删除挂载或应用私有状态会使资源库关系、账户、相册和处理记录丢失。操作前先创建完整备份。

## 必须区分的三类数据

1. **媒体存储**：包含普通原件、`.lumilioroot`、各资源库及其 `.lumiliorepo`、`inbox/` 和 `.lumilio/`。
2. **应用私有状态**：包含 SQLite 数据库、配置、密钥、数据库快照、云会话和应用日志。
3. **程序文件**：Desktop 应用或 Docker 镜像，可以重新安装。

仅删除程序文件，通常可以保留前两类数据。删除应用私有状态会丢失账户、资源库登记、相册、标签、人物和任务状态；只保留数据库而不保留原件，也不能恢复完整照片收藏。

## 安全步骤

1. 停止写入和导入。
2. 创建数据库快照，并在宿主机层把 `.sqlite3` 与配套 manifest 一起复制到独立位置；界面下载的单个 SQLite 只作补充副本。
3. 复制全部媒体存储，并验证原件可读。
4. 记录当前版本、挂载和网络设置。
5. 停止 Desktop 运行时或 Docker 容器。
6. 只删除你已经明确识别为程序文件的内容。

<!-- TODO(uninstall-ux): Desktop 当前没有经过代码验证的一键“保留数据卸载”向导。可选方向：提供导出清单、状态目录定位和卸载前检查。 -->
