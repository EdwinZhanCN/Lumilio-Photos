---
title: "扫描已有目录"
description: "让流明集分批观察资源库文件，并理解进度、增量更新和安全缺失协调。"
page_id: "use/scan"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-22"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/storage/roe/controller/controller.go
- server/internal/storage/roe/changefeed
- server/internal/storage/roe/materializer
- server/config/config.go
- server/internal/db/repo/queries
-->

# 扫描已有目录

扫描会启动资源库观察操作，分批登记资源库中的普通媒体文件。它包括 `inbox/**`，但跳过应用私有的 `.lumilio/**`。扫描适合文件已经位于资源库中的场景，不负责把宿主机任意路径自动挂载进 Docker。

## 手动与周期扫描

默认运行时每 300 秒请求一次周期观察，并对最近仍在变化的文件保留 5 秒稳定窗口。每个资源库只有一个逻辑控制器；重复点击不会启动互相竞争的遍历，而是返回已插入或已合并的持久操作编号。

请求返回后，界面会分别显示排队、遍历、追赶变化、最终确认以及完成、部分完成、失败或已取消状态。目录和文件观察、等待哈希的字节、已哈希字节、错误覆盖和剩余后台任务会逐步更新；导航和其他资源库操作不需要等待扫描结束。

首次观察会分批遍历。此后，受支持的本地文件系统使用 Windows USN/目录变化通知、macOS FSEvents 或 Linux inotify 作为变化提示；健康游标下没有变化的增量操作不会重新遍历整棵目录。周期完整验证仍会修复遗漏提示。

## 文件移动与删除

目录移动在本机身份明确时只更新一个目录关系，不会重写或重新哈希所有后代。文件内容使用完整 BLAKE3 和大小确认：同一所有者的相同字节复用一个资产，并为每个实际副本保留独立位置。

文件变化通知本身不能证明文件已经消失。只有没有读取错误、已经追赶完变化且完整枚举过父目录的观察，才会关闭缺失位置；离线资源库、权限错误、游标缺口、通知溢出、取消或部分遍历都会保留先前有效位置。外置或不可靠卷还会经过持久稳定等待。缺失协调不会删除磁盘原件，也不会代表用户的“移入回收站”意图；文件重新出现且内容相同时会恢复到原有资产。
