# Lumilio Photos 跨仓库产品闭环执行计划

> ⚠️ **执行前提（强制，不讨论）**：本次执行所有改动不考虑兼容性（旧版本、协议 major、跨仓库版本偏移等）与生产环境（回滚、灰度、存量用户数据、发布风险等）。按最直接的路径完成闭环即可。

状态：计划已完成，尚未执行。

审查日期：2026-08-04。

## 1. 目标

让 `Lumilio-Photos`、`Lumen-Hub`、`Lumen-SDK` 和 `Lumilio-Assets` 形成一条可重复部署、升级和发布的产品链，同时保留 Lumen 的核心优势：Photos 负责存储与媒体管理，任意局域网设备可以独立运行 Hub，并通过 mDNS 自动上线和下线。

本计划只解决真实产品闭环和重复维护问题，不建设跨仓库治理平台。

## 2. 固定决策

1. 普通 PC 用户使用 Photos Desktop；Linux、DIY NAS 和成品 NAS 使用 Docker Compose；独立计算设备使用双语 `lumen-cli` 或 Site Docker 向导。
2. Docker 正式支持仅为 Linux `network_mode: host`。不发布 bridge、端口映射或 Host Broker 变体。
3. Host Broker 只保留为 SDK 可选高级工具，不进入主安装路径。
4. 不增加代码签名、设备配对或局域网安全基础设施，也不把平台告警作为文档或验收内容。
5. Desktop 内置 Hub 精确锁定 release 与 SHA-256；用户自行部署的 Hub 只要求 data-plane protocol major 兼容。
6. 四种公开能力名称固定为：

   | ID | 中文 | English |
   | --- | --- | --- |
   | `siglip` | 图像语义分析 | Image Semantic Analysis |
   | `face` | 人物识别 | Person Recognition |
   | `ocr` | OCR文字识别 | OCR Text Recognition |
   | `bioclip` | BioCLIP物种识别 | BioCLIP Species Recognition |

   协议和内部函数名不改；README、Site、Web、Desktop、CLI 和安装器只使用以上 Exact Term。
7. Desktop 内置 Hub 与 Desktop 版本一一绑定：pin 随 Desktop 发布更新，**不做运行时自动更新**（不检查、不下载、不重装）；存量用户已装 Hub 不单独升级。Hub 发版与 Desktop 发版相互独立，Desktop 只引用已发布的 Hub release，发布顺序单向为 Hub 先、Photos 后。

## 3. 执行基线

| 仓库 | 基线 |
| --- | --- |
| Lumilio Photos | `a4d8c0d2230894a8944cb979a1c4bac982b04710`；dirty，现有 Compose/Site 实验先单独提交 |
| Lumen Hub | `cd67719660056244c405835d07786110fc7c1223`；`v0.1.1` |
| Lumen SDK | `9514d11c954abdaba8750acbc5054602cefb3eed`；`v1.3.2` |
| Lumilio Assets | `fcef55b4081bbb490bcee2b00688ed16cd9997cc`；`assets-v1.1.0` |

禁止 reset/checkout 丢弃 Photos 现有改动。各仓库独立提交，不能把四仓库改动塞进一个不可独立回滚的提交。

## 4. 当前必须解决的问题

- Compose 文件已重命名，但 Taskfile、README、工具注释和 release body 仍有旧引用；三份 Lumen Compose 尚未进入测试。
- Desktop 仍锁定 Hub `v0.1.0`，而当前正式版本是 `v0.1.1`。
- preset、能力、模型和 profile 在 Hub、CLI、Site 与 Desktop 多处手工复制。
- `ml_service.proto` 和 `control.proto` 各有两份可独立漂移的副本；SDK 不过滤不兼容 Hub。
- CLI 没有双语、custom 和统一的再次配置入口；Site 仍公开手写完整 YAML 路径。
- Desktop Updates 页面没有接通 Wails v3 Updater。
- Assets 新版本需要人工同步修改三个 lock 字段。
- WASM、OpenAPI、Wails bindings 等生成物缺少可重复 drift check。
- Desktop 与 Docker release 相互独立，可能公开半完成版本；manual Docker dispatch 还会推 `edge`。
- 公开术语、Host Broker 定位、中英文安装路径仍不一致。

## 5. 执行阶段

### Phase 0：修复当前断链

