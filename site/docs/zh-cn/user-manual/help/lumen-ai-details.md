---
title: Lumen AI 进阶
description: 说明 Lumen Hub、lumen-cli、mDNS、静态节点和 Host Broker 的网络边界与排障方法。
---

# Lumen AI 进阶

普通用户只需要先看 [Lumen AI](../features/lumen-ai) 的 Desktop 或 Server 路径。本页给部署者和排障人员，解释“节点为什么能被发现但仍然不能推理”，以及 bridge 网络、Docker Desktop 和 Host Broker 的边界。

## 先记住实际数据路径

Lumen 的发现和推理不是同一条链路：

~~~text
mDNS / 静态节点 / Host Broker
              │  只提供节点身份、地址和能力提示
              ▼
        Lumilio Server
              │  直接 gRPC 发送推理请求和必要的媒体派生数据
              ▼
        Lumen Hub（50051）
~~~

Host Broker 不在推理数据路径中。它不会替你转发媒体，也不会让一个容器自动获得访问另一个网络的能力。

## 三个名字不要混淆

| 名称 | 作用 | 是否运行模型 |
| --- | --- | --- |
| **Lumen Hub** | 自托管的多模型推理服务；提供语义、人脸、OCR、BioCLIP 等任务 | 是 |
| **lumen-cli** | Hub 的安装器和启动器；检测平台、下载匹配构建、生成配置并启动 Hub | 否 |
| **Lumen SDK / Host Broker** | SDK 负责发现、连接池和按任务路由；lumen-hostd 只负责把主机发现到的节点发布给容器 | 否 |

Desktop 把 lumen-cli 背后的安装和监督流程集成进控制面板，所以 Desktop 用户通常不需要单独安装 CLI。Server 用户可以直接使用 Docker 镜像，也可以在独立主机上安装 lumen-cli。

## 发现后端是叠加的，不是优先级链

Lumilio Server 的 [lumen] 清单会把下面的入口同时交给 Lumen SDK：

| TOML 字段 | 入口 | 什么时候使用 |
| --- | --- | --- |
| discovery_mdns_enabled = true | mDNS 浏览 _lumen._tcp | 节点和 Server 在同一可达局域网，容器能收到组播 |
| discovery_static_nodes = ["主机:50051"] | 静态节点 | 地址稳定，或者容器收不到 mDNS；也是 Docker Hub 最明确的接入方式 |
| discovery_hub_url = "http://主机:5866" | Host Broker WebSocket | 主机可以发现节点，但 Server 容器无法自己浏览 mDNS |

入口会并行运行，事件会合并。静态节点不是“mDNS 失败后才启用”的备用开关；推荐的混合模式就是保留 mDNS，同时写入至少一个你明确管理的静态 Hub 地址。

### Server 的推荐清单

下面的片段只展示 Lumen 段落；仍需放在版本完整、通过校验的 Server TOML 中：

~~~toml
[lumen]
discovery_enabled = true
discovery_mdns_enabled = true
discovery_hub_url = ""
discovery_static_nodes = ["192.168.1.30:50051"]
discovery_service_type = "_lumen._tcp"
discovery_domain = "local"
~~~

discovery_static_nodes 接受的是 Lumen Hub 的 host:port，不能填浏览器使用的 HTTPS 地址。discovery_hub_url 接受的是 Host Broker 的 HTTP(S) 地址，默认端口通常是 5866；不要把 Lumen Hub 的 50051 填入这个字段。

修改完整清单后重启 Server。Web **设置 → AI**只决定语义、视频语义、BioCLIP、OCR 和人脸任务是否排队，不能代替 TOML 中的节点发现配置。

## Docker 网络：先用静态节点

### Linux host network

Lumilio 的 Linux Compose 默认使用 host network。Lumen Hub 可以是同一台主机上的 Docker 容器，只要把 50051 发布到主机，Server 就可以使用主机地址或本机地址直连。mDNS 通常也能沿用主机的网络接口。

### bridge、NAS 隔离和 Docker Desktop

普通 bridge 网络、部分 NAS 的容器网络以及 Docker Desktop 在 macOS/Windows 上的虚拟机边界，可能导致：

- Lumilio Web 页面可以打开，但 Server 看不到 mDNS 节点；
- Host Broker 能列出节点，但 Broker 发布的地址从容器不可达；
- Hub 广播了 127.0.0.1 或只在宿主机可解析的名称，容器无法使用；
- 50051 端口只在容器内部开放，没有发布到 Server 可达的地址。

这类部署先做两件事：

1. 给 Lumen Hub 发布 50051，并确认从 Lumilio Server 所在的网络命名空间能直连；
2. 在 discovery_static_nodes 中写入一个容器实际能访问的地址。

只有静态地址无法满足“动态发现多个节点”的需求时，才进一步考虑 Host Broker。不要把“加了 Host Broker”当作网络路由或反向代理。

