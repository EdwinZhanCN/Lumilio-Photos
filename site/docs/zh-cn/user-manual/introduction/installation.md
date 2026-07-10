# 安装

Lumilio Photos 是本地优先的:你的照片、视频与数据库都保存在自己的设备上。
根据它要运行的位置选择安装方式:

| 运行位置 | 安装方式 |
|---|---|
| Mac(Apple Silicon) | [macOS 应用](#macos-apple-silicon)——菜单栏应用,组件全部内置 |
| Windows 10/11(x64) | [Windows 安装器](#windows)——按用户安装,组件全部内置 |
| Linux 服务器或 NAS | [Docker Compose](#docker-linux-服务器-nas) |

桌面版内置了私有 PostgreSQL 与全部媒体工具,无需额外安装任何东西。
所有下载均在 [GitHub Releases 页面](https://github.com/EdwinZhanCN/Lumilio-Photos/releases)。

## macOS(Apple Silicon)

1. 从最新 release 下载 `.dmg`,打开后把 **Lumilio Photos** 拖入
   **Applications(应用程序)**。
2. 启动应用。应用暂未经过公证,首次打开时 macOS 会弹出 Gatekeeper 提示:
   打开**系统设置 → 隐私与安全性**,点击**仍要打开**(仅需一次)。
3. Lumilio Photos 常驻**菜单栏**(没有 Dock 图标)。首次运行会出现设置窗口:
   - **照片库位置**——原始文件的存放位置。可以选择外置硬盘;数据库与密钥
     始终保存在本机磁盘。窗口会实时校验该位置是否可写入并显示剩余空间。
   - **条款与开源许可**——阅读并同意。
4. 应用初始化私有数据库后,会自动在默认浏览器中打开
   `http://localhost:6680`。
5. 在浏览器中完成首次运行向导,创建**管理员账户**(先设置密码;
   随后可以添加验证器应用、通行密钥和恢复代码,均可跳过)。

::: tip 更新
有新版本时,菜单栏菜单会显示**有新版本**——点击打开 release 页面,
替换 Applications 中的应用即可。你的照片库与数据库不受影响。
:::

应用数据(数据库、密钥、日志)位于
`~/Library/Application Support/Lumilio Photos/`。卸载时从菜单栏退出、
删除应用;如需同时清除数据,再删除该文件夹(照片库位置是独立的,
永远不会被删除)。

## Windows

1. 从最新 release 下载 `Lumilio-Photos-<版本>-windows-amd64-setup.exe`
   并运行。SmartScreen 可能提示未知发布者——选择**更多信息 → 仍要运行**。
2. 安装器为**按用户安装**(无需管理员权限),会创建开始菜单快捷方式,
   并在缺失时自动安装 Microsoft Edge WebView2 运行时
   (首次运行的设置窗口需要它)。
3. 从开始菜单启动 **Lumilio Photos**。它运行在**系统托盘**中,
   首次运行的设置窗口与 macOS 相同:选择照片库位置、同意条款,
   然后浏览器会打开 `http://localhost:6680` 完成管理员账户向导。

卸载请前往**设置 → 应用**;卸载器会停止应用及其数据库,
并可选择是否一并移除应用数据。

## Docker(Linux 服务器 / NAS)

需要安装 Docker 及 Compose 插件。

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/docker-compose.release.yml
LUMILIO_STORAGE=/srv/photos docker compose -f docker-compose.release.yml up -d
```

将 `LUMILIO_STORAGE` 设置为存放媒体库的目录。然后打开
`http://<主机>:6657` 完成首次运行向导——它会创建管理员账户,
并自动轮换初始数据库凭据(生成的密钥持久化在你的存储目录下)。

- 用 `LUMILIO_VERSION=v1.0.0` 固定版本(默认 `latest`)。
- 端口:Web 界面 `6657`(HTTP)/ `6658`(HTTPS),API `6680`。

## 可选:AI 功能

语义搜索、人脸识别与 OCR 为可选功能,由
[Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) 推理服务提供。
启用之前不会下载任何内容。

- **桌面版(本机):**菜单栏/托盘 → **在本机启用 AI**。应用会自动下载
  匹配你硬件的 hub 构建并托管运行;首次启动还会下载模型权重(约 1.3 GB)。
- **另一台机器或 Docker:**在局域网内运行 Lumen Hub(Docker 标签
  `cpu` / `vulkan` / `cuda`),并让服务端指向它,例如在 Docker 的
  `server` 服务上设置 `LUMEN_DISCOVERY_STATIC_NODES=<hub主机>:50051`。
  详见 Lumen Hub 的 README。
