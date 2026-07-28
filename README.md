<div align="center">

# 流明集（Lumilio Photos）

**简体中文** | [English](README.en.md)

<img width="128" height="148" alt="流明集标志" src="https://github.com/user-attachments/assets/9e51f2dd-af9c-47da-9232-cff9a6e6bf4f" />

为个人媒体库打造的本地优先照片与视频管理工具。

[![Go](https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go)](https://go.dev/)
[![React](https://img.shields.io/badge/React-19-61DAFB?style=for-the-badge&logo=react)](https://react.dev/)
[![SQLite](https://img.shields.io/badge/SQLite-3-003B57?style=for-the-badge&logo=sqlite&logoColor=f5f5f5)](https://sqlite.org/)
[![License: GPL v3](https://img.shields.io/badge/License-GPLv3-blue?style=for-the-badge&logo=gnu)](LICENSE)

</div>

> [!WARNING]
> 流明集是一款免费、开源且仍在积极开发中的 beta 软件。升级前请备份重要媒体库，并阅读发布说明以了解当前限制。

流明集将原始文件和应用数据保存在你所控制的基础设施上，为大型媒体库提供浏览、导入、整理、搜索和处理的一体化工作空间。AI 辅助功能完全可选；即使没有模型服务或外部 AI 提供商，基础媒体库功能仍可正常使用。

## 主要功能

- 本地优先的照片与视频媒体库，以及边界清晰的存储仓库
- 相册、人物、地点、堆叠、收藏与重复文件管理
- 上传、目录扫描、元数据提取、缩略图生成与转码
- 基于媒体库元数据的搜索与筛选
- 通过 Lumen 提供可选的语义搜索、人脸识别、OCR、BioCLIP 物种识别和分类能力
- 响应式 Web 界面，以及 macOS 和 Windows 桌面应用
- 多用户身份验证，并可选启用 MFA 和 Passkey

## 安装

请根据媒体库的运行环境选择合适的发行方式：

| 运行环境 | 推荐方式 |
| --- | --- |
| macOS（Apple Silicon） | 从 [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) 下载 `.dmg` |
| Windows 10/11（x64） | 从 [GitHub Releases](https://github.com/EdwinZhanCN/Lumilio-Photos/releases) 下载 `setup.exe` |
| Linux 服务器或 NAS | 使用下方基于已发布镜像的 Docker Compose 配置 |
| 贡献者开发环境 | 阅读[贡献指南](CONTRIBUTING.md) |

桌面应用包含嵌入式 SQLite catalog 和所需媒体工具。应用运行在 Windows 系统托盘或 macOS 菜单栏中，并通过默认浏览器打开 `http://localhost:6680`。各平台的详细步骤和当前签名限制请参阅[安装指南](site/docs/zh-cn/user-manual/introduction/installation.md)。

### Docker Compose

Docker 交付面向安装了 Docker Engine 与 Compose 2.23.1+ 的 Linux 主机。
三种生产变体都使用 host network：

- [`compose.caddy.yml`](deploy/compose/compose.caddy.yml)：推荐方案，由同机
  Caddy 提供公网 HTTPS。
- [`compose.acme.yml`](deploy/compose/compose.acme.yml)：由流明集直接申请
  并终止证书。
- [`compose.proxy.yml`](deploy/compose/compose.proxy.yml)：接入已有的同机代理，
  或经过显式配置的远程 HTTPS 代理。

部署前请阅读[安装指南](site/docs/zh-cn/user-manual/introduction/installation.md)。
生产环境需要显式提供带 schema 版本的完整 manifest，并为媒体和应用状态配置
彼此独立的持久化目录。

## 参与贡献

开发环境准备、生成代码流程、测试方式和提交约定统一放在
[CONTRIBUTING.md](CONTRIBUTING.md)。

## 可选的 Lumen AI

语义向量、人脸识别、OCR、BioCLIP 物种识别和分类能力由独立的
[Lumen Hub](https://github.com/EdwinZhanCN/Lumen-Hub) 推理节点提供。这些功能
需要用户主动启用，不会影响媒体的导入、浏览和基础整理功能。

- 桌面端可从系统托盘或菜单栏中选择 **在此设备上启用 AI**。流明集会下载、运行并管理与当前设备兼容的 Hub。
- Docker 或远程设备需要单独运行 Lumen Hub 并配置节点发现。网络模式、自动发现和兼容回退边界请参阅 [AI 与 Lumen](site/docs/zh-cn/user-manual/introduction/lumen.md)。

## 文档

- [用户安装指南](site/docs/zh-cn/user-manual/introduction/installation.md)
- [用户手册](site/docs/zh-cn/user-manual/features/index.md)
- [贡献指南](CONTRIBUTING.md)
- [桌面端开发与打包](desktop/README.md)
- [工程协作指南](AGENTS.md)

## 开源许可

流明集使用 [GNU General Public License v3.0](LICENSE) 许可证发布。
