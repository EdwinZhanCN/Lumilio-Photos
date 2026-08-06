---
title: Lumen Intelligence
description: 在 Desktop、独立设备或 Docker 上启用本地优先的媒体理解能力，了解预设、地区、缓存与再配置行为。
---

<script setup lang="ts">
import DockerComposeConfigurator from "../../../.vitepress/components/DockerComposeConfigurator.vue";
</script>

# Lumen Intelligence

Lumen Intelligence 是一套运行在**你自己的设备或局域网内**的媒体理解能力：图像语义分析、人物识别、OCR文字识别和 BioCLIP物种识别。媒体和数据库留在存放它们的机器上，计算可以放在同一台设备，也可以放在局域网内另一台更合适的机器上。

这些能力都是可选的。未配置、未启用或节点不可达时，导入、浏览、相册、分享和备份照常工作。

## 这些能力解决什么问题

| 能力 | 你会看到的结果 | 适合的场景 |
| --- | --- | --- |
| 图像语义分析 | 用自然语言查找“海边的船”“生日聚会”等内容；也可以用一张媒体查找相似内容 | 不想先建立文件夹或标签时，用内容找媒体 |
| 视频语义 | 从视频帧生成语义索引，搜索视频中的场景 | 查找包含某个场景的视频 |
| 人物识别 | 检测人脸、生成特征并聚类，随后可以在人物页整理和命名 | 按人物回顾家庭活动或旅行记录 |
| OCR文字识别 | 识别媒体中的文字并建立全文检索 | 收据、菜单、文档、截图和路牌 |
| BioCLIP物种识别 | 根据图像给出物种候选，并在生物图鉴相册中整理 | 植物、动物和自然观察记录 |

这些任务由后台队列按需执行。打开任务开关不会立即处理已有的全部媒体；可以在[状态](./monitor)中查看节点、队列和覆盖率，再选择补齐或重建。

## 三条受支持的部署路径

选择哪条路径取决于两件事：**媒体放在哪台设备**，以及**计算资源在哪台设备**。

