<script setup lang="ts">
import DockerComposeConfigurator from '../../../.vitepress/components/DockerComposeConfigurator.vue'
</script>

# 安装

如果安装后所有管理员都无法登录，请按照[恢复管理员访问权限](./break-glass.md)操作，不要直接编辑数据库。

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

Desktop 会引导你选择语言和下载区域，并显示本机默认存储位置。外部目录在启动完成后通过 Desktop 控制面板授权；不要把“挂载资源库”用于初始化空目录。Desktop 会管理私有 PostgreSQL 数据库；Lumen Hub 为可选组件，只在你启用本地 AI 时另行下载。

::: tip 本地网络权限
如需通过 mDNS 发现其他 Lumen 节点，请允许 macOS 的本地网络访问或 Windows 防火墙提示。拒绝该权限不影响基础媒体管理。
:::

## Docker

Docker 发布使用 GHCR 上的单个 `lumilio-server` 多架构镜像，同一容器同时提供 Web 界面和 API。Beta 版本应使用 GitHub Release 对应的精确镜像标签；`edge` 是手动测试通道，不建议用于重要资料库。

<DockerComposeConfigurator />

下载配置后，在其所在目录执行：

```bash
docker compose up -d
```

启动完成后，在部署主机上通过 `http://localhost:6657` 打开 Web 界面；同一局域网内的其他设备应将 `localhost` 替换为部署主机的局域网地址。该端口同时承载 Web 界面和 API。

媒体目录与应用状态目录是两个必需且独立的持久化挂载。媒体目录保存原始媒体；应用状态目录保存 `library.sqlite3`、应用快照、凭据和日志，并应位于本机可靠磁盘。两个目录都必须允许容器 UID 10001 写入。只要保留这两个宿主目录，就可以删除并重新创建应用容器而不丢失状态。

::: danger 不要直接公开到互联网
默认 Compose 配置面向本地网络试用。如需远程访问，请先配置 HTTPS、可信的反向代理、认证与防火墙边界。
:::

::: warning 通过流明集创建备份
容器运行时不要直接复制 `library.sqlite3`、`-wal` 或 `-shm`。请在“设置 → 服务器”中创建并下载一致性快照，并单独备份媒体目录。
:::

Docker 中的 Lumen 网络模式边界请参阅 [Lumen AI](../features/lumen-ai.md)。不可变运行策略需要通过完整 schema v2 manifest 配置；普通环境变量不会覆盖这些字段。

## 从源码运行

源码运行面向开发者与贡献者，不是普通用户的推荐安装方式。开发环境需要 Go、Node.js/Vite+、Docker、Make 以及项目使用的媒体工具。

```bash
make setup
make dev
```

具体版本、平台依赖和开发命令以当前仓库中的开发文档与 CI 配置为准。

## 安装后检查

1. 确认流明集界面可以打开，且首次设置页没有报告服务失联。
2. 确认默认存储位置指向预期的本机目录或 Docker 持久化挂载。
3. 使用少量已有备份的照片完成首次试用。

继续阅读[首次使用](./first-use.md)和[存储位置与仓库](./repositories.md)。
