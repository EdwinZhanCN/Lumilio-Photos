---
title: Lumen Intelligence
description: 在 Desktop、独立设备或 Docker 上启用本地媒体理解：语义搜索、人物、OCR 与物种识别。
---

<script setup lang="ts">
import DockerComposeConfigurator from "../../../.vitepress/components/DockerComposeConfigurator.vue";
</script>

# Lumen Intelligence

Lumen Intelligence 是运行在**你自己设备或局域网内**的媒体理解能力：图像语义分析、人物识别、OCR文字识别、BioCLIP物种识别。**全部可选**——不开时，导入、浏览、相册、分享和备份照常工作。

| 能力 | 你会得到什么 |
| --- | --- |
| 图像语义分析 | 用自然语言找「海边的船」「生日聚会」，或用一张照片找相似内容 |
| 视频语义 | 按场景搜索视频中的画面 |
| 人物识别 | 自动聚类人脸，到人物页命名整理 |
| OCR文字识别 | 识别收据、截图、文档中的文字并全文检索 |
| BioCLIP物种识别 | 识别动植物，整理到生物图鉴相册 |

任务由后台队列按需执行；打开开关不会立即处理已有全部媒体，可在[状态页](./monitor)查看覆盖率后补齐或重建。

## 选哪条路径

媒体在哪台设备、计算资源在哪，决定用哪条：

| 路径 | 适用情况 |
| --- | --- |
| [Desktop 本机](#路径一-desktop-本机) | 主要在一台 Mac/Windows 上使用 |
| [lumen-cli 独立设备](#路径二-lumen-cli-独立设备) | 有一台可直连操作系统的 Windows/macOS/Linux 设备 |
| [Docker（Linux/NAS）](#路径三-docker-linux-nas) | 计算设备是 Linux 服务器、NAS 或 GPU 主机 |

三条路径能力相同：流明集只把任务发给**已发现且就绪**的节点，节点离线不会悄悄换用别的节点。

## 路径一：Desktop 本机

1. 打开 **Desktop 控制面板 → Lumen**，开启本机节点；
2. 向导中选择**下载地区**、**能力预设**和**计算后端**，确认**模型缓存位置**（选项含义见[配置项](#配置项)）；
3. 保存，等待状态变为**运行中/就绪**（模型下载和预热完成前不能处理任务）；
4. 到 Web **设置 → AI** 打开需要的任务。

**再次配置**：控制面板中修改地区/预设/缓存。流程是**先验证候选配置 → 运行中受控重启 → 失败恢复旧配置和旧运行状态**。已安装的 release profile 在当前安装中固定，不能更换。

## 路径二：lumen-cli 独立设备

macOS Apple Silicon、Linux x64 或 Jetson Linux：

~~~bash
curl --proto '=https' --tlsv1.2 -LsSf https://lumilio.org/lumen/install.sh | sh
lumen-cli configure   # 首次配置和再次配置都用它
lumen-cli start
~~~

Windows x64（PowerShell）：

~~~powershell
powershell -ExecutionPolicy Bypass -c "irm https://lumilio.org/lumen/install.ps1 | iex"
lumen-cli configure
lumen-cli start
~~~

- **`configure` 同时用于首次配置和再次配置**；`init` 只是兼容别名；
- 再次配置时已有选择是默认项，确认摘要后才会覆盖；运行中会受控重新加载；
- 显式指定语言：`lumen-cli configure --lang en` 或 `--lang zh-CN`；不指定时按 `LC_ALL → LC_MESSAGES → LANG` 自动选择，不支持的 locale 会继续回退；
- 配置写在 `~/.lumen/`（`config.yaml`、`bootstrap.json`）；`start` 下载匹配系统和后端的构建、校验后后台启动，模型在首次启动时下载。

**安装/排障信息**：默认监听 `0.0.0.0:50051` 并启用 mDNS，需在防火墙放行 TCP 50051 与 mDNS。常用命令：`validate`（只查配置）、`reload`（重读配置并重启）、`run`（前台运行看日志）、`stop`。

## 路径三：Docker（Linux/NAS）

**零配置默认**：发行文件 `lumen-cpu/vulkan/cuda.compose.yml` 内置 **basic 预设 + other 地区 + host network**，直接：

~~~bash
docker compose -f lumen-cpu.compose.yml up -d --wait
~~~

前置条件：现代 Docker Compose、Linux host network；Vulkan 需 `/dev/dri`，CUDA 需 [NVIDIA Container Toolkit](https://docs.nvidia.com/datacenter/cloud-native/container-toolkit/latest/install-guide.html)。

**改地区/预设/自定义组合**：用下面的向导。它只输出 **canonical 环境变量（薄 intent）+ 启动命令**——网页不生成完整运行时 YAML，Lumen Intelligence 在容器启动时渲染并校验完整配置。

<DockerComposeConfigurator />

**启动与就绪**：首次启动下载模型，容器保持 `starting` 正常；gRPC Health 返回 `Serving` 后才变 `healthy` 并开始 mDNS 广播。`docker compose ps` / `logs -f lumen-hub` 查看状态和日志。**不要改成 bridge 或加 `ports: 50051`**——host network 是保留 mDNS 发现的支持边界。

## 配置项

| 项 | 含义 | 默认/建议 |
| --- | --- | --- |
| **region**（下载地区） | 程序/模型从官方源还是中国大陆镜像下载 | 只影响下载地址，不影响界面语言 |
| **preset**（能力预设） | 一次决定启用哪些服务和模型 | 不确定时选 **basic** |
| **cache**（模型缓存位置） | 模型权重下载到哪里 | 不属于资源库，不要放进媒体备份 |
| **profile**（安装 profile） | Desktop 安装时固定的程序形态 | 当前安装中不可更换 |

预设资源参考（选型参考，非硬性上限）：

| 预设 | 能力 | 资源参考 |
| --- | --- | --- |
| minimal | 语义 + 人物 | 4 GB 内存 / 2 GB GPU / 约 2 GB 磁盘 |
| basic | 全四项 | 6 GB 内存 / 3 GB GPU / 约 6 GB 磁盘 |
| brave | 更强语义模型 + 完整物种目录 | 8 GB 内存 / 4 GB GPU / 约 10 GB 磁盘 |

**计算后端**：Desktop 选 Metal/GPU/CPU（Apple Silicon 优先 Metal）；lumen-cli 按系统自动匹配；Docker 由镜像标签（cpu/vulkan/cuda）决定。低功耗设备先用 minimal + CPU。

## 数据边界

节点接收任务所需的**媒体派生数据**（特征、帧、文字），不会收到数据库或账户数据。远程节点的运行者能看到这些数据和日志，请只使用信任的设备；不要把 `50051` 暴露到公网。原始媒体、数据库、模型缓存和节点日志应分别备份。

排障细节见 [Lumen Intelligence 进阶](../help/lumen-intelligence-details)。
