---
title: "支持的媒体格式"
description: "列出当前后端按扩展名接受的照片、RAW、视频和音频格式。"
page_id: "reference/media-formats"
audience: "所有用户"
platform: "Desktop、Server"
baseline_commit: "86da6be7147fa9749c99b914cd79a5f677b92676"
last_verified: "2026-08-06"
verification_status: "verified"
---

<!--
code-evidence:
- server/internal/utils/file/validator.go
- server/internal/utils/imaging/process.go
- web/src/features/studio/flows/editor/export/ExportPanel.tsx
-->

# 支持的媒体格式

## 照片

`jpg`、`jpeg`、`png`、`webp`、`gif`、`bmp`、`tiff`、`tif`、`heic`、`heif`

## RAW

`cr2`、`cr3`、`nef`、`arw`、`dng`、`orf`、`rw2`、`pef`、`raf`、`mrw`、`srw`、`rwl`、`x3f`

## 视频

`mp4`、`mov`、`avi`、`mkv`、`webm`、`flv`、`wmv`、`m4v`、`3gp`、`mpg`、`mpeg`、`m2ts`、`mts`、`ogv`

## 音频

`mp3`、`aac`、`m4a`、`flac`、`wav`、`ogg`、`aiff`、`wma`、`opus`、`oga`

## 导出编码

Server 图像导出支持 JPEG、PNG、WebP 和 AVIF；工作室实际显示的格式以当前界面为准。

::: warning 校验方式
当前上传和扫描的初始支持判断以文件名扩展名为主，传入 Content-Type 不决定最终类型。扩展名受支持不保证损坏文件或罕见编码一定能被解码、提取或转码。
:::
