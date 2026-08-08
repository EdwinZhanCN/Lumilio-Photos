---
title: "内容重复与同名冲突"
description: "区分字节内容、视觉相似和目标路径命名冲突。"
page_id: "concepts/duplicate-vs-conflict"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/api/handler/asset_handler.go
- server/internal/service/duplicate_service.go
- server/internal/storage/repocfg/repo_config.go
- server/internal/sourcing/materializer.go
-->

# 内容重复与同名冲突

**内容重复**表示同一资源库中已有相同完整哈希和文件大小的可验证文件。上传可以直接返回重复，不再保留第二份暂存内容。

**视觉相似**由感知哈希等检测得到，只说明画面近似，不保证字节相同。合并时应人工选择保留项；人脸关系只有完全重复组才允许迁移。

**同名冲突**表示两个不同内容希望写入同一目标文件名。资源库配置使用 `rename` 或 `uuid` 生成不冲突名称。

所以“同名”不等于“重复”，“相似”也不等于可以安全删除。界面应展示检测方法和保留项，而不是只给一个节省空间数字。