1. 先提交 Photos 当前 dirty 基线。
2. 统一使用 `compose.yml`、`caddy.compose.yml`、`acme.compose.yml`，修复全部旧引用。
3. `compose:test` 对 Photos 三份 Compose、Lumen CPU/Vulkan/CUDA 和 E2E overlay 执行 `docker compose config`，并断言：
   - Lumen 全部使用 host network；
   - Vulkan 映射 `/dev/dri`；
   - CUDA 声明 `gpus: all`；
   - ~~正式 Lumen Compose 不包含 `ports`~~（该断言已随 9.2-1 移除，无端口约束由固定决策 2 文档保证）。
4. 从 Hub `v0.1.1` manifest 读取 Desktop 所需 artifact URL/SHA-256，更新当前 pin，不手抄 checksum。
5. 修正 Hub、SDK、Photos README/config 注释：Docker 正式路径是 host network + mDNS，Host Broker 不是主路径。

完成条件：当前 CI 不再因 Compose 重命名失败；六份正式 Compose 可解析；Desktop 能安装并启动 Hub `v0.1.1`。

### Phase 1：建立单一版本事实源

#### Hub

1. 由 `lumen-schema` 和 dist profile 生成 schema-versioned `manifest.json`，包含：Hub 版本、四能力 Exact Term、preset/custom 选项、模型/数据集、资源建议、platform/profile/backend、artifact URL/SHA-256 和 protocol provenance。
2. CLI、Launcher 和 Docker env parser 直接使用 `lumen-schema`；Site 与 Desktop 消费 release catalog，不再维护平行常量。
3. 为 preset 和代表性 custom 组合生成少量稳定 fixtures，验证不同入口得到相同配置。

#### Photos 消费 Hub

4. 新增 `lumen.lock.json`，只锁定一个 Hub 正式 release tag。
5. Renovate 使用 GitHub Releases 更新这个 tag，独立开 PR、关闭 automerge；`task lumen:sync/check` 从固定 tag 校验 catalog、`SHA256SUMS` 和消费者构建。

#### Assets

6. Assets 发布不携带元数据文件：`node scripts/verify.mjs`（含 LFS pointer 和 profile 子集检查）通过后打 `assets-vX.Y.Z` tag 并推 GitHub Release 即可。消费者（Photos）从 tag 自身重建三个派生字段：`git ls-remote` 解析 commit、下载该 tag 的 `assets.json` 计算 sha256。
7. Photos 提供 `task assets:reconcile [RELEASE=...]`：无参数时选择最高稳定 SemVer，一次性更新三个 lock 字段，默认拒绝降级。
8. Assets 更新走本地命令：`task assets:reconcile [RELEASE=...]` 后人工查看 diff 并提交，无 workflow、无分支、无 PR、无 schedule；Assets 不保存 Photos token，也不发送 `repository_dispatch`。（ADR-015 修订：变更由人主动发起，无需 PR 呈递机制。）

完成条件：Hub 升级只改一个 release pin；Assets 升级只产生一个三字段同步 PR；两者都由 Photos CI 验证并人工合并。

### Phase 2：锁定协议兼容

1. SDK 是 `ml_service.proto` 权威源，Hub vendor 原始文件；Hub 是 `control.proto` 权威源，Photos Desktop vendor 原始文件。
2. 消费端记录 source tag、commit 和 proto SHA-256，并做 byte-for-byte 检查。Go import path 差异通过 `protoc --go_opt=M...` 处理，不修改 proto。
3. SDK 与 Hub 增加 `buf lint` 和针对固定当前-major baseline tag 的 `buf breaking`，使用 `WIRE_JSON` policy。本计划不提前设计未来 major 迁移；当前 major 内 breaking change 一律失败。
4. SDK 只把已经通过**带内验证**且支持当前 data-plane major 的节点加入任务池；DNS-SD、static 和 Host Broker 的发现元数据均不得作出兼容性 verdict，也不得让 task hint 绕过验证：
   - Hub 的 DNS-SD TXT 只保留展示级 `v`、`runtime`；节点身份来自 DNS-SD instance name。`proto`、`tasks`、`uuid`、`status`、`version` 和 `cap_hash` 不进入 Hub 广播契约。
   - SDK 节点状态固定为 `pending → compatible | incompatible`。发现后允许建立连接，但 `pending` 节点不可被 picker 选中；只有完整读取 capability stream 后才形成 verdict。`protocol_version` 存在时必须属于当前 major；`Unimplemented`、无法解析或不同 major 均标记 `incompatible`。暂时性错误保持 `pending` 并重试。
   - 同一节点的 resolver/mDNS 刷新只更新地址以外的展示元数据，不得清除 capability verdict；真实 transport 重连或 endpoint 变化才回到 `pending` 并重新带内验证。验证使用 generation 隔离，旧连接迟到结果不得覆盖新连接 verdict。
   - `incompatible` 节点保持可见、不可调度；重连后的新 capability verdict 可以恢复。`NodeInfo.Compatible`/`IncompatibleReason` 保留用于展示，不作为 resolver 输入。
   - picker 区分“未知”和“已知不支持”：存在 `pending` 节点时返回 `balancer.ErrNoSubConnAvailable`，由 RPC context/deadline 限制等待；所有节点均已验证且确实不支持 task 时才立即返回 `no node supports task`。
