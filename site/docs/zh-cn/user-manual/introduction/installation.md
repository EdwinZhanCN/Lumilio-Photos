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

Desktop 会引导你选择语言和下载区域，并显示本机默认存储位置。外部目录在启动完成后通过 Desktop 控制面板授权；不要把“挂载资源库”用于初始化空目录。Desktop 会在本机应用数据目录管理嵌入式 SQLite catalog；Lumen Hub 为可选组件，只在你启用本地 AI 时另行下载。

::: tip 本地网络权限
如需通过 mDNS 发现其他 Lumen 节点，请允许 macOS 的本地网络访问或 Windows 防火墙提示。拒绝该权限不影响基础媒体管理。
:::

### Desktop 网络访问

Desktop 控制面板提供三个明确的网络模式：

- **仅本机**是默认模式。服务只监听回环地址，并打开
  `http://localhost:6680`；没有其他设备访问需求时应保持此模式。
- **局域网 HTTP**会把监听地址扩展到局域网，但规范 Origin 仍是
  `http://localhost:6680`。它只适合可信家庭网络，并要求确认其他设备的
  流量没有加密；Passkey 仍只能在 Desktop 本机的 localhost 地址使用。
  操作系统防火墙可能还需要允许入站连接。
- **外部 HTTPS**用于已有的 HTTPS 反向代理。填写精确的公网 HTTPS Origin
  和每个可信代理的最小 CIDR；Desktop 自身绝不申请 ACME 证书。代理必须转发
  原始 scheme、host 与客户端地址。保存后 Desktop 会重启并检查就绪状态；候选
  配置启动失败时会自动恢复上一个可用配置。

主 Origin 同时也是 WebAuthn Origin，其 hostname 就是 RP ID。因此修改
hostname 后需要重新注册 Passkey；迁移期间请保留密码与 TOTP 恢复路径。

## Docker（Linux 服务器 / NAS）

需要在 Linux 上安装 Docker Engine 与 Compose 2.23.1 或更高版本。所有受支持的生产
Compose 都使用 host network，不再发布或转换端口；进程直接绑定完整 manifest
指定的宿主机监听地址。生产环境没有隐式明文模式。

先设置两个持久化宿主机目录：

```bash
export LUMILIO_STORAGE=/srv/lumilio/media
export LUMILIO_STATE=/srv/lumilio/state
export LUMILIO_IMAGE=ghcr.io/edwinzhancn/lumilio-server:latest
mkdir -p "$LUMILIO_STORAGE" "$LUMILIO_STATE"
```

### 同机 Caddy（推荐）

将域名 A/AAAA 记录指向主机，并开放公网 TCP 80/443：

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.caddy.yml
export LUMILIO_DOMAIN=photos.example.com
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config init --profile docker-external-proxy \
  --origin "https://${LUMILIO_DOMAIN}" \
  --trusted-proxy 127.0.0.1/32 \
  --output /data/app-state/server.toml
docker compose -f compose.caddy.yml up -d
```

Lumilio 只监听 `127.0.0.1:6680`。Caddy 直接绑定宿主机 TCP 80/443，通过
loopback 转发请求；证书状态保存在命名卷 `caddy_data` 和 `caddy_config` 中。

### 内置 ACME HTTPS

需要由 Lumilio 自己申请并终止证书时：

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.acme.yml
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config init --profile docker-acme \
  --origin https://photos.example.com --email admin@example.com \
  --output /data/app-state/server.toml
docker compose -f compose.acme.yml up -d
```

host-network 容器直接绑定 TCP 80/443。镜像只给非 root Server binary 授予
绑定低端口所需的 capability；如果端口已被宿主机进程占用，启动会失败。
CertMagic 的账户与证书保存在 `/data/app-state/tls`。证书申请失败会阻止启动，
绝不会降级到 HTTP。

### 已有反向代理

同一主机上已经直接运行 Caddy、Nginx 或 Traefik 时：

```bash
curl -LO https://raw.githubusercontent.com/EdwinZhanCN/Lumilio-Photos/main/deploy/compose/compose.proxy.yml
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config init --profile docker-external-proxy \
  --origin https://photos.example.com \
  --trusted-proxy 127.0.0.1/32 \
  --output /data/app-state/server.toml
docker compose -f compose.proxy.yml up -d
```

代理 upstream 指向 `127.0.0.1:6680`。若代理位于另一台主机，生成 manifest
时使用 `--listen <lumilio-host-ip>:6680`，并且只信任该代理的 `/32` 或
`/128`；宿主机防火墙也应只允许同一地址访问。代理必须覆盖而不是追加转发的
scheme、host 与客户端地址。`deploy/reverse-proxy/` 中保留了最小 Nginx 和
Traefik 示例。

媒体与应用状态必须使用两个独立持久化挂载。状态目录保存 schema v3 manifest、`library.sqlite3`、快照、凭据、日志和 ACME 状态，并应位于可靠本机磁盘。两个目录都必须允许容器 UID 10001 写入。修改配置后先运行：

```bash
docker run --rm -v "$LUMILIO_STATE:/data/app-state" "$LUMILIO_IMAGE" \
  server config validate --config /data/app-state/server.toml
```

Passkey Origin 始终精确等于 `server.primary_origin`，RP ID 始终是其 hostname。修改 hostname 后，已有 Passkey 仍会保留，但不能在新的 RP ID 登录；请保留密码 + TOTP 恢复路径并重新注册 Passkey。

::: warning 通过流明集创建备份
流明集运行时，不要直接复制 `library.sqlite3`、`-wal` 或 `-shm`，也不要用宿主机 SQLite 工具打开它们；跨容器挂载边界会破坏 WAL 锁协调。请在“设置 → 服务器”中创建并下载一致性快照，并单独备份媒体目录。
:::

Docker 中的 Lumen 网络模式边界请参阅 [Lumen AI](../features/lumen-ai.md)。不可变运行策略需要通过完整 schema v3 manifest 配置；普通环境变量不会覆盖这些字段。

## 从源码运行

源码运行面向开发者与贡献者，不是普通用户的推荐安装方式。开发环境需要 Go、Node.js/Vite+、Make、libvips、libraw、FFmpeg、ExifTool，以及重建浏览器 WASM 时使用的 Rust。SQLite 嵌入在 Go 进程中；只有容器交付和 E2E 工作流需要 Docker。

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
