---
title: 安装 Lumilio Photos
description: 按 macOS、Windows 或 Linux 选择正式支持的安装方式。
---

# 安装 Lumilio Photos

先确认 Lumilio Photos 将运行在哪一台设备上，再选择对应的安装方法。如果你准备用一台电脑访问部署在 Linux 服务器上的 Lumilio Photos，应选择 **Linux**，而不是浏览器所在电脑的系统。

| 运行 Lumilio Photos 的设备 | 安装方式 | 当前支持的平台 |
| --- | --- | --- |
| Mac | Desktop 应用 | Apple Silicon |
| Windows 电脑 | Desktop 应用 | Windows 10/11 x64 |
| Linux 服务器或 NAS | Docker Server | Linux amd64、arm64 |

::: warning 仍处于测试阶段
Lumilio Photos 目前是 Beta 软件。请先使用测试媒体或已有可靠备份的媒体库试用，不要把它作为重要媒体的唯一副本。
:::

## macOS

macOS 版本是常驻菜单栏的 Desktop 应用，适合主要在一台 Mac 上使用。应用已经包含数据库和媒体处理工具，不需要另外安装数据库。

### 检查 Mac 是否受支持

打开苹果菜单中的 **关于本机**，查看“芯片”一项：

- 显示 Apple M 系列芯片时，可以安装当前版本；
- Intel Mac 暂不在正式发布范围内。

### 下载并安装

1. 打开项目的 [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) 页面，进入最新版本。
2. 下载文件名中包含 `macos-arm64` 的 `.dmg` 文件。
3. 打开下载的 DMG，把 **Lumilio Photos** 拖入“应用程序”文件夹。
4. 从“应用程序”中启动 Lumilio Photos。

当前 macOS 应用尚未经过 Apple 公证。第一次启动时，系统可能会阻止打开：

1. 先尝试启动一次应用；
2. 打开 **系统设置 → 隐私与安全性**；
3. 找到 Lumilio Photos 的提示，选择 **仍要打开**；
4. 再次确认启动。

请只对从项目 GitHub Releases 下载的安装文件执行这一步。

### 第一次启动后

Lumilio Photos 不会显示在程序坞中，而是常驻菜单栏。应用会先打开 Desktop 控制面板中的首次设置向导；完成设置后，本机服务启动，默认浏览器会打开：

```text
http://localhost:6680
```

看到“首次运行设置”欢迎页面，表示 Desktop 已经安装并成功启动。接下来继续[完成首次设置](./first-use)。

::: tip 关闭浏览器不会退出应用
Web 主界面运行在浏览器中，但后台服务由菜单栏应用管理。需要完全退出时，请使用菜单栏中的退出命令。
:::

## Windows

Windows 版本是常驻系统托盘的 Desktop 应用，支持 Windows 10 和 Windows 11 x64。应用已经包含数据库和媒体处理工具。

### 选择安装文件

项目发布两个 Windows 文件：

| 文件 | 适合谁 |
| --- | --- |
| 文件名包含 `windows-amd64-setup.exe` | 大多数用户，推荐 |
| 文件名包含 `windows-amd64.zip` | 不希望运行安装程序的用户 |

安装程序会创建开始菜单入口、注册卸载程序，并在系统缺少 Microsoft Edge WebView2 Runtime 时尝试自动安装。便携 ZIP 不会完成这些操作。

### 使用安装程序（推荐）

1. 打开项目的 [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) 页面，进入最新版本。
2. 下载文件名中包含 `windows-amd64-setup.exe` 的文件。
3. 运行安装程序。它按当前用户安装，不需要管理员权限。
4. 安装完成后，从开始菜单启动 **Lumilio Photos**。

当前 Windows 安装程序尚未进行 Authenticode 签名。如果 SmartScreen 显示“Windows 已保护你的电脑”，请确认文件来自项目 GitHub Releases，然后选择 **更多信息 → 仍要运行**。

### 使用便携 ZIP

1. 从同一发布页面下载文件名中包含 `windows-amd64.zip` 的文件。
2. 把 ZIP 完整解压到一个长期保留的文件夹，不要直接在压缩包中运行。
3. 打开解压后的 `Lumilio Photos` 文件夹，运行 `lumilio-photos.exe`。

便携版需要系统已经安装 Microsoft Edge WebView2 Runtime，否则 Desktop 控制面板无法显示。拿不准时，请使用上面的安装程序。

