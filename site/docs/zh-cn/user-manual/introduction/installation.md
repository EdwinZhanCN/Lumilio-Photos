<script setup lang="ts">
import DockerComposeConfigurator from '../../../.vitepress/components/DockerComposeConfigurator.vue'
</script>

# 安装

::: warning 注意
流明集当前处于 Beta 阶段。请先使用测试媒体或已有可靠备份的资料库进行试用，不要将本应用作为重要媒体的唯一存储位置。
:::

流明集可以作为 Desktop 应用运行，也可以通过 Docker 部署在 NAS 或 Linux 服务器上。如果只想在一台电脑上试用，优先选择 Desktop；如果需要持续运行并从多台设备访问，选择 Docker。

| 方式 | 适合用户 | 当前发布平台 |
| --- | --- | --- |
| Desktop | 普通用户、单机资料库 | macOS Apple Silicon、Windows x64 |
| Docker | NAS、Linux 服务器、多设备访问 | Linux amd64、Linux arm64 |
| 源码运行 | 开发者与贡献者 | 取决于本地开发环境 |

## Desktop

请前往 [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) 选择最新发布，并下载与系统匹配的文件。

### macOS Apple Silicon

下载名称包含 `macos-arm64` 的 DMG，打开后将流明集拖入“应用程序”。当前安装包为 ad-hoc 签名，尚未完成 Apple notarization。首次启动时，macOS 可能要求你在“系统设置 → 隐私与安全性”中确认打开。

macOS Intel 当前尚未进入 Desktop 发布矩阵。

### Windows x64

推荐下载名称包含 `windows-amd64-setup.exe` 的安装程序。如果不希望安装，也可以下载对应的便携 ZIP，解压后运行应用。

当前 Windows 发布文件尚未完成 Authenticode 签名，SmartScreen 可能显示警告。请只从项目的 GitHub Releases 下载，并核对文件名与发布版本。

### 首次启动

Desktop 会引导你选择语言、下载区域和 storage root。它会管理私有 PostgreSQL 数据库；Lumen Hub 为可选组件，只在你启用本地 AI 时另行下载。

::: tip 本地网络权限
如需通过 mDNS 发现其他 Lumen 节点，请允许 macOS 的本地网络访问或 Windows 防火墙提示。拒绝该权限不影响基础媒体管理。
:::

## Docker

Docker 发布由 GHCR 上的 `lumilio-server`、`lumilio-web` 和 `lumilio-db` 三个多架构镜像组成。Beta 版本应使用 GitHub Release 对应的精确镜像标签；`edge` 是手动测试通道，不建议用于重要资料库。

<DockerComposeConfigurator />

下载配置后，在其所在目录执行：

```bash
docker compose up -d
```

启动完成后，在部署主机上通过 `http://localhost:6657` 打开 Web 界面；同一局域网内的其他设备应将 `localhost` 替换为部署主机的局域网地址。配置器默认不将 Server API 端口暴露给宿主机；只有明确的集成需求时才应启用该选项。

::: danger 不要直接公开到互联网
默认 Compose 配置面向本地网络试用。如需远程访问，请先配置 HTTPS、可信的反向代理、认证与防火墙边界。
:::

Docker 中的 Lumen 网络模式边界请参阅 [Lumen AI](../features/lumen-ai.md)。该功能页的可执行配置仍在完善。

### 视频转码加速

Compose 配置器可以为视频转码选择 CPU、Intel/AMD VAAPI 或 NVIDIA NVENC。这项设置只影响 FFmpeg 视频转码，不会改变 Lumen 的 AI 推理后端。

| 选项 | 当前实现 | 宿主要求 |
| --- | --- | --- |
| CPU | `libx264` | 无需 GPU，兼容性最高 |
| Intel / AMD GPU | VAAPI，使用 `/dev/dri/renderD128` | Linux 驱动、`/dev/dri` 设备和正确权限 |
| NVIDIA GPU | `h264_nvenc` | NVIDIA 驱动、NVIDIA Container Toolkit 与支持 NVENC 的 FFmpeg |

::: warning 不要依赖未实现的自动选择
当前服务端虽接受 `auto` 和 `qsv` 配置值，但转码代码尚未实现自动探测或 QSV 分支，两者实际会使用 CPU。配置器因此不提供这两个选项。
:::

## 从源码运行

源码运行面向开发者与贡献者，不是普通用户的推荐安装方式。开发环境需要 Go、Node.js/Vite+、Docker、Make 以及项目使用的媒体工具。

```bash
make setup
make dev
```

具体版本、平台依赖和开发命令以当前仓库中的开发文档与 CI 配置为准。

## 安装后检查

1. 确认流明集界面可以打开，且首次设置页没有报告服务失联。
2. 确认 storage root 指向预期的本机目录或 Docker 持久化挂载。
3. 使用少量已有备份的照片完成首次试用。

继续阅读[首次使用](./first-use.md)。
