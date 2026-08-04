---
title: Lumen AI 进阶
description: 理解 Lumen Hub 的发现、连接、就绪状态和 Docker 网络边界，并按数据路径排查问题。
---

# Lumen AI 进阶

普通部署先看 [Lumen AI](../features/lumen-ai)。本页只解释网络边界和排障方法，不是另一套安装流程。

## 发现和推理是两条链路

~~~text
Lumen Hub 就绪
      │
      ├── mDNS 发布节点地址与能力 ──→ Lumilio Photos
      │
      └── gRPC :50051 ←────────────── Lumilio 直接发送推理任务
~~~

mDNS 只负责让 Photos 知道“局域网中有哪些可用节点”。真正的媒体派生数据与推理结果由 Lumilio Server 直接通过 gRPC 与 Hub 交换，不经过发现服务转发。

因此排障时要分别确认：

1. Hub 是否完成模型下载和预热；
2. Photos 是否收到了 Hub 的 mDNS 广播；
3. Photos 是否能够直连广播中的地址与 `50051`；
4. Hub 是否提供当前任务需要的模型能力。

## 三个程序的职责

| 名称 | 作用 | 适合的分发路径 |
| --- | --- | --- |
| **Lumen Hub** | 提供图像语义分析、人物识别、OCR文字识别和 BioCLIP物种识别 | Docker 镜像或 CLI 下载的原生程序 |
| **lumen-cli** | 检测平台、生成配置、下载并监督 Lumen Hub | 能直接使用 Windows、macOS 或 Linux 的计算设备 |
| **Lumen SDK** | 由 Lumilio Photos 使用，负责发现节点、连接和按任务路由 | 已集成在 Lumilio Photos 中，不需要用户单独部署 |

Desktop 已经把 lumen-cli 的安装与监督流程放进控制面板。Linux 服务器、NAS 或独立 GPU 主机可以直接使用 Docker Compose。

## 官方 Docker 网络契约

Lumilio Server 和 Lumen Hub 的发行 Compose 都使用 Linux `network_mode: host`。容器直接复用宿主机的局域网接口，mDNS 和 gRPC 不需要经过 Docker 端口转换：

~~~text
局域网
├── NAS 主机网络
│   └── Lumilio Photos 容器（host network）
├── GPU 主机网络
│   └── Lumen Hub 容器（host network）
└── 临时打开的 PC
    └── lumen-cli / Desktop 管理的 Lumen Hub
~~~

这三种节点可以同时出现，也可以随时上线或下线。Photos 会根据已经发现且就绪的能力进行连接和任务路由；媒体存储不需要迁移到计算设备。

### 为什么发行文件不提供 bridge 版本

`ports: 50051:50051` 只能发布 TCP 端口，并不会让 bridge 容器自动获得局域网 mDNS。即使额外写入静态地址，也只能建立一条固定连接，无法完整保留节点动态上线、下线和零配置发现的产品行为。

因此当前发行物不会提供 bridge、额外端口映射或 Host Broker 变体。如果容器平台不能创建 host network 项目，应视为当前 Docker 分发路径不受支持，而不是继续修改发行 Compose 猜测兼容方式。

## 启动、健康与发现的先后顺序

Lumen Hub 会先启动控制面，再下载和加载模型：

1. **starting**：gRPC 进程已启动，但模型仍在下载、加载或预热；
2. **healthy**：标准 gRPC Health 已返回 `Serving`，所选服务可以接受任务；
3. **mDNS 广播**：Hub 就绪后才发布节点地址与实际能力；
4. **Photos 连接**：SDK 收到节点后建立连接，并把能力显示在状态页。

第一次启动时间主要取决于向导所选模型、下载地区、磁盘和计算后端初始化。`lumen-models` 是命名卷，重建或升级容器后会继续复用已经下载的模型。

容器内部已经包含健康检查命令。直接验证当前 Hub 是否真正可用：

~~~bash
docker exec lumen-hub lumen-hub healthcheck
~~~

命令成功退出表示 gRPC Health 已经是 `Serving`；端口能够建立 TCP 连接并不等价于模型就绪。

## 按现象排障

| 现象 | 首先检查 |
| --- | --- |
| 容器一直是 `starting` | `docker logs lumen-hub` 中的模型下载、磁盘空间和后端初始化状态 |
| Vulkan 日志显示 CPU 或 `llvmpipe` | 主机是否存在 `/dev/dri`，驱动是否支持 Vulkan 1.3，Compose 是否保留设备映射 |
| CUDA 容器启动失败 | 主机 `nvidia-smi`、NVIDIA Container Toolkit，以及 Compose 是否保留 `gpus: all` |
| Hub 已 `healthy`，Photos 仍没有节点 | 两边是否都使用 host network；主机防火墙和局域网是否允许 mDNS |
| Photos 看见节点但连接失败 | 从 Photos 所在主机确认广播地址的 TCP `50051` 可达，检查 Hub 主机防火墙 |
| 节点已连接但缺少某项任务 | Hub 的完整配置是否启用了对应服务，状态页是否显示该模型能力 |
| 节点和能力正常但队列不动 | Web **设置 → AI**中的任务开关、队列失败记录和资源库范围 |

如果需要确认基础网络，从 Lumilio Photos 所在主机测试 Hub 的局域网地址：

~~~bash
nc -vz 192.168.1.30 50051
~~~

这只能确认 TCP 可达；最终仍以 Hub 健康检查和 Lumilio 状态页显示的能力为准。

## lumen-cli 维护命令

CLI 路径适合能够直接操作操作系统的计算设备：

~~~bash
lumen-cli validate   # 验证完整配置
lumen-cli start      # 后台启动
lumen-cli run        # 前台启动并查看日志
lumen-cli reload     # 重新读取配置并重启
lumen-cli stop       # 停止后台 Hub
~~~

CLI 默认把配置与启动信息放在用户目录的 `.lumen/`，模型缓存放在 `.lumen/models`。不要把其他平台下载的 Hub 压缩包复制过来运行；应让 CLI 选择与系统和计算后端匹配的发布物。

## 手动静态节点只用于明确的高级拓扑

Lumen SDK 仍支持在 Lumilio Server 的完整 TOML 中加入 `discovery_static_nodes`。它适合已经明确接受固定节点地址、并能自行维护完整 Server 清单的部署者，也可以暂时用于区分“mDNS 没收到”和“gRPC 无法连接”。

静态节点不是官方 Docker 向导的必需步骤，也不会让 bridge 网络获得动态发现能力。普通用户应保留发行 Compose 的 host network，并让 Photos 自动发现节点。

## 数据边界

Lumen 节点会接收任务所需的媒体派生数据。只在自己信任的局域网设备上运行节点，不要把 `50051` 直接转发到公网。原始媒体、Lumilio SQLite 状态、Lumen 模型缓存和节点日志具有不同用途，应分别管理和备份。
