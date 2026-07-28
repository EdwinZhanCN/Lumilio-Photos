<div align="center">

# Lumilio Photos

[English](README.md) | **简体中文**

<img width="128" height="148" alt="Lumilio Photos 标志" src="https://github.com/user-attachments/assets/9e51f2dd-af9c-47da-9232-cff9a6e6bf4f" />

为个人媒体库打造的本地优先照片与视频管理工具。

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=for-the-badge&logo=react)](https://react.dev/)
[![SQLite](https://img.shields.io/badge/SQLite-3-003B57?style=for-the-badge&logo=sqlite&logoColor=f5f5f5)](https://sqlite.org/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue?style=for-the-badge&logo=gnu)](LICENSE)

</div>

> [!WARNING]
> Lumilio Photos 是一款免费、开源且仍在积极开发中的 beta 软件。升级前请备份重要媒体库，并阅读发布说明以了解当前限制。

Lumilio Photos 将原始文件和应用数据保存在你所控制的基础设施上，为大型媒体库提供浏览、导入、整理、搜索和处理的一体化工作空间。AI 辅助功能完全可选；即使没有模型服务或外部 AI 提供商，基础媒体库功能仍可正常使用。

## 主要功能

- 本地优先的照片与视频媒体库，以及边界清晰的存储仓库
- 相册、人物、地点、堆叠、收藏与重复文件管理
- 上传、目录扫描、元数据提取、缩略图生成与转码
- 基于媒体库元数据的搜索与筛选
- 通过 Lumen 提供可选的语义搜索、人脸识别、OCR 和分类能力
- 响应式 Web 界面，以及 macOS 和 Windows 桌面应用
- 多用户身份验证，并可选启用 MFA 和 Passkey

## 安装

请根据媒体库的运行环境选择合适的发行方式：

| 运行环境 | 推荐方式 |
| --- | --- |
| macOS（Apple Silicon） | 从 [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) 下载 `.dmg` |
| Windows 10/11（x64） | 从 [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) 下载 `setup.exe` |
| Linux 服务器或 NAS | 使用下方基于已发布镜像的 Docker Compose 配置 |
| 贡献者开发环境 | 使用 `make setup` 和 `make dev` 从源码运行 |

桌面应用包含嵌入式 SQLite catalog 和所需媒体工具。应用运行在 Windows 系统托盘或 macOS 菜单栏中，并通过默认浏览器打开 `http://localhost:6680`。各平台的详细步骤和当前签名限制请参阅[安装指南](site/docs/zh-cn/user-manual/introduction/installation.md)。

### Docker Compose

运行前需要在 Linux 上安装 Docker Engine 及 Compose 2.23.1+。所有受支持的
生产栈都使用 host network，让 Lumen 发现和宿主机服务共享一个明确的网络
边界。单个应用镜像同时提供 Web 界面和 API。

推荐在同一台主机上运行 Caddy：

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.caddy.yml
export LUMILIO_STORAGE=/srv/lumilio/media
export LUMILIO_STATE=/srv/lumilio/state
export LUMILIO_IMAGE=ghcr.io/edwinzhancn/lumilio-server:latest
export LUMILIO_DOMAIN=photos.example.com
mkdir -p "$LUMILIO_STORAGE" "$LUMILIO_STATE"
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config init --profile docker-external-proxy \
  --origin "https://${LUMILIO_DOMAIN}" --trusted-proxy 127.0.0.1/32 \
  --output /data/app-state/server.toml
docker compose -f compose.caddy.yml up -d
```

启动后打开生成配置时指定的精确 HTTPS Origin。
`deploy/compose/compose.acme.yml` 由 Lumilio 直接占用公网 TCP 80/443；
`deploy/compose/compose.proxy.yml` 用于已有的同机代理，或经过显式配置的远程
HTTPS 代理。项目不再提供 Docker 明文开发栈：贡献者使用 `make dev`，隔离的
浏览器测试使用 `web/e2e/compose.yml`。

> [!IMPORTANT]
> 生产环境从 `/data/app-state/server.toml` 读取完整 schema v3 manifest；CLI flags 只生成或验证文件，不会覆盖运行时策略。流明集运行时，不要直接复制或打开 SQLite 文件；请在“设置 → 服务器”中创建一致性快照，并单独备份媒体目录。

## 本地开发

### 环境要求

- Go 1.25+
- [Vite+](https://viteplus.dev/) 及其支持的 Node.js 运行时
- Make
- Rust 和 `wasm-pack`（仅在重新构建浏览器 WASM 包时需要）
- 原生媒体库与工具：libvips、libraw、FFmpeg 和 ExifTool
- Docker 与 Compose v2（仅用于容器交付和 E2E 工作流）

克隆项目并启动开发环境：

```bash
git clone https://github.com/EdwinZhanCN/Lumilio-Photos.git
cd Lumilio-Photos
make setup
make dev
```

`make dev` 会启动位于 `6680` 端口的 API 和位于 `6657` 端口的 Web 应用；SQLite 嵌入在 Go 进程中，不需要数据库服务。`make setup` 会复制完整的 schema v3 manifest 到被 Git 忽略的 `server/config/server.local.toml`。默认开发 catalog 位于 `server/.local/lumilio/library.sqlite3`，媒体仍位于 `server/data/storage`。服务端不提供配置默认值或普通环境变量覆盖。

### 常用命令

```bash
make dev              # 启动服务端和 Web 开发进程
make server-dev       # 仅启动 API 服务
make web-dev          # 仅启动 Web 开发服务器
make test             # 运行服务端和前端质量检查
make server-test      # 运行 Go 服务端测试
make web-test         # 运行前端类型、代码规范和单元测试
make web-browser-test # 构建并运行生产环境浏览器冒烟测试
make desktop-test     # 运行桌面端模块测试
make dto              # 重新生成 OpenAPI 文档和前端 API 类型
make db-reset         # 删除开发数据库状态（破坏性操作）
make dev-reset        # 重建本地配置与 SQLite 状态，并保留媒体
```

版本化的 demo 与 E2E 媒体来自独立发布的
[`Lumilio-Assets`](https://github.com/EdwinZhanCN/Lumilio-Assets) 仓库。在
`web/` 中运行 `vp run assets:sync` 可同步 `assets.lock.json` 锁定的默认
profile；运行 `vp run assets:sync -- --profile=e2e` 可同步同一锁定 revision
下的其他 profile。文件会经过散列校验，并且只写入被忽略的
`.cache/lumilio-assets/` 目录。

## 可选的 Lumen AI

语义向量、人脸识别、OCR 和分类能力由独立的 [Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) 推理节点提供。这些功能需要用户主动启用，不会影响媒体的导入、浏览和基础整理功能。

- 桌面端可从系统托盘或菜单栏中选择 **在此设备上启用 AI**。Lumilio Photos 会下载、运行并管理与当前设备兼容的 Hub。
- Docker 或远程设备需要单独运行 Lumen Hub 并配置节点发现。网络模式、自动发现和兼容回退边界请参阅 [AI 与 Lumen](site/docs/zh-cn/user-manual/introduction/lumen.md)。

## 项目结构

| 路径 | 用途 |
| --- | --- |
| `server/` | Go API、处理队列、存储、数据库迁移和外部集成 |
| `web/` | React 19 与 TypeScript Web 应用 |
| `desktop/` | Wails v3 桌面宿主，内嵌 Server 与 SQLite catalog |
| `wasm/` | 浏览器端媒体处理流程使用的 Rust WebAssembly 包 |
| `site/` | 基于 VitePress 的用户与开发文档 |

## 文档

- [用户安装指南](site/docs/zh-cn/user-manual/introduction/installation.md)
- [用户手册](site/docs/zh-cn/user-manual/features/index.md)
- [桌面端开发与打包](desktop/README.md)
- [贡献者指南](AGENTS.md)

## 开源许可

Lumilio Photos 使用 [GNU General Public License v3.0](LICENSE) 许可证发布。