5. 测试覆盖 SDK 自动发现/离线移除/再次上线、`pending` 请求等待、TXT/static/broker task hint 不可绕过验证、capability 不兼容后的同地址 rediscovery 不翻转 verdict、transport 重连后重新验证恢复、旧 generation 结果不可覆盖新 verdict，以及 Photos 对兼容和不兼容节点的调度。Hub 可以在 data plane 尚未 Ready 时发布 mDNS；SDK 必须保持 `pending`、重试 capability，并在 Ready 后无需新的发现事件即可恢复。

完成条件：proto 只有一个文本源；当前 major 的破坏性修改会失败；任何发现入口都不会在带内验证前调度任务；resolver 刷新不能推翻 verdict；不兼容节点重连后可以重新验证恢复。

### Phase 3：完成 CLI、Site 与 Desktop 配置闭环

1. CLI 使用 `clap` 解析命令、保留 `cliclack` TUI；公开入口统一为幂等的 `lumen-cli configure`，`init` 仅作为同实现兼容 alias。
2. 支持 `--lang zh-CN|en`；未指定时按 `LC_ALL` → `LC_MESSAGES` → `LANG` 判断，`zh*` 映射中文、`en*` 映射英文、其他回退英文，不把语言写进 Hub bootstrap。
3. `configure` 支持 `minimal/basic/brave/custom`。custom 至少选择一项能力，只在存在多个合法模型/数据集时继续询问；配置校验成功后原子写入。
4. 删除 Site 公开的 `LumenConfigBuilder`、手写完整 YAML 和旧“自定义配置部署”路径。原生节点统一使用 CLI。
5. Site Docker 向导从 pinned catalog 读取选项，生成 `.env` 并下载仓库维护的 CPU/Vulkan/CUDA Compose；用户不手写 Compose/YAML。
6. Desktop 保持 preset-first，不要求安装 CLI，但从同一 catalog 构建 preset/profile。
7. CLI 增加 `update` 命令（**用户主动触发**，非自动检查/后台拉取）：从 Hub release manifest 读取当前平台 artifact URL/SHA-256，下载校验后原子替换二进制，提示重启生效；不手抄 checksum，依赖 Phase 1.1 manifest 落地。

完成条件：中文/英文 CLI 可以首次配置和再次配置，`lumen-cli update` 可手动升级到新版本；CLI、Site、Desktop 对同一 preset/custom 产生相同语义；Site 只保留真实三条部署路径。

### Phase 4：接通 Wails Desktop Updater

1. 使用 Wails v3 `app.Updater`、GitHub provider、下载事件、SHA-256、解压和内置 helper；删除 Photos 自研 updater 的下载、manifest 和 staging 实现。
2. 使用精确资产名和 `AssetMatcher`：
   - `Lumilio-Photos-<tag>-macos-arm64.app.zip`；
   - `Lumilio-Photos-<tag>-windows-amd64-setup.exe`；
   - `Lumilio-Photos-<tag>-windows-amd64-portable.zip`。
   稳定版忽略 prerelease；prerelease 构建可以继续接收后续 prerelease 和最终稳定版。
3. `ChecksumAsset` 使用 `SHA256SUMS`；选中资产没有 SHA-256 或 `Verification == nil` 时直接失败。
4. macOS 由 Wails 替换顶层 `.app` 并重启；DMG 只负责首次安装。
5. Windows installer 由 Wails 下载校验后启动 Inno Setup，启动成功才退出当前 Desktop，不调用单 EXE swap。
6. Windows portable 只检查并下载完整新版，不声称自动替换整个应用目录。
7. 运行版本统一来自 tag；本地 `dev` 构建不连接生产更新源。