### 第一次启动后

Lumilio Photos 会常驻系统托盘，并先显示 Desktop 控制面板中的首次设置向导。完成设置后，本机服务启动，默认浏览器会打开：

```text
http://localhost:6680
```

看到“首次运行设置”欢迎页面，表示 Desktop 已经安装并成功启动。接下来继续[完成首次设置](./first-use)。

::: tip 关闭浏览器不会退出应用
需要完全退出 Lumilio Photos 时，请使用系统托盘中的退出命令。
:::

## Linux

Linux 版本是通过 Docker Compose 运行的 Server，适合需要在服务器或 NAS 上持续运行，并通过浏览器访问的用户。第一次启动不需要域名、HTTPS 或手工编写配置。

### 开始前准备

你需要：

- 一台 amd64 或 arm64 Linux 主机；
- Docker Engine 和 Docker Compose 2.23.1 或更高版本；
- 当前用户能够运行 Docker；
- 局域网中的其他设备能够访问这台主机的 TCP 6680。

先确认 Docker Compose 版本：

```bash
docker compose version
```

### 启动 Server

新建一个部署目录，在其中下载默认 Compose 文件并启动：

```bash
mkdir lumilio-server
cd lumilio-server
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.yml
docker compose up -d
```

默认会在当前目录创建两个持久化目录：

- `./lumilio/media` 保存原始媒体和资源库；
- `./lumilio/app-state` 保存 SQLite 数据库、凭据、日志和数据库快照。

删除容器不会删除这两个目录。升级或迁移前，仍应分别备份媒体目录和应用状态。

### 检查 Server 是否启动

```bash
docker compose ps
```

当 `lumilio` 服务显示为健康状态后，在同一局域网的浏览器中打开：

```text
http://Linux主机的IP地址:6680
```

例如主机地址是 `192.168.1.20`，就打开 `http://192.168.1.20:6680`。看到“首次运行设置”欢迎页面，表示 Server 已经安装并成功启动。

如果服务没有就绪，先查看日志：

```bash
docker compose logs lumilio
```

::: warning 局域网 HTTP 不会加密
默认部署便于先完成安装，但 HTTP 流量可能被同一网络中的其他设备读取。密码和 TOTP 可以使用，远程浏览器不能使用通行密钥。需要长期远程访问或使用通行密钥时，再配置[HTTPS 与通行密钥](../help/https)。
:::

### 使用其他目录

如果不希望把数据放在部署目录中，可以在启动前指定两个路径：

```bash
export LUMILIO_STORAGE=/srv/lumilio/media
export LUMILIO_STATE=/srv/lumilio/app-state
mkdir -p "$LUMILIO_STORAGE" "$LUMILIO_STATE"
docker compose up -d
```

`LUMILIO_STORAGE` 是媒体目录，`LUMILIO_STATE` 是应用状态目录。不要让二者相互包含。

如果需要让不同资源库分别位于多块磁盘，不要增加第二个 `/data/storage`。完成首次设置后，按照[Server 挂载资源库](./repositories#server)把每块磁盘挂载到 `/data/storage/<资源库名称>`。

::: tip 为什么使用 host network
默认 Compose 使用 Linux host network，让 Server 可以发现局域网中的 Lumen 节点，无需添加 `ports`。这是 Docker 版 Lumen 自动发现的支持边界；如果平台不能创建 host network 项目，当前官方路径不支持用该平台部署 Docker 版 Lumen Hub。具体流程见 [Lumen AI](../features/lumen-ai)。
:::

::: danger 不要直接复制运行中的数据库
Lumilio 运行时，不要使用主机上的 SQLite 工具打开或复制 `library.sqlite3`、`-wal` 或 `-shm`。请通过 Lumilio 创建一致的数据库快照，并单独备份媒体目录。具体方法见[备份与数据完整性](./integrity)。
:::

## 安装完成后

无论使用哪一种系统，下一步都是[完成首次设置](./first-use)：创建管理员账户、设置登录保护并创建主资源库。

完成基本设置后，如有需要再阅读：

- [HTTPS 与通行密钥](../help/https)：让其他设备安全访问，或配置更复杂的反向代理；
- [理解资源库与原始文件](./repositories)：确认媒体、数据库和备份分别存放在哪里；
- [Lumen AI](../features/lumen-ai)：按需启用本地或局域网 ML 能力；
- [Lumilio Agent](../features/agent)：按需启用对话式整理助手。
