---
title: "认识流明集"
description: "理解流明集的核心用途、适合人群和当前产品边界。"
page_id: "getting-started/about"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- README.md
- desktop/README.md
- server/internal/service/lumen_service.go
- server/internal/api/router.go
- web/src/app/router/routes.tsx
-->

# 认识流明集

流明集是一套以普通文件为原件、以资源库组织媒体，并提供浏览、搜索、整理、编辑、分享和可选智能能力的个人媒体系统。它既可以作为个人电脑上的 Desktop 应用运行，也可以作为长期在线的 Server 运行。

## 它适合什么情况

流明集适合已经拥有照片、RAW、视频或音频目录，希望在不把原件锁入私有容器的前提下建立统一照片收藏的人。它也适合愿意为家庭资源库负责存储、账户、备份和升级的一位管理员。

媒体浏览和基本整理不依赖 Lumen Intelligence。图像语义分析、OCR文字识别、人物识别、BioCLIP物种识别等能力需要可用的 Lumen Intelligence 节点；Lumilio Agent 还需要管理员配置受支持的模型提供方。

## 它当前不是什么

当前 baseline 没有移动客户端，因此不应把流明集视作手机照片自动备份方案。它也不是零运维托管服务：Docker 部署者仍需要管理挂载、权限、网络、数据库快照和媒体备份。

它还不是企业级数字资产管理系统。代码提供管理员和普通用户角色，但没有组织租户、审批流或细粒度企业治理模型。

## 产品组成

- **流明集（Lumilio Photos）**：Web、Server 与 Desktop 组成的媒体管理产品。
- **Lumen Intelligence**：独立的计算能力层，为流明集提供图像语义分析、OCR文字识别、人物识别和BioCLIP物种识别等任务。
- **Lumilio Agent**：在明确权限和确认机制下，通过模型提供方协助检索与整理媒体。

<!-- TODO(product-positioning): 当前注册入口在初始化后仍公开存在，因此“家庭成员由管理员邀请加入”并不符合代码。可选方向：关闭公开注册、邀请码、管理员创建账户。本文只描述现状。 -->