完成条件：现有 Updates 页面真实可用；macOS 和 Windows installer 完成各自正确更新流程；portable 不执行错误的局部替换。

### Phase 5：收口依赖、WASM 与生成物

1. 四仓库使用基础 Renovate 配置。只有真实耦合项分组：Wails tuple、gRPC+protobuf、Go/Node/pnpm toolchain、后续 WASM toolchain；`lumen-sdk` 保持独立人工审核 PR。
2. 关键组和所有 `0.x`/prerelease 关闭 automerge；普通成熟依赖的低风险 patch/minor 才允许 CI 通过后自动合并。
3. 明确 Go、Node、pnpm、Rust、Wails、Vite+ 的权威版本文件；`task setup` 补齐 Desktop panel，但不默认安装 WASM 工具链。
4. `wasm/` 建立 workspace 和单一 `Cargo.lock`。确认无调用方后删除 `thumbnail-wasm`、`export-wasm`；保留 crate 使用固定 `wasm-pack`，提供按需 `wasm:setup/build/check`。
5. 按子系统建立生成物检查：Server 的 sqlc/OpenAPI/config、Web 的 types/Redoc/WASM、Desktop 的 Wails/proto/resources、Site 的向导 fixtures。只在相关路径、schedule 和 release 中运行，不建立全仓库常驻 mega gate。
6. 增加 scoped Exact Term lint 和关键双语 route/link 检查；未完成的英文 placeholder 先从 sidebar 隐藏。

完成条件：依赖升级由 Renovate 发现、真实消费者 CI 裁决；提交生成物都有可重复命令；普通开发不需要安装无关工具链。

### Phase 6：完成发布与公开文档闭环

1. Photos、Hub、SDK 的手动 `workflow_dispatch(release_tag)` 执行正式 build/test/package/checksum，但不创建 tag/Release、不推 GHCR；Actions artifacts 仅用于打 tag 前复核。
2. 各仓库沿用现有 workflow，只改成 draft-first/publish-last：必要 jobs 全部成功后才发布 GitHub Release；失败不更新 `latest`、`edge` 或 flavor tags。
3. Docker dry run 分平台导出 OCI layout，不对默认 Docker store `load` multiarch；只对 runner 原生平台做容器 smoke。平台范围固定为 Photos/Hub CPU 的 amd64+arm64，以及 Hub Vulkan/CUDA 的 amd64。
4. 启用 GitHub Immutable Releases。`SHA256SUMS` 用于下载完整性；错误版本发布新的 patch，不移动 tag 或替换资产。
5. Photos 生成简洁的 `release-manifest.json`，记录 Photos tag/commit、Desktop checksum、Docker digest、Hub pin、SDK/proto 和 Assets revision。
6. 每个 Phase 同步其 README/Site 内容；本阶段只做终审：真实 Compose 名、host network、CUDA Container Toolkit、Vulkan `/dev/dri`、四能力 Exact Term、中英文关键安装路径和 Site 部署/回滚说明。
7. 按 SDK → Hub → Assets（仅 fixtures 变化时）→ Photos 顺序完成首次发布列车；无 contract 变化的单仓库 bugfix 可独立发布。

完成条件：打 tag 前可完整验证；公开 Release 不再出现半完成状态；从任一 README 进入 Site 都只看到与设备相符的真实部署路径。

## 6. 关键验收

| 场景 | 结果 |
| --- | --- |
| Desktop macOS | SQLite 首次设置、本机 Hub、LAN 自动发现和 `.app.zip` 更新均可用 |
| Desktop Windows installer/portable | installer 正确交接 Inno Setup；portable 只下载完整新版 |
| Photos Docker | host network、mDNS、SQLite/媒体持久化正常 |
| Lumen Docker | CPU 可启动；Vulkan 有 `/dev/dri`；CUDA 有 `gpus: all` 并明确 Container Toolkit 前置条件 |
| CLI | 中英文、preset/custom、首次/再次 configure、validate/start/reload/stop 可用 |
| 动态拓扑 | Hub 上线自动出现、离线移除、再次上线恢复；无 Hub 时 Photos 基础功能不受影响 |
| 协议 | 不兼容节点可见但不接收任务；兼容节点继续工作 |
| Catalog/Assets | Hub 只更新一个 pin；Assets 只更新一个三字段 PR，重复执行幂等 |
| Release dry run | 产生可检查 artifacts/OCI layout，但不创建 tag/Release、不推 GHCR |
| Site | 关键中英文链接有效，Docker 向导输出可解析，没有旧术语和 placeholder 入口 |

