---
title: "流明集帮助中心"
description: "按任务、症状和责任找到与当前代码一致的流明集中文文档。"
page_id: "index"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- TAG.txt
- HEAD.txt
- site/docs/.vitepress/sidebar/zh-cn.ts
-->

# 流明集帮助中心

这里是流明集（Lumilio Photos）的主文档。最有效的使用方式不是从头通读，而是先选择你现在要完成的任务：第一次安装、日常整理、处理故障，或者维护一座长期运行的媒体库。

## 从哪里开始

| 你的目标 | 建议入口 |
| --- | --- |
| 判断流明集是否适合自己 | [认识流明集](./getting-started/about.md) |
| 在个人电脑上首次使用 | [选择运行方式](./getting-started/choose-runtime.md) |
| 部署家庭 Server | [安装 Docker Server](./getting-started/install-docker.md) |
| 上传或接入第一批媒体 | [导入第一批媒体](./getting-started/import-first-media.md) |
| 找不到功能或操作方法 | [使用流明集](./use/index.md) |
| 界面报错、任务不前进 | [从症状开始排查](./troubleshooting/index.md) |
| 负责磁盘、账户、升级与恢复 | [管理与安全](./admin/index.md) |
| 想理解资源库和数据边界 | [了解原理](./concepts/index.md) |
| 查格式、目录、端口或状态 | [参考](./reference/index.md) |
| 阅读架构、API 或参与开发 | [开发者](./developer/index.md) |

## 文档中的角色

“用户”指浏览、整理、编辑或分享媒体的人；“管理员”指负责账户、资源库、存储、备份、升级和服务状态的人。在个人部署中，这两个角色往往是同一个人。只有明确涉及宿主机、配置文件或恢复流程时，文档才要求管理员权限。

## 事实与版本

本轮中文主文档逐页对照 baseline `86da6be7147fa9749c99b914cd79a5f677b92676`。每页源码顶部都保存了验证日期和主要代码依据；未能从当前实现唯一确定的行为会以 `TODO` 注释保留，而不会写成确定承诺。

::: warning 测试版边界
当前代码标识为 `v1.0.0-beta.4-177-g86da6be7`。测试版可以认真管理真实媒体，但仍应先以少量副本验证工作流，并始终保留独立、可恢复的原件与数据库备份。
:::
