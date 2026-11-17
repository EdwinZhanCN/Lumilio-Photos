---
title: "事件"
description: "按时间和地点生成事件，并使用重命名、合并、拆分和成员管理修正结果。"
page_id: "use/events"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified-with-todo"
---

<!--
code-evidence:
- server/internal/event
- server/internal/event/service.go
- web/src/features/events
- web/src/locales/zh/translation.json
-->

# 事件

事件仅根据照片和视频（包括实况照片）的时间和地点派生，用于把一段活动集中显示。初始结果是算法组织，不是不可变事实；用户可以重命名、选择封面、隐藏、合并、拆分、移动或移除成员。

音频仅在音乐中浏览，不参与事件自动分组；即使录音带有拍摄时间，也不能在 rc.1 中手动加入事件。事件数量、封面、媒体列表、查看器前后导航和事件分享均使用同一组照片和视频成员；资源库浏览范围只筛选显示结果，不改变事件本身。重建保留照片和视频成员的人工修正。直接打开或分享的音频仍可播放。

从事件移除媒体只解除事件成员关系，媒体仍保留在资源库。隐藏事件不会删除内容。拆分操作以选中媒体为边界；合并前应确认目标事件 ID 和成员范围。

管理员可以重建事件。重建会根据当前元数据重新计算自动结果，并可能改变尚未人工固定的分组；执行前应建立数据库快照，并在完成后抽查人工编辑是否按预期保留。

<!-- TODO(event-contract): 当前用户界面需要通过事件 ID 完成部分合并或移动操作，暴露了实现标识。可选方向：可搜索事件选择器、时间线拖放、冲突预览。 -->