| 路径 | 适用情况 | 由谁管理配置 |
| --- | --- | --- |
| [Desktop 本机](#路径一-desktop-管理的本机-lumen-intelligence) | 主要在一台 Mac 或 Windows 电脑上使用 | Desktop 控制面板 |
| [lumen-cli 独立设备](#路径二-lumen-cli-管理的独立设备) | 有一台可以直接操作操作系统的 Windows、macOS 或 Linux 设备，想把它变成局域网推理节点 | `lumen-cli configure` |
| [Docker（Linux / NAS）](#路径三-linux-nas-的-docker) | 计算设备运行 Linux、NAS 或独立的 GPU 主机 | 环境变量（薄 intent）或发行 Compose |

三条路径使用同一套能力接口：Lumilio 只把任务发给**已发现且已就绪**的节点，节点离线后任务会等待或失败，不会偷偷改用到其他节点。

## 路径一：Desktop 管理的本机 Lumen Intelligence

这是 Desktop 的默认路径，适合“媒体和计算在同一台电脑”的情况。

### 首次启用

1. 打开 **Desktop 控制面板 → Lumen**，开启本机节点。
2. 在配置向导中选择**下载地区**、**能力预设**和**计算后端**，并确认**模型缓存位置**。选项含义见[统一配置模型](#预设-region-cache-与-profile)。
3. 保存配置。Desktop 会下载与当前系统匹配的 Lumen Intelligence 程序，首次启动还会下载所选模型。
4. 等待控制面板中的状态变为**运行中**或**就绪**。模型下载和预热完成前，节点可能已经启动但还不能处理任务。
5. 回到 Web 的 **设置 → AI**，打开需要的任务；在[状态](./monitor)中确认节点能力和队列状态。

在中国大陆，下载地区和模型会通过镜像源获取。

### 再次配置（安装后修改）

已安装状态下，可以在控制面板中修改**下载地区、能力预设和模型缓存位置**。再次配置的流程是：

1. 控制面板先渲染并验证一份候选配置，验证通过前不会停止当前节点；
2. 运行中再次配置会执行受控重启：停止 → 写入新意图 → 重新生成并启动；
3. 任一步失败都会恢复**旧配置和旧运行状态**，不会留下半套配置。

已安装的 release profile 在当前安装事务中保持固定；再次配置修改的是 preset、地区和后端，而不是把已安装的程序换掉。

模型缓存不是资源库的一部分。不建议把它放进媒体备份，也不要在节点运行时手动删除缓存文件。

## 路径二：lumen-cli 管理的独立设备

lumen-cli 可以把一台 Windows、macOS Apple Silicon 或 Linux 计算机变成局域网推理节点。它适用于你能直接操作这台设备操作系统的情况。

### 安装与首次配置

macOS Apple Silicon、Linux x64 或 Jetson Linux：

~~~bash
# 下载安装脚本
curl --proto '=https' --tlsv1.2 -LsSf https://lumilio.org/lumen/install.sh | sh

# 首次配置，会有引导选项，使用键盘方向键选择，回车确认
lumen-cli configure

# 启动节点，稍后会自动下载模型
lumen-cli start
~~~

Windows x64 PowerShell：

~~~powershell
# 下载安装脚本
powershell -ExecutionPolicy Bypass -c "irm https://lumilio.org/lumen/install.ps1 | iex"

# 首次配置
lumen-cli configure

# 启动节点
lumen-cli start
~~~

### configure 同时用于首次配置和再次配置

`lumen-cli configure` 是唯一实现：**首次配置和再次配置都使用它**。`init` 只是兼容别名，新环境请直接使用 `configure`。

- 再次配置时，已有配置中的**下载地区、预设、计算后端和模型缓存**会作为交互默认项，确认后才会覆盖；
- 选择完成后会显示摘要，并要求最终确认；节点运行中时，确认后会受控重新加载；
- 显式指定语言：`lumen-cli configure --lang en` 或 `lumen-cli configure --lang zh-CN`，其他值会明确失败；
- 不指定 `--lang` 时，按 `LC_ALL → LC_MESSAGES → LANG` 顺序自动选择；高优先级 locale 不支持时会继续回退，不会卡住。

配置写在用户目录的 `.lumen/` 下（`config.yaml` 与 `bootstrap.json`）。`lumen-cli start` 会下载匹配当前系统和计算后端的程序，校验后在后台启动；模型在第一次启动时下载。

### 安装与排障语境下的节点信息

以下内容只用于安装和排查：

- 默认监听 `0.0.0.0:50051` 并启用 mDNS；需要在设备防火墙中允许来自局域网的 TCP `50051` 和 mDNS 组播。
- 常用维护命令：

  ~~~bash
  lumen-cli validate   # 只检查配置
  lumen-cli reload     # 重新读取配置并重启节点
  lumen-cli run        # 在前台运行并直接查看日志
  lumen-cli stop       # 停止后台节点
  ~~~

- 不想运行安装脚本时，可以从 [Lumen-Hub Releases](https://github.com/EdwinZhanCN/Lumen-Hub/releases) 下载对应平台的压缩包和校验文件。不要把一个平台的压缩包复制到另一个平台使用。

::: tip 计算后端选择
macOS Apple Silicon 推荐 Metal；Jetson Linux 推荐 CUDA；其他 Linux ARM 平台推荐 CPU。
:::

## 路径三：Linux / NAS 的 Docker

Docker 是 Linux 服务器、NAS 和独立 GPU 计算主机的正式分发路径。

### 零配置默认：发行 Compose 文件

发布文件 `lumen-cpu.compose.yml`、`lumen-vulkan.compose.yml`、`lumen-cuda.compose.yml` 是完整的零配置部署：内置 **basic 预设**、**other 下载地区**、host network 和 `lumen-models` 持久化卷，不需要手写 YAML、`.env` 或静态节点地址。

~~~bash
docker compose -f lumen-cpu.compose.yml up -d --wait
~~~

开始前确认目标设备支持现代 Docker Compose 和 Linux `host` network。Vulkan 还要求主机提供 `/dev/dri`；CUDA 还要求安装 [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)。

### 其他地区、预设或自定义组合：env + 命令向导

需要修改下载地区、能力预设或自定义模型组合时，使用下面的向导。它只输出 **canonical 环境变量（薄 intent）和启动命令**：Lumen Intelligence 在容器启动时用这些变量渲染并校验完整配置，网页不会生成完整的运行时 YAML。

<DockerComposeConfigurator />

三份 Compose 文件都使用 Linux host network，这是保留局域网 mDNS 自动发现的支持边界。不要把配置改成 `bridge`，也不要添加 `ports: 50051` 后假设发现仍受支持；如果容器平台不能创建 host network 项目，当前官方 Docker 路径不支持在该平台上保留自动发现能力。

### 启动与就绪

第一次启动会下载并加载所选模型。这段时间容器保持 `starting` 是正常现象；只有标准 gRPC Health 返回 `Serving` 后，容器才会变为 `healthy` 并开始发布 mDNS。

~~~bash
docker compose -f lumen-cpu.compose.yml ps
docker compose -f lumen-cpu.compose.yml logs -f lumen-hub
~~~

Hub 变为 `healthy` 后，到 Lumilio Photos 的[状态](./monitor)页确认节点和模型能力已经出现。只要 Photos 与节点都按发行文件使用 host network，连接过程不需要额外配置。

- **Vulkan** 文件已映射 `/dev/dri:/dev/dri`；**CUDA** 文件使用 `gpus: all`；**CPU** 文件是唯一同时提供 `amd64` 与 `arm64` 镜像的版本。

## 预设、region、cache 与 profile

无论哪条路径，下面四个概念的含义一致：

- **region（下载地区）**：程序和模型从官方源还是中国大陆镜像下载。只影响下载地址，不影响界面语言和媒体位置。
- **preset（能力预设）**：一次决定启用哪些服务、使用哪种模型和需要预留的空间。不确定时从 **basic** 开始。

| 预设 | 包含能力 | 主要模型和数据集 | 资源参考 |
| --- | --- | --- | --- |
| **最小（minimal）** | 图像语义分析、人物识别 | SigLIP 2 base + InsightFace `antelopev2` | 至少 4 GB 内存、2 GB GPU/统一内存、约 2 GB 磁盘 |
| **基础（basic）** | 最小 + OCR文字识别 + BioCLIP物种识别 | SigLIP 2 base、PP-OCRv6 small、BioCLIP-2 + `TreeOfLife200MCore` | 至少 6 GB 内存、3 GB GPU/统一内存、约 6 GB 磁盘 |
| **完整（brave）** | 更强的图像语义分析模型和完整 BioCLIP 物种目录 | SigLIP 2 `so400m-patch14-384`、PP-OCRv6 small、BioCLIP-2 + `TreeOfLife200M` | 至少 8 GB 内存、4 GB GPU/统一内存、约 10 GB 磁盘 |

表中的数值是选型参考，不是所有设备的硬性上限。模型权重和物种目录会按需映射，首次下载和预热时可能出现短时内存峰值。

- **cache（模型缓存位置）**：模型权重下载到哪里。缓存不是资源库的一部分，也不建议放进媒体备份。
- **profile（安装 profile）**：Desktop 安装时确定、并固定到当前安装事务中的程序形态。再次配置可以修改 preset、地区和缓存，但不能在当前安装中更换已固定的 profile。

### 计算后端

计算后端决定模型在哪种硬件上执行，不会改变 Lumilio 的任务接口：

| 部署方式 | 可选后端 | 说明 |
| --- | --- | --- |
| Desktop | Metal、GPU、CPU | macOS Apple Silicon 优先 Metal；Windows x64 优先 GPU；CPU 兼容性最好 |
| lumen-cli | macOS 的 Metal/CPU；Windows 的 GPU/CPU；Linux 的 CPU、Vulkan、CUDA、ROCm 等发布构建 | `lumen-cli` 会根据系统和硬件选择匹配的构建 |
| Docker | `cpu`、`vulkan`、`cuda` 镜像标签 | 计算后端由镜像标签和容器设备映射决定 |

更强的后端通常速度更快，但也更依赖驱动、显存或统一内存。低功耗设备先使用最小预设和 CPU；如果节点已发现但任务不可用，先在[状态](./monitor)确认模型是否已经完成下载和预热。

## 数据边界

节点会接收任务需要的**媒体派生数据**（特征、帧、文字等），不会收到资源库数据库或账户数据。远程节点的运行者可能看到这些数据和运行日志，请只使用你信任的设备。不要把 `50051` 端口直接暴露到公网；原始媒体、资源库数据库、模型缓存和节点日志应分别备份。

节点发现、连接、就绪状态与 Docker 网络边界的详细排障方法见 [Lumen Intelligence 进阶](../help/lumen-intelligence-details)。