## Host Broker：仅用于特殊网络

Host Broker 的程序名是 lumen-hostd，由 Lumen SDK 提供。它在能接收局域网 mDNS 的主机上发现节点，再通过 HTTP/WebSocket（默认 :5866）向容器发布节点快照和变化。

### 什么时候值得使用

- Lumilio Server 位于无法接收 mDNS 的 Docker Desktop 或 bridge 网络；
- 主机上有多台会变化的 Lumen 节点，不希望手动维护静态地址；
- 你已经能确认 Broker 发布的每个节点地址，都能从 Lumilio 容器直接访问。

### 什么时候不要使用

- 只有一台地址固定的 Lumen Hub；
- Linux host network 已经可以收到 mDNS；
- 你只是想让容器访问另一个网段；
- 你还没有先验证 50051 的直连。

Host Broker 当前是发现控制面，不是认证网关，也不代理 gRPC 推理。请把它限制在可信局域网或专用网络中；不要直接把 5866 暴露到公网。更多命令和服务安装方式应以当前 Lumen SDK 发布物为准，不能把 lumen-hostd 与 lumen-cli 混为一谈。

## Lumen Hub 的启动状态

Lumen Hub 的 gRPC 端口可能在模型下载前就开始监听，但这不代表可以推理。启动阶段大致如下：

1. **启动**：配置已读取，控制接口可查询；
2. **下载**：首次获取模型权重和 BioCLIP 目录；
3. **加载/预热**：把模型放入选定的 CPU、Metal、Vulkan、CUDA 或 ROCm 后端；
4. **就绪**：读取能力并可接收推理；
5. **失败**：控制接口保留错误信息，便于查看具体模型或文件问题。

mDNS 只在 Hub 就绪后广播。看到节点端口已打开但任务仍不可用，先等待模型加载完成，再检查能力列表。首次启动下载失败不会自动变成可用的 CPU 推理。

## lumen-cli 的进阶用法

常规安装和三条命令见主指南。需要手工排查时：

- lumen-cli init 写入默认的 ~/.lumen/config.yaml 和 ~/.lumen/bootstrap.json；
- lumen-cli validate 只验证配置，不会启动 Hub；
- lumen-cli start 在后台准备匹配当前平台和后端的发布物，校验校验和后启动；
- lumen-cli run 在前台运行，适合直接观察日志；
- lumen-cli reload 重新读取配置并启动；
- lumen-cli stop 停止后台进程。

CLI 会根据系统和硬件选择发布 profile：macOS Apple Silicon 使用 Metal 或 CPU，Windows x64 使用便携 GPU 或 CPU，Linux x64 可选择 CPU、Vulkan、CUDA、ROCm，Linux arm64 还包括 CPU、便携 GPU 和部分 Jetson profile。不要把一个平台下载的 Hub 压缩包复制到另一个平台使用。

默认模型缓存是 ~/.lumen/models。把缓存放到独立磁盘时，只改缓存路径，不要把模型目录和 Lumilio 资源库混在一起。首次启动的模型文件、运行日志和 Lumilio 数据库拥有不同的备份周期。

## 逐层排障

按顺序检查，可以快速判断故障在哪一层：

| 观察到的现象 | 优先检查 |
| --- | --- |
| 能看到 Web，但能力页没有节点 | [lumen] 是否启用；mDNS 是否跨过容器边界；静态节点是否写在 Server 的完整 TOML 中 |
| 节点已发现，但连接失败 | 从 Server 容器直接测试 host:50051；检查 Hub 端口发布、防火墙和广播地址 |
| 节点已连接，但没有 OCR/人脸/BioCLIP | Hub 是否真的启用了对应服务；模型是否仍在下载或预热；Desktop 预设或 Docker 配置是否包含该服务 |
| Hub 显示运行中，但队列不动 | Web **设置 → AI**开关、任务能力、队列失败记录和资源库范围 |
| 首次启动很久 | 模型下载、镜像地区、磁盘空间和 GPU 后端初始化；不要立即重复启动多个 Hub |
| Broker 能列节点，推理仍失败 | Broker 发布的地址必须从 Lumilio 容器直连；Broker 不会修复路由或地址解析 |

节点“已发现”、“已连接”、“任务可用”和“索引覆盖率”是四个不同概念。只有任务可用并且队列完成，媒体才会出现对应的语义、人脸、OCR 或分类结果。

## 安全和数据边界

Lumen 节点会接收任务需要的媒体派生数据。远程节点的运行者可能看到这些数据和运行日志，请只使用你信任的设备。不要把 50051 或 5866 端口直接暴露在公网，也不要把 Host Broker 当成认证层。

模型权重和 BioCLIP 目录来自外部发布源；可以在受控网络中配置镜像或缓存，但不要把下载凭据写进 Lumilio Server 的 TOML。原始媒体、资源库内部目录、SQLite 数据库、Lumen 模型缓存和节点日志应分别备份。