## 7. 稳定命令

```text
Lumilio-Photos
  task compose:test
  task lumen:sync
  task lumen:check
  task assets:reconcile [RELEASE=assets-vX.Y.Z]
  task assets:check
  task toolchain:check
  task wasm:setup
  task wasm:build
  task wasm:check
  task generated:check
  task release:check RELEASE_TAG=vX.Y.Z

Lumen-Hub
  task proto:check
  cargo xtask contract-check
  cargo xtask release-check --tag vX.Y.Z

Lumen-SDK
  task proto:check
  go test -race ./...

Lumilio-Assets
  node scripts/verify.mjs
```

## 8. 完成定义

- 四仓库工作树 clean，lock、proto provenance、release assets 和镜像 digest 可复核；
- Compose、catalog、Assets、protocol、generated、toolchain 和术语检查进入对应 CI；
- Hub pin 由 Renovate 单入口更新，Assets lock 由 reconcile 单入口更新，均不自动合并；
- Photos、Hub、SDK dry run 不产生公开发布副作用；正式 Release publish 后不可修改；
- Desktop、Photos Docker、Lumen CLI/Docker 和动态 LAN 拓扑至少完成一次真实验收；
- README、Site 和 release body 只描述实际支持的发行路径。

## 9. Phase 0–2 审查上下文（待逐项讨论）

审查日期：2026-08-04。

本节只记录当前本地实现的审查上下文和候选问题，不代表已经接受、拒绝或调整原计划。后续逐项讨论后，再决定修复方式、阶段归属和是否影响对应 Phase 的完成判定。

### 9.1 阶段边界修正

- 实际创建 tag、发布 SDK/Hub/Assets、更新 Photos 对新 SDK 的依赖，以及完成首次 SDK → Hub → Assets（按需）→ Photos 发布列车，属于 Phase 6。
- 因此，Photos 当前仍依赖 `lumen-sdk v1.3.2`，本身不作为 Phase 2 未完成的证据；不要求为了验收 Phase 2 现在提前发布 SDK 或更新 Photos 依赖。
- Phase 2 所写的“Photos 对兼容和不兼容节点的调度”可在 Phase 6 消费新 SDK 时完成真实跨仓库验收。是否还需要在 Phase 2 保留一个不依赖正式 tag 的 Photos 侧测试，留待讨论。
- Hub/SDK/Assets 本地分支领先 `origin/main` 也不单独视为 Phase 0–2 的实现缺陷；公开发布状态由 Phase 6 收口。

### 9.2 候选问题

