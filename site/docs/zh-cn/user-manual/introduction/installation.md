---
title: 安装流明集（Lumilio Photos）
description: 按 macOS、Windows 或 Linux 选择安装方式，校验下载文件后完成安装。
---

# 安装流明集（Lumilio Photos）

先确定流明集运行在哪台设备上，再选对应的安装方式。

| 设备 | 安装方式 |
| --- | --- |
| Mac（Apple Silicon） | 桌面版 .dmg |
| Windows 10/11 x64 | 桌面版安装程序 |
| Linux 服务器或 NAS | Docker Compose |

所有安装文件都发布在 [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases)。

::: warning 下载后先校验
每个 release 都公布 SHA-256 校验值。下载后先校验再安装：**校验值不匹配就停止，删除文件并从官方 release 重新下载。**

- macOS：`shasum -a 256 <文件.dmg>`
- Windows（PowerShell）：`Get-FileHash -Algorithm SHA256 <文件.exe>`
:::

## macOS

1. 打开 GitHub Releases，下载文件名含 `macos-arm64` 的 `.dmg`。
2. 校验文件（见上），把应用拖入「应用程序」文件夹。
3. 启动。应用未公证，首次可能被 Gatekeeper 拦截：打开 **系统设置 → 隐私与安全性**，对该文件点 **仍要打开**。

**成功标志**：菜单栏出现图标，浏览器打开 `http://localhost:6680` 并显示首次设置页。

**卸载**：退出应用，删除「应用程序」中的应用；如需连数据一起删除，再删除 `~/Library/Application Support/Lumilio Photos/`（媒体库位置单独保留，不会被删除）。

## Windows

1. 打开 GitHub Releases，下载文件名含 `windows-amd64-setup.exe` 的安装程序（或 `windows-amd64.zip` 便携版）。
2. 校验文件（见上），运行安装程序。安装到当前用户，不需要管理员权限。
3. 启动。安装程序未签名，SmartScreen 可能提示：确认文件来自官方 release 且校验匹配后，点 **更多信息 → 仍要运行**。

**成功标志**：系统托盘出现图标，浏览器打开 `http://localhost:6680` 并显示首次设置页。

**卸载**：设置 → 应用 → 卸载。便携版直接删除解压目录。

## Linux（Docker）

前置条件：Linux amd64/arm64 主机、Docker Compose 2.23.1+、当前用户能运行 Docker。

```bash
mkdir lumilio-server && cd lumilio-server
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.yml
docker compose up -d
```

数据默认存在 `./lumilio/media`（媒体）和 `./lumilio/app-state`（数据库、凭据、备份）。不想放在部署目录时，启动前设置：

```bash
export LUMILIO_STORAGE=/srv/lumilio/media
export LUMILIO_STATE=/srv/lumilio/app-state
```

**成功标志**：`docker compose ps` 中 `lumilio` 为健康状态，浏览器打开 `http://<主机IP>:6680` 显示首次设置页。

**故障排查**：`docker compose logs lumilio` 查看日志。

::: warning 局域网 HTTP 不加密
默认部署便于先完成安装，但流量可被同网络设备读取。需要长期远程访问时，再配置 [HTTPS 与通行密钥](../help/https)。
:::

## 安装完成后

下一步：[完成首次设置](./first-use)——创建管理员、设置登录保护、创建主资源库。
