---
title: 升级流明集（Lumilio Photos）
description: 升级前先备份，然后按 Desktop 或 Docker 分别完成升级。
---

# 升级流明集（Lumilio Photos）

**升级前先创建一份数据库备份**（[备份与数据完整性](./integrity)），并确认没有正在运行的导入或编辑任务。升级不修改你的媒体和数据库。

## Desktop（macOS / Windows）

1. 菜单栏或系统托盘出现 **Update available** 时点击；
2. 下载新版本安装文件（按[安装页](./installation)校验 SHA-256）；
3. 运行安装程序或替换「应用程序」中的应用；
4. 重新启动。

**成功标志**：启动后进入主界面，设置 → 关于 中版本号已更新。
**失败恢复**：删除新版本，重新安装上一个可用版本；媒体和数据库不受影响。

## Docker（Linux / NAS）

在部署目录执行：

```bash
docker compose pull
docker compose up -d
docker compose ps
```

**成功标志**：`lumilio` 服务健康，版本已更新。
**失败恢复**：查看 `docker compose logs lumilio`；必要时恢复旧镜像标签并重新启动。媒体目录（`./lumilio/media`）和应用状态（`./lumilio/app-state`）在容器重建后保留，只要没有手动删除。

> 升级后如发现数据或功能异常，回滚到上一版本前先保留当前版本的日志和错误样本（见[诊断与日志](../features/monitor)）。