1. **已解决：Compose 无端口断言已移除。** 原 `taskfile.yml` 三处 `grep -vqE '^    ports:'` 断言写反（语义为“存在非 ports 行”，恒通过、拦不住 ports）；尝试修正为 `! grep -qE` 后无法在 go-task 的 shell 处理下可移植执行。决定：移除三处 ports 断言，保留 host-network、`/dev/dri`、gpus 断言；无端口约束继续由固定决策 2 的文档与人工 review 保证，不再设自动 gate。
2. **完整 Hub catalog 尚未进入 Site/Desktop 消费链。** `server/tools/lumenlock` 当前只解析 artifact 和 preset ID，生成的 `desktop/internal/lumen/release_catalog.go` 也只包含 release artifact；Desktop Go、Desktop React 和 Site 仍分别维护 preset、model、dataset、资源建议或能力展示常量。需要讨论这是 Phase 1 的单一事实源缺口，还是按 Phase 3/5 的消费者和生成物工作收口。
3. **已解决：Assets 元数据回退路径 fail-open 已随机制删除关闭。** 原 `assetslock.resolveReleaseMetadata` 对 release.json 的下载失败/损坏/不一致统一回退为 legacy 重建；ADRC 决定 Lumilio-Assets 不发布 `release.json`，该路径现为唯一路径（从 tag 重建），候选问题不再存在。
4. **已解决：Assets reconcile workflow 已删除。** 原 `assets-reconcile.yml` 将 `workflow_dispatch` 输入直接插入 shell 命令、使用从 release 推导的固定分支名（并发/残留分支会冲突），且手动触发意味着变更是人发起的、无需 PR 呈递机制。决定：删除 workflow，改本地命令 `task assets:reconcile` + 人工提交；两个反模式随机制消失。
5. **已按 Q5 决策修正：兼容性只做带内验证。** SDK 不再读取 TXT `proto` 或从 resolver 接收兼容性 verdict；同地址 rediscovery 不改变已形成的 capability verdict，真实重连或 endpoint 变化才重新验证。回归测试覆盖 `discover → capability v2 → rediscover → 仍 incompatible → infer 失败`。
6. **已随 Q5 修正：`ExitIdle` 不再存在无 SubConn 的 TXT-incompatible 节点。** 所有发现节点都先建立 SubConn 并进入 `pending`，同时 `ExitIdle` 保留 `sc != nil` 防护；incompatible 只影响 picker，不移除用于检测重连的 SubConn。
7. **已按 Q5/Q4 决策关闭：mDNS 早于 data plane Ready 的窗口由 pending 状态吸收。** Hub 可以先被发现；SDK 在 capability 返回前不调度真实任务，暂时性 `UNAVAILABLE` 保持 pending 并重试，Hub Ready 后无需新的发现事件即可恢复。因此不要求把 mDNS 注册移动到 Ready 之后。
8. **已解决：vendored proto provenance 检查自报 hash——修正表述并锁死 re-vendor 入口。** 常规 `contract-check` 仅验证 vendored `ml_service.proto` 与同仓库 `provenance.json` 自洽（自报 hash），不访问 SDK 远端。决定不引入远端比对（单人项目无实际收益、引入网络依赖）；改为：① 输出文案明确“仅验证与 provenance 记录一致”；② `--sync-sdk <tag>` 声明为唯一 re-vendor 入口，vendor 文件只能经它更新。
9. **Exact Term 仍有已知展示漂移。** 例如 Desktop panel 仍出现 `Image semantic analysis`、`People recognition`、`OCR text recognition`、`BioCLIP species recognition`，中文也有 `OCR 文字识别`、`BioCLIP 物种识别` 的空格差异。Phase 5 已计划增加 scoped Exact Term lint；需要讨论是否随 catalog 消费提前修复，还是保留到 Phase 5 统一处理。

### 9.3 已执行验证

以下检查通过：

- Photos：`task compose:test`、`task lumen:check`、`task assets:check`；
- Photos Desktop：`go -C desktop test ./internal/lumen`；
- Hub：`cargo test --workspace`、`cargo xtask contract-check`、`cargo xtask config-fixtures --check`；
- SDK：`go test -race ./...`、`make proto:check`；
- Assets：`node scripts/verify.mjs`；
- 四仓库相对各自执行基线的 `git diff --check`；
- 验证结束后四个工作树均为 clean。

Photos Server 的局部 `internal/service` 和 `internal/api/handler` 测试在当前 macOS 环境被 `pkg-config` 返回的非法 `-Xpreprocessor` 参数阻断；本次未把该环境失败归因于 Phase 0–2 改动，但也没有据此宣称完整 Photos Server 测试已经通过。

### 9.4 Q5/Q4 最终决策与执行修订

最终决策：DNS-SD 只负责发现可连接的 host/port 和少量展示元数据；data-plane 版本与功能必须通过 `/home_native.v1.Inference/StreamCapabilities` 带内验证。发现层不得携带或推导兼容性 verdict，task hint 也不得在 capability verdict 前进入调度池。

状态转换固定为：

```text
discovered
  → pending（可连接、可见、不可调度）
      → capability current major → compatible（按 capability task 调度）
      → different/unparsable major 或 Unimplemented → incompatible（可见、不可调度）

同地址 discovery refresh：保持 verdict
transport reconnect / endpoint change：回到 pending，重新验证
node expired：删除节点状态
```

Q4 选择“未知等待、已知不支持失败”：只要存在尚未完成带内验证的节点，picker 返回 `balancer.ErrNoSubConnAvailable` 等待下一次 picker update；调用方 context/deadline 是等待上限。只有不存在 pending 节点，且所有已验证节点都不支持目标 task 时，才立即返回确定性错误。

本修订同时取代 Phase 2 原先隐含的 TXT 预筛方案；Photos 无需因此修改。实际发布 SDK/Hub、更新 Photos SDK 依赖和跨仓库真实验收仍按 9.1 的阶段边界留在 Phase 6。

本地修正完成后已验证：Hub `cargo test --workspace`、`cargo fmt --all -- --check`、`cargo xtask contract-check`；SDK `go test -race ./...`、`make proto:check`。本次只修改 Hub、SDK 和本计划，没有发布 tag/Release，也没有更新 Photos 的 SDK 依赖。
