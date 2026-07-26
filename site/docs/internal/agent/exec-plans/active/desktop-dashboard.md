# Lumilio Photos Desktop Dashboard UI/UX Implementation Plan

> - 路径：`site/docs/internal/agent/exec-plans/active/desktop-dashboard.md`
> - 唯一基线：`88a0836b64dafa5527b3bc0024a329800cfa62c7`
> - 基线提交：`feat(network): converge origin TLS and proxy deployment`
> - 工作分支：从该提交创建或继续当前实现分支
> - 状态：Active
> 视觉参考：本任务附带的 `lumilio-desktop-dashboard-uiux.html`；只参考信息层级和交互，不复制其独立 CSS 系统

## 1. Goal

在不扩大 Desktop 产品职责的前提下，将现有 Wails 私有控制面板整理为一个紧凑、可靠、可恢复的本机 Runtime Control Panel。

最终职责边界：

```text
Desktop Wizard
    仅负责首次宿主机设置
    本机默认存储、地区、条款、可选本机 Lumen Hub

Desktop Dashboard
    Lumilio Server 的启动状态、重启、失败恢复
    Desktop 所拥有的 Server Runtime 配置
    本机 Storage Location 授权和 Repository 重连
    Desktop 监督的本机 Lumen Hub 配置与生命周期
    日志、本机路径和有限诊断

Browser Web App
    用户、认证、Passkey 注册
    照片、相册、人物、地点、搜索、Agent
    媒体任务、图库产品设置、备份产品流程
```

最终 Dashboard 信息层级：

```text
Header
├── Refresh
├── Settings
└── Open Lumilio

Primary Runtime
├── Lumilio Server
└── Lumen Hub

Storage Locations
└── 保留现有全部原生目录和 Repository 操作

Support
├── App / Error / Lumen logs
├── Local paths
└── 可选的脱敏 Diagnostics export
```

统一 Settings Dialog：

```text
Desktop Settings
├── General
│   └── Download region
├── Lumilio Server
│   ├── Local
│   ├── LAN HTTP
│   └── External HTTPS
├── Lumen Hub
│   └── 复用现有 backend/profile、preset、cache 配置
└── Runtime Configuration
    ├── Current Runtime
    ├── Candidate TOML
    ├── Validate
    ├── Semantic changes
    ├── Apply and Restart
    └── Restore Last-known-good
```

完成后，用户无需终端即可：

- 看清 Server 和 Lumen Hub 当前状态；
- 打开正确的 `primary_origin`；
- 修改网络模式；
- 修改常用 Lumen Hub 配置；
- 验证并应用完整 Runtime TOML；
- 在错误配置导致 Server 启动失败时恢复 last-known-good；
- 查看日志和本机路径。

## 2. 基线代码事实

以下内容已经在 `88a0836` 中存在，实施时不得重复建设或错误删除。

### 2.1 网络与 Passkey 部署已经完成

当前 Server 已经使用 Runtime Manifest Schema v3：

```text
server.listen
server.primary_origin
server.tls
server.proxy
auth.passkey
```

`server/config.LoadAppConfig`：

- 严格要求完整 Manifest；
- 禁止未知字段；
- 规范化 Origin；
- 从 `primary_origin` 推导 Passkey Origin 和 RP ID；
- 校验 TLS/Proxy 组合；
- 计算 Manifest SHA-256。

Desktop 已经支持：

```text
local
lan_http
external_https
```

`desktop/supervisor/config.go` 中：

- `DesktopSettings.Version == 1`；
- `NetworkMode` 已存在；
- local 固定为 `127.0.0.1:6680` + `http://localhost:6680`；
- LAN HTTP 固定为 `0.0.0.0:6680`，primary origin 保持 localhost；
- external HTTPS 校验 HTTPS Origin、listen 和 trusted CIDRs；
- Desktop 明确不拥有 ACME 证书。

`Supervisor.ServerURL()` 已经返回当前 `primary_origin`，不再是纯硬编码 localhost。

### 2.2 网络设置已经有真实 Apply + Restart + Rollback

当前 `SettingsPanel.svelte` 已经包含：

- Download region；
- 三种 NetworkMode；
- LAN HTTP 风险确认；
- LAN 地址展示；
- external HTTPS origin；
- same-host / remote proxy；
- listen；
- trusted CIDRs；
- RP ID hostname 变化警告。

当前 `/__onb/network`：

```text
读取当前 DesktopSettings
→ 构造 candidate
→ Supervisor.ApplyNetworkSettings(...)
→ 返回 previous/effective origin 与 rpIDChanged
```

当前 `ApplyNetworkSettings` 已经：

```text
normalize candidate
→ atomic SaveSettings
→ Stop
→ Start
→ startup 失败则恢复旧 settings
→ 重新启动 last-known-good profile
```

并且已有真实 SQLite Desktop E2E：

```text
TestDesktopNetworkRestartRollback
```

因此本计划不是重新实现 deployment/origin/TLS，也不是重新发明网络设置后端。

本计划应当：

- 把已有网络表单迁入统一 Settings Dialog；
- 将网络 Apply 流程泛化为完整 Runtime candidate；
- 修复现有 lifecycle 安全缺口；
- 保留现有网络语义和测试。

### 2.3 当前 Dashboard 结构

当前 `Dashboard.svelte` 依次渲染：

```text
PhotosCard
StorageLocationsPanel
HubCard
LogPanel
SettingsPanel
PathsPanel
```

所有卡片纵向等权显示。

当前窗口：

```text
Width:     760
Height:    720
MinWidth:  640
MinHeight: 620
```

最终 UI 必须继续适配这组尺寸，不应变成大型桌面照片应用。

### 2.4 当前 Wizard 与 Dashboard 已分离

`App.svelte` 根据：

```text
mode = onboarding | dashboard
```

显示 `Wizard` 或 `Dashboard`。

Wizard 不是本次重构目标。不得把以下内容加入 Dashboard：

- Welcome；
- 创建管理员；
- Passkey 注册；
- 导入照片；
- 产品 onboarding。

### 2.5 现有成熟能力必须保留

#### `StorageLocationsPanel.svelte`

已支持：

- add Storage Location；
- remove external location；
- attach Repository；
- Storage Location moved conflict；
- Repository relocate/copy identity conflict；
- reveal in Finder/Explorer；
- active/offline/error 状态。

不得重写其业务流程。

#### `HubCard.svelte`

已支持：

- enable/disable；
- restart；
- check update；
- update/download；
- backend/profile/preset/cache/version/error 状态。

#### `ConfigureDialog.svelte`

已支持：

- backend/profile；
- preset；
- native cache path picker；
- cache writable validation；
- cache relocation 二次确认；
- save 后重新启动 Hub。

应抽取和复用，不得重写第二套 Lumen 配置逻辑。

#### `LogPanel.svelte`

已支持：

- App/Error/Lumen tabs；
- 固定 allowlist；
- bounded tail；
- Lumen control-plane tail 优先、文件 fallback。

不得将其改成任意文件读取器。

#### `PathsPanel.svelte`

已支持：

- storage；
- logs；
- backups；
- app data；
- previous Lumen cache；
- reveal。

### 2.6 当前 lifecycle 的真实缺口

当前 `desktopApp` 仍只有：

```go
ready  bool
status string
lastStage
```

当前 Server 启动失败：

```text
显示 native error dialog
→ app.Quit()
```

这与最终 Recovery Dashboard 冲突。

当前 `Supervisor.Stop()`：

- timeout 时只记录日志；
- 不返回 timeout error；
- 随后清空 cancel；
- 释放 single-instance lock。

因此在 timeout 后，调用方可能继续 `Start()`，旧 Runtime 仍可能持有 listener、River 或 SQLite。

当前 `ApplyNetworkSettings`：

- 在停止旧 Runtime 前已经替换持久化 settings；
- rollback 只针对正常函数返回路径；
- 没有跨 Desktop 进程崩溃的 apply journal。

这些是本计划需要修正的核心，不是 UI 细节。

### 2.7 当前配置所有权

当前：

```text
desktop-settings.json
    持久化 Desktop 用户选择
    包含 network、Lumen、region、onboarding 等

config/server.toml
    每次启动由 server.template.toml 重新生成
    是该次 Runtime 的严格不可变输入
```

所以 Raw TOML 编辑器不能直接把 `config/server.toml` 当作持久化配置文件。

## 3. 明确不做

本计划不实现：

- 新的 first-run onboarding；
- Browser Web App 产品功能；
- Docker 控制面；
- Desktop ACME；
- DNS challenge；
- 远程 Lumen 主机系统管理；
- River queue 管理台；
- 照片数量、相册、人物、搜索等 Dashboard 信息；
- 新组件库；
- 完整 UI 主题重写；
- 假造的 Lumen capability chips；
- 没有 backend 证据的 uptime、database health、free-space 指标；
- 全量浏览器 E2E 框架；
- screenshot golden infrastructure；
- exhaustive kill-at-every-line fault matrix。

真实 Diagnostics ZIP、update badge 和更多健康指标列为 P1，不阻塞核心 Dashboard。

## 4. 最终 UI

### 4.1 Header

```text
Local Runtime
Desktop Control Panel

[Refresh] [Settings] [Open Lumilio]
```

规则：

- `Open Lumilio` 使用 Runtime snapshot 中的 canonical browser URL；
- Server 未运行时禁用；
- Refresh 刷新 private panel state；
- Settings 打开统一 Dialog；
- 保留现有 TitleBar 语言切换，不在 General 中重复放第二个语言入口。

### 4.2 Primary Runtime Grid

正常 760px 宽度：

```text
ServerCard | HubCard
```

最小宽度或内容不足时：

```text
ServerCard
HubCard
```

不得产生横向滚动。

### 4.3 ServerCard

正常状态显示 backend 可证明的数据：

```text
Lumilio Server
Running / Starting / Restarting / Failed
version
primary origin
network mode
TLS mode
proxy mode
Passkey availability summary
current startup stage
```

Actions：

```text
Open
Restart
More
```

More：

```text
Open Server Settings
Copy Origin
Open Error Log
```

不增加长期 Stop 按钮。Quit 仍是停止 Desktop runtime 的正常入口。

#### Failed State

```text
Lumilio Server could not start

Stage
Loading Runtime Manifest

Reason
<sanitized actionable error>

[Retry]
[Edit Configuration]
[Restore Last-known-good]
[Open Error Log]
```

Server 失败时 Wails panel 和 tray 必须继续存在。

### 4.4 HubCard

保留当前已证明的数据：

```text
state
backend/profile
preset
cache path
installed/latest version
download/update progress
error
```

保留当前 actions：

```text
Enable / Disable
Restart
Check Update
Update
Configure
```

`Configure` 改为打开统一 Settings Dialog 的 Lumen Hub tab。

不得显示当前 Lumen status contract 没有提供的 capability list。

### 4.5 Storage Locations

保持全宽。

只调整视觉层级：

- 清晰 section header；
- Add Location 为 primary action；
- Attach Repository 为 secondary action；
- row 状态、路径、reveal/remove 更紧凑；
- default location 不显示 remove。

所有现有 conflict UI 必须保留。

### 4.6 SupportPanel

将 LogPanel 和 PathsPanel 收入一个低频 disclosure：

```text
Diagnostics and Local Files
App / Error / Lumen logs
Local paths
```

默认 collapsed。

以下状态可自动突出或默认展开：

```text
Runtime failed
Lumen failed
存在当前错误提示
```

所有现有 log/path actions 保留。

## 5. 统一 Settings Dialog

使用 Bits UI Dialog，适配当前窗口。

Tabs：

```text
General
Lumilio Server
Lumen Hub
Runtime Configuration
```

必须支持：

- focus trap；
- Escape；
- close 后焦点返回；
- dirty draft 关闭确认；
- apply 期间明确 busy 状态；
- tab-specific footer action。

### 5.1 General

P0 只移动现有：

```text
Download region
```

Footer：

```text
Save Settings
```

只修改 region 时不重启 Server，也不重启 Lumen。

### 5.2 Lumilio Server

把当前 `SettingsPanel.svelte` 中的网络功能完整迁移进该 tab。

使用三张 mode cards：

```text
Local only
LAN HTTP
External HTTPS
```

显示当前 candidate 解析结果：

```text
Listen
Primary Origin
Passkey RP ID
TLS owner
Proxy requirement
Remote Passkey behavior
```

保留现有规则：

#### Local

```text
listen = 127.0.0.1:6680
primary_origin = http://localhost:6680
```

#### LAN HTTP

```text
listen = 0.0.0.0:6680
primary_origin 保持 http://localhost:6680
```

必须继续要求风险确认。

#### External HTTPS

支持：

```text
same-host proxy
remote proxy
primary origin
listen
trusted CIDRs
```

继续提示 hostname 改变会改变 RP ID。

Footer：

```text
Apply and Restart
```

### 5.3 Lumen Hub

复用 `ConfigureDialog.svelte` 现有内容：

```text
backend/profile
preset
cache path
cache validation
cache move confirmation
```

Footer：

```text
Save and Restart Hub
```

不得因此重启 Lumilio Server。

### 5.4 Runtime Configuration

明确显示：

```text
Current Runtime
Candidate Configuration
```

P0：

```text
Current canonical TOML, read-only
Candidate TOML editor
Validate
Reset Candidate
Semantic changes
Apply and Restart
Restore Last-known-good
Reveal active server.toml
```

不自动保存输入。

Footer：

```text
Validate, Apply, and Restart
```

语义变化至少覆盖：

```text
Network mode
Listen
Primary Origin
Passkey RP ID
Passkey enabled
Logging level
Repository scan
Transcode hardware acceleration
```

不要求 P0 实现完整 line-by-line diff。

## 6. Runtime 配置文件模型

为避免 `desktop-settings.json`、结构化表单和 Raw TOML 三者 drift，采用以下明确文件角色：

```text
<app-data>/config/runtime.toml
    持久化、完整、schema-v3 的 Desktop Runtime intent
    结构化 Server tab 与 Raw TOML 编辑器操作同一份内容

<app-data>/config/server.toml
    每次 Runtime 启动前 materialize 的实际完整 Manifest
    是该 generation 的不可变输入
    仍以 0600 写入

<app-data>/config/runtime.last-known-good.toml
    最近一次通过 readiness 的 runtime intent

<app-data>/config/runtime.candidate.toml
    apply 期间的 staged candidate；正常状态不存在

<app-data>/config/runtime-apply.json
    小型 apply journal；正常状态不存在

desktop-settings.json
    只保存 Desktop host/control-plane 设置
    onboarding、region、language、Lumen Hub lifecycle/config、
    LAN warning acceptance 等
```

### 6.1 DesktopSettings v2

将 disk schema 从 version 1 升到 version 2。

v2 中：

- network Server fields 不再是 Runtime 的 source of truth；
- network mode 从 `runtime.toml` 的 resolved config 推导；
- 保留 `LANHTTPWarningAcceptedVersion`；
- 保留 onboarding/region/language/Lumen fields；
- 不在 JSON 中嵌套完整 TOML string。

迁移 v1 → v2：

1. 读取旧 network fields；
2. 使用当前 `server.template.toml` 和 host bindings 生成等价 schema-v3 Manifest；
3. 写入 `runtime.toml`；
4. 严格加载确认；
5. 写入 `runtime.last-known-good.toml`；
6. 原子写 DesktopSettings v2；
7. 不改变 local/LAN/external 行为。

使用独立的 v1/v2 disk structs 完成迁移，不要靠删除字段后让 JSON decoder 静默忽略旧值。

### 6.2 HostProjection

Desktop 每次启动都覆盖 host-owned fields：

```text
schema_version = 3
environment = production

database.path
server.web_root
server.cors_allowed_origins = []
logging.dir
storage.path
storage.cloud_state_path
storage.backups_path
auth.secret_key_file
tools.exiftool_path
tools.ffmpeg_path
tools.ffprobe_path
lumen.discovery_static_nodes
lumen.deployment_id
```

Desktop 还必须拒绝：

```text
server.tls.mode = acme
```

Desktop 只允许：

```text
off
external
```

`server.tls.http_listen/email/storage_path` 在 Desktop 中由模式固定为空。

Raw editor 可以显示 host-owned fields，但修改时必须返回明确 validation issue，不得静默接受或悄悄丢弃。

允许用户编辑其他 Runtime policy，例如：

```text
server.listen / primary_origin / proxy
logging.level / formats / repository audit
repository_scan
geocoding
auth TTL / passkey enabled/name / rate limits
transcode.hardware_accel
Lumen 非 host-owned的 discovery/timeouts/chunk policy
```

`lumen.discovery_static_nodes` 在 P0 由 Desktop 固定为本机监督 endpoint；远程静态节点编辑不是本次目标，mDNS discovery 保持可用。

### 6.3 单一 Candidate

Settings Dialog 内只有一份 candidate TOML。

```text
Server structured form
        ↕ backend patch/parse
Candidate TOML
        ↕ strict validation
Resolved network summary
```

前端不自行解析 TOML。

结构化 Server tab 修改时调用 backend patch endpoint，backend 返回：

- 更新后的 candidate TOML；
- resolved network；
- semantic changes；
- validation issues。

Raw editor修改后 Validate，也由 backend返回同样的 resolved summary。

## 7. Server config loader 复用

不要在 Desktop 重写 Schema v3 校验。

在 `server/config` 增加窄接口：

```go
func LoadAppConfigBytes(manifestPath string, data []byte) (AppConfig, error)
```

`LoadAppConfig(path)` 改为：

```text
ReadFile
→ LoadAppConfigBytes(path, data)
```

它必须继续：

- DisallowUnknownFields；
- 完整字段 presence；
- relative path 解析；
- Origin/TLS/Proxy/Passkey 规则；
- Manifest SHA-256；
- loaded invariant。

Desktop candidate validation：

```text
parse candidate generic TOML
→ check/patch host ownership
→ materialize candidate bytes
→ serverconfig.LoadAppConfigBytes(...)
```

不在 TypeScript 或 Desktop Go 中复制 Server 规则。

## 8. Runtime lifecycle 重构

### 8.1 Typed Runtime State

替换长期使用的：

```text
ready bool
status string
```

定义：

```go
type RuntimePhase string

const (
    RuntimeStopped    RuntimePhase = "stopped"
    RuntimeStarting   RuntimePhase = "starting"
    RuntimeRunning    RuntimePhase = "running"
    RuntimeRestarting RuntimePhase = "restarting"
    RuntimeFailed     RuntimePhase = "failed"
)
```

Snapshot：

```go
type RuntimeSnapshot struct {
    Phase                  RuntimePhase
    Stage                  string
    ErrorCode              string
    ErrorMessage           string
    BrowserURL             string
    CanOpen                bool
    CanRestart             bool
    LastKnownGoodAvailable bool
    Network                NetworkSummary
}
```

`NetworkSummary` 由严格加载后的 `AppConfig` 生成，不由 Svelte 从 URL 猜测。

Panel、tray、ServerCard 都读取同一个 snapshot。

### 8.2 Host lock 与 Runtime generation 分离

当前 single-instance lock 由每次 `Start/Stop` 获取和释放，不适合失败后保持控制面板。

改为：

```text
Supervisor host lifetime
├── Prepare/AcquireHostLock
├── StartRuntime generation
├── StopRuntime generation
├── RestartRuntime
└── Close/ReleaseHostLock
```

规则：

- Desktop app 存活期间始终持有 instance lock；
- Server startup 失败时仍持有；
- Runtime restart 不释放 host lock；
- 仅 Desktop Quit 时释放。

### 8.3 Runtime generation ownership

引入明确 generation：

```go
type runtimeGeneration struct {
    cancel context.CancelFunc
    done   chan struct{}
    err    error
}
```

Supervisor 保证：

- 同时最多一个 generation；
- Start 在旧 generation 未结束时拒绝；
- Stop cancel 后等待 `done`；
- timeout 返回明确错误；
- timeout 时不把 generation 标记为已停止；
- timeout 时不允许启动新 generation；
- RepositoryControl 在 stop/restart 窗口清空，避免使用旧 manager；
- 新 generation ready 后重新设置。

### 8.4 Serialized operations

以下操作共用一个 operation mutex：

```text
initial runtime start
manual restart
candidate apply
last-known-good restore
application shutdown
```

并发请求返回 `409 operation_in_progress`。

### 8.5 Startup failure

改为：

```text
RuntimeFailed snapshot
→ 保留 Wails window 和 tray
→ 不 app.Quit()
→ Dashboard 提供恢复动作
```

`ErrAlreadyRunning` 可继续作为 host-level fatal case处理，但普通 config、port、manifest、database startup error必须进入 Recovery UI。

## 9. Apply、Rollback 与 Crash Reconciliation

### 9.1 Candidate apply

顺序：

```text
1. 验证 base fingerprint
2. 严格 validate candidate
3. 拒绝 host-owned changes
4. 原子写 runtime.candidate.toml
5. 写 journal phase=candidate_staged
6. StopRuntime，并确认 generation 完全退出
7. Stop失败：
   - 删除 staged candidate/journal
   - 保持 runtime.toml 不变
   - 不启动第二个 generation
8. 将 candidate 原子替换为 runtime.toml
9. journal phase=candidate_promoted
10. StartRuntime
11. readiness成功：
    - runtime.toml → runtime.last-known-good.toml
    - 删除 candidate/journal
    - RuntimeRunning
12. candidate启动失败：
    - journal phase=rolling_back
    - runtime.last-known-good.toml → runtime.toml
    - StartRuntime
13. rollback成功：
    - 删除 candidate/journal
    - RuntimeRunning
    - 返回 candidate rejected + rolled back
14. rollback失败：
    - RuntimeFailed
    - 保留 journal和两个错误
```

### 9.2 Manual restart

只使用当前 `runtime.toml`：

```text
StopRuntime
→ StartRuntime
→ readiness
```

不修改 candidate 或 last-known-good。

### 9.3 Restore Last-known-good

```text
stage last-known-good as candidate
→ 使用同一 Apply engine
```

不得建立第二套恢复实现。

### 9.4 Desktop crash reconciliation

启动时检测 journal。

最小规则：

```text
candidate_staged
    当前 runtime.toml 未改变
    删除 staged candidate + journal
    正常启动 current

candidate_promoted / rolling_back
    恢复 runtime.last-known-good.toml
    删除 staged candidate
    正常启动 last-known-good
```

不要求本次做穷举 kill matrix，但必须为以上状态写直接单元测试。

## 10. Private Control-plane API

继续使用现有 `/__onb` 私有边界。

不改为 public OpenAPI，不改成 Wails JS bindings。

新 API 使用 typed Go structs，不再继续增加 `map[string]any`。

### 10.1 State

```http
GET /__onb/state
```

保留 Wizard 所需字段，Dashboard 新增：

```json
{
  "runtime": {
    "phase": "running",
    "stage": "ready",
    "browserURL": "http://localhost:6680",
    "canOpen": true,
    "canRestart": true,
    "lastKnownGoodAvailable": true,
    "network": {
      "mode": "local",
      "listen": "127.0.0.1:6680",
      "primaryOrigin": "http://localhost:6680",
      "tlsMode": "off",
      "proxyMode": "disabled",
      "passkeyOrigin": "http://localhost:6680",
      "rpID": "localhost",
      "remotePasskeyAvailable": false
    }
  }
}
```

迁移完成后删除长期重复的：

```text
ready
serverURL
stage
network
```

旧顶层字段，不保留双 contract。

### 10.2 Restart

```http
POST /__onb/runtime/restart
```

可返回 `202 accepted`，由 panel polling观察 phase。

### 10.3 Read config

```http
GET /__onb/runtime/config
```

返回：

```json
{
  "currentToml": "...",
  "candidateToml": "...",
  "baseFingerprint": "sha256:...",
  "lastKnownGoodAvailable": true,
  "hostManagedPaths": [],
  "network": {},
  "issues": []
}
```

### 10.4 Patch network candidate

```http
POST /__onb/runtime/config/patch-network
```

Request：

```json
{
  "baseFingerprint": "sha256:...",
  "toml": "...",
  "network": {
    "mode": "external_https",
    "primaryOrigin": "https://photos.example.com",
    "proxyLocation": "same_host",
    "listen": "127.0.0.1:6680",
    "trustedProxyCIDRs": ["127.0.0.1/32", "::1/128"],
    "acceptLANWarning": false
  }
}
```

Response：

```text
updated candidate TOML
resolved network summary
semantic changes
issues
```

该接口不持久化、不重启。

### 10.5 Validate

```http
POST /__onb/runtime/config/validate
```

返回：

```text
valid
canonical candidate TOML
resolved config summary
field/code/message issues
semantic changes
requiresRestart
```

### 10.6 Apply

```http
POST /__onb/runtime/config/apply
```

规则：

- stale fingerprint → `409`；
- operation active → `409`；
- validation failure → `400`；
- accepted operation → `202`；
- panel通过 `/state` 查看最终结果。

### 10.7 Restore

```http
POST /__onb/runtime/config/restore
```

使用同一 apply engine。

### 10.8 Existing `/__onb/network`

迁移阶段可作为 compatibility endpoint。

统一 Settings 完成后：

- Server tab 改用 candidate patch/apply；
- 删除 `/__onb/network`；
- 删除 `NetworkSavePayload/Result`；
- 避免两套网络 Apply 路径长期并存。

## 11. Panel component plan

目标结构：

```text
desktop/panel/src/components/dashboard/
├── Dashboard.svelte
├── ServerCard.svelte
├── HubCard.svelte
├── StorageLocationsPanel.svelte
├── SupportPanel.svelte
├── LogPanel.svelte
├── PathsPanel.svelte
├── settings/
│   ├── SettingsDialog.svelte
│   ├── GeneralSettings.svelte
│   ├── ServerSettings.svelte
│   ├── LumenSettingsForm.svelte
│   ├── RuntimeConfigSettings.svelte
│   └── SemanticChanges.svelte
└── shared/
    ├── ServiceStatus.svelte
    └── CopyableOrigin.svelte
```

只在确实降低复杂度时拆组件，不为每一行创建 wrapper。

### 11.1 `Dashboard.svelte`

改为：

```svelte
<header />
<div class="grid">
  <ServerCard />
  <HubCard />
</div>
<StorageLocationsPanel />
<SupportPanel />
<SettingsDialog />
```

自动 polling 条件：

```text
runtime starting/restarting
Lumen busy
config operation active
```

稳定时保持当前 on-demand refresh。

### 11.2 `PhotosCard.svelte`

重命名/替换为 `ServerCard.svelte`。

它只消费 typed runtime snapshot，不自行推导 URL/TLS/Passkey。

### 11.3 `HubCard.svelte`

保留所有即时 actions。

删除自己拥有的 `ConfigureDialog` open state，改为打开全局 Settings 的 Lumen tab。

### 11.4 `ConfigureDialog.svelte`

抽取成：

```text
LumenSettingsForm.svelte
```

保留：

- `untrack` 防止 polling 覆盖编辑；
- backend/profile 推荐；
- preset；
- cache picker；
- relocation confirm；
- error handling。

完成 parity 后删除旧 dialog。

### 11.5 `SettingsPanel.svelte`

其内容迁移：

```text
Region → General
Network → Server
```

完成后删除旧 inline card。

### 11.6 `SupportPanel.svelte`

组合现有 LogPanel 和 PathsPanel。

不要把两者全部复制进一个超大文件。

### 11.7 TitleBar

保留当前语言 toggle。

P0 不新增重复的 Settings language selector。

若要持久化 toggle，可作为独立小改动调用现有 settings API，但不阻塞 Dashboard。

## 12. TypeScript state

新增：

```ts
type RuntimePhase =
  | "stopped"
  | "starting"
  | "running"
  | "restarting"
  | "failed";

interface RuntimeSnapshot {
  phase: RuntimePhase;
  stage?: string;
  errorCode?: string;
  errorMessage?: string;
  browserURL?: string;
  canOpen: boolean;
  canRestart: boolean;
  lastKnownGoodAvailable: boolean;
  network?: RuntimeNetworkSummary;
}
```

Store helper：

```text
runtimeBusy
runtimeRunning
runtimeFailed
canOpenLumilio
anyServiceBusy
```

`photosStatus()` 改为 Server runtime helper，并删除对 `ready bool` 的依赖。

前端不得从 URL 猜测 RP ID 或 proxy 状态。

## 13. Dev Mock

当前 mock 位于：

```text
desktop/panel/dev/mock-api.ts
```

扩展支持：

```text
runtime=running
runtime=starting
runtime=restarting
runtime=failed

network=local
network=lan
network=external

lumen=disabled
lumen=starting
lumen=running
lumen=failed
```

并模拟：

- restart；
- config read；
- network patch；
- validate success/error；
- apply success；
- candidate fail + rollback success；
- candidate fail + rollback fail；
- restore last-known-good。

可以通过 query parameter 或固定 dev helper选择状态。

Production UI 不得保留“故障预览”按钮。

## 14. 实施阶段

### Phase 0：重新核对基线

1. 确认 HEAD 为 `88a0836b64dafa5527b3bc0024a329800cfa62c7` 或其直接后继。
2. 阅读：
   - root `AGENTS.md`；
   - `desktop/panel/AGENTS.md`；
   - architecture/BACKEND/DESIGN/core-beliefs；
   - 已完成的 deployment-origin plan。
3. 运行 baseline：
   - `make desktop-test`；
   - panel `vp check`；
   - panel `vp test`；
   - panel `vp build`。
4. 记录当前已知失败。
5. inventory 所有：
   - `ready`；
   - `serverURL`；
   - `ApplyNetworkSettings`；
   - `Supervisor.Stop`；
   - `SettingsPanel`；
   - `ConfigureDialog`；
   - `/__onb/network`；
   - server.toml materialization。

Exit：

- 确认网络 Schema v3 已完成；
- 不再把 deployment-origin 作为待实现依赖；
- 当前网络 E2E 结果已记录。

### Phase 1：安全 Runtime lifecycle

1. 将 host lock 与 runtime generation 分离。
2. 引入 runtimeGeneration。
3. Stop timeout 返回 error并保持 ownership。
4. 清理 stale RepositoryControl。
5. serialized operation。
6. typed RuntimeSnapshot。
7. startup failure保留Wails panel。
8. tray改用snapshot。
9. `/__onb/state`新增typed runtime。
10. `/__onb/runtime/restart`。

Focused tests：

- start success；
- start failure不Quit；
- restart success；
- stop timeout不允许第二Start；
- concurrent restart 409；
- host lock在Runtime failed时仍持有；
- RepositoryControl在restart窗口不可用。

Exit：

- failed/restarting可被panel观察；
- lifecycle安全边界先于UI apply完成。

### Phase 2：Dashboard layout

1. 新建 ServerCard。
2. Server/Lumen responsive grid。
3. Header Settings。
4. canonical Open Lumilio。
5. Server failed Recovery UI。
6. Storage仅重排。
7. SupportPanel折叠Logs/Paths。
8. 保留所有原动作。
9. mock runtime states。
10. 删除无用旧PhotosCard。

Visual checks：

- 760×720；
- 640×620；
- light/dark；
- zh/en；
- long URL/path；
- running/starting/restarting/failed。

Exit：

- 视觉达到批准原型的信息层级；
- 无假数据；
- 无功能回退。

### Phase 3：统一 Settings，复用现有表单

1. SettingsDialog shell。
2. General移入Region。
3. 当前Network UI抽成ServerSettings。
4. ConfigureDialog抽成LumenSettingsForm。
5. HubCard Configure打开Lumen tab。
6. tab-specific footer。
7. dirty close。
8. 移除旧SettingsPanel。
9. 暂时仍可调用现有 `/network`，保证小步提交。
10. Lumen parity验证。

Exit：

- 只有一个Settings入口；
- Network/Lumen现有行为完全保留；
- 没有两套Lumen form。

### Phase 4：Runtime intent 与 Raw TOML

1. DesktopSettings v2。
2. v1 network迁移到runtime.toml。
3. runtime.toml/LKG/candidate/journal路径。
4. HostProjection。
5. `LoadAppConfigBytes`。
6. config read/validate。
7. network patch candidate。
8. Runtime Configuration tab。
9. host-owned field rejection。
10. semantic changes。
11. fingerprint。
12. structured/raw 同candidate。

Exit：

- runtime.toml是持久化Runtime source；
- server.toml仍是每generation materialized immutable input；
- frontend不解析TOML；
- active file不被直接危险编辑。

### Phase 5：统一 Apply 和 Rollback

1. 通用 RuntimeConfigManager。
2. staged candidate。
3. safe StopRuntime。
4. promote/start/readiness。
5. LKG更新。
6. rollback。
7. journal reconciliation。
8. restore endpoint。
9. ServerSettings切到统一candidate/apply。
10. 删除 `/network` 和 `ApplyNetworkSettings`旧专用路径。
11. E2E network rollback改为通用config rollback。
12. manual restart不打开新browser tab。

Focused tests：

- apply success；
- stale fingerprint；
- host field rejected；
- stop timeout不promote；
- candidate startup fail + rollback success；
- rollback fail → RuntimeFailed；
- candidate_staged reconciliation；
- candidate_promoted reconciliation；
- v1→v2 external profile preserved；
- first/second launch保留Library ID。

Exit：

- Network和Raw config共用唯一Apply engine；
- bad config不会让Desktop失去恢复入口。

### Phase 6：Mock、a11y、i18n、清理

1. 完整mock scenarios。
2. 所有新copy加入现有i18n table。
3. focus/Escape/dirty dialog。
4. status不只依赖颜色。
5. reduced motion。
6. minimum window。
7. 删除compatibility fields。
8. 删除旧组件/API。
9. 更新Desktop README和内部架构文档。
10. 完成plan outcomes。

### Phase 7：P1 可选

不阻塞：

- sanitized diagnostics ZIP；
- Dashboard update badge；
- backend-proven uptime；
- backend-proven free-space；
-更详细stage timeline。

## 15. 验证门

遵循仓库工具。

Panel：

```bash
cd desktop/panel
vp install
vp check
vp test
vp build
```

Desktop：

```bash
make desktop-test
make desktop-build
```

由于会修改 `server/config`：

```bash
make server-test
```

不需要 `make dto`，除非意外修改 public HTTP API；本计划只修改 private `/__onb`。

### 必须的聚焦 Go tests

- DesktopSettings v1→v2 migration；
- runtime intent strict load；
- host projection；
- host-owned rejection；
- typed panel state；
- startup failure keeps control panel；
- restart serialization；
- Stop timeout；
- config fingerprint；
- candidate apply；
- rollback；
- journal reconciliation；
- first/second launch；
- network external/local/LAN semantics；
- log allowlist；
- Storage现有测试保持。

### 必须的 panel checks

不引入新重型框架。

覆盖：

- runtime状态映射；
- Server actions enablement；
- Settings tab footer；
- network draft；
- raw validation issues；
- semantic warnings；
- dirty close；
- Lumen form parity；
- Recovery actions。

### 手工视觉矩阵

```text
Window:
760×720
640×620

Theme:
light
dark

Language:
zh
en

Runtime:
starting
running
restarting
failed

Network:
local
LAN HTTP
external HTTPS

Lumen:
disabled
starting/downloading
running
failed

Settings:
General
Server
Lumen
Runtime
validation error
apply
rollback
```

不要求完整跨平台 UI automation；现有 CI build/test matrix继续作为跨平台门。

## 16. Definition of Done

- [ ] 基线记录为 `88a0836b64dafa5527b3bc0024a329800cfa62c7`。
- [ ] 不重复实现已经完成的 Schema v3/network/TLS/proxy。
- [ ] Wizard职责不变。
- [ ] Dashboard仍是本机Runtime控制面。
- [ ] Runtime有typed stopped/starting/running/restarting/failed状态。
- [ ] Server startup failure不再直接退出Desktop。
- [ ] host lock在Desktop生命周期内保持。
- [ ] Stop timeout阻止第二generation。
- [ ] Header有Refresh、Settings、Open Lumilio。
- [ ] Open Lumilio使用primary origin。
- [ ] Server/Lumen为首屏primary cards。
- [ ] ServerCard有Recovery状态。
- [ ] HubCard保留全部现有actions。
- [ ] Lumen form复用现有实现。
- [ ] Storage全部现有流程保持。
- [ ] Logs/Paths进入低优先级SupportPanel。
- [ ] 统一Settings Dialog完成。
- [ ] Region移入General。
- [ ] 现有三种Network UI迁入Server tab。
- [ ] Lumen配置迁入Lumen tab。
- [ ] Runtime tab区分Current和Candidate。
- [ ] `runtime.toml`成为持久化Runtime intent。
- [ ] `server.toml`仍是materialized immutable input。
- [ ] DesktopSettings v1→v2迁移保留现有网络配置。
- [ ] structured/raw编辑同一candidate。
- [ ] frontend不复制Server config validation。
- [ ] host-owned fields不可修改。
- [ ] Desktop拒绝ACME。
- [ ] candidate使用fingerprint。
- [ ] Apply使用staging、safe stop、readiness、LKG rollback。
- [ ] 未完成journal可在下次启动reconcile。
- [ ] 旧 `/__onb/network` 和专用 `ApplyNetworkSettings` 最终删除。
- [ ] 不显示虚假capability/health数据。
- [ ] mock覆盖核心状态。
- [ ] light/dark、最小窗口、zh/en、keyboard检查通过。
- [ ] panel、desktop、server质量门通过。
- [ ] 文档更新。
- [ ] plan Progress、Decision Log、Surprises、Outcomes完整。
- [ ] plan移入completed。
- [ ] working tree clean。

## 17. Agent执行纪律

- 这是 living plan，每阶段更新 Progress。
- 先保留现有网络功能，再逐步迁入统一candidate。
- 不建立临时Schema v2 UI。
- 不复制Lumen或Storage业务逻辑。
- 不让TypeScript负责安全配置验证。
- 不直接把generated `server.toml`作为持久化编辑目标。
- 不在Stop未确认成功时启动第二Runtime。
- 不把startup error只放在native dialog。
- 不通过吞错、隐藏测试或删除rollback让CI变绿。
- 每阶段形成小型可审查commit。
- generated文件只能通过仓库命令生成。
- 不覆盖无关用户改动。
- 最终报告列出：
  - component tree；
  - runtime lifecycle；
  - config file roles；
  - settings migration；
  - private endpoints；
  - apply/rollback；
  - preserved Storage/Lumen behavior；
  - validation commands；
  - P1 backlog。

## Progress

- [x] Phase 0：Baseline
  - 2026-07-26 重新确认 `HEAD == 88a0836b64dafa5527b3bc0024a329800cfa62c7`，
    分支为 `experimental/sqlite`，它与 `origin/experimental/sqlite` 同步；唯一已有工作树内容是
    本计划的未跟踪草稿，现已归位到本节声明的短路径。
  - Schema v3、Origin/TLS/Proxy、Desktop local/LAN HTTP/external HTTPS、
    canonical `ServerURL()` 与现有 network restart/rollback E2E 均已在基线代码中确认，
    不作为本计划的待实现依赖。
  - `make desktop-test` 在受限 sandbox 中因 loopback bind `EPERM` 失败；允许本地 listener 后
    完整通过（包括 `TestDesktopRuntimeFirstAndSecondLaunch` 和
    `TestDesktopNetworkRestartRollback`）。
  - Panel baseline：`vp install` 已是最新；`vp check` 通过（15 files formatted，
    26 files lint/type clean）；`vp build` 通过（682 modules）；`vp test` 因基线没有任何
    `*.test` / `*.spec` 文件而以 code 1 报告 `No test files found`。
  - Inventory：host 仍以 `desktopApp.ready/status/lastStage` 表示 Server；tray 的 Open action
    读取 `ready`；普通 startup failure 显示 native error 后直接 `app.Quit()`。
  - Inventory：`Supervisor.Start` 每 generation 获取 host lock，失败或 `Stop` 时释放；
    `Stop` timeout 仅日志后清空 cancel，允许后续 `Start`；RepositoryControl 在 restart
    窗口不会主动清空；没有 operation serialization 或 typed snapshot。
  - Inventory：network source-of-truth 仍在 DesktopSettings v1；每次 `Start` 从
    `server.template.toml` materialize `config/server.toml` 并 strict-load；
    `/__onb/network` 调用专用 `ApplyNetworkSettings`，先持久化候选再 stop/start，
    只覆盖正常进程内 rollback。
  - Inventory：Dashboard 仍按 `PhotosCard → StorageLocationsPanel → HubCard → LogPanel →
    SettingsPanel → PathsPanel` 等权纵排；Hub 自有 `ConfigureDialog`；Panel state/API/mock
    仍暴露顶层 `ready/serverURL/stage/network`。
- [x] Phase 1：Runtime lifecycle
  - 新增 `RuntimePhase`、`RuntimeSnapshot`、strict-config-derived `NetworkSummary`；
    panel、tray 和 Server actions 读取同一个 snapshot，Panel helper 不再依赖顶层 `ready`。
  - `Prepare`/`Close` 明确拥有 host lock；`runtimeGeneration{cancel, done, err}` 只拥有一次
    `server/app` generation；restart 和 legacy network apply 都不释放 host lock。
  - `StopRuntime` 必须等待 generation `done`；timeout 返回
    `ErrRuntimeStopTimeout`、保留 generation ownership，并让后续 `Start` 返回
    `ErrRuntimeGenerationActive`，不会启动第二个 listener/River/SQLite generation。
  - initial start、manual restart、network apply 和 shutdown 共用 `operationMu`；
    panel `POST /__onb/runtime/restart` 在 claim gate 后返回 `202`，并发请求得到
    `409 operation_in_progress`。
  - stop/restart 在 cancel 前主动清空 `RepositoryControl`，新 generation ready 后仍由
    `server/app` 的既有 hook 重新发布。
  - 普通 manifest/listen/database/startup failure 进入 `RuntimeFailed`，保留 tray 并打开
    Control Panel；只有 host-level `ErrAlreadyRunning` 仍退出第二个 Wails host。
  - `/__onb/state` 已新增 typed `runtime` contract；旧顶层 compatibility fields 暂留到
    Phase 6 清理。tray Open action 与 host launcher 只在 snapshot `CanOpen` 时使用
    canonical `BrowserURL`。
  - Focused tests：stop timeout/second generation、host lock failed lifetime、concurrent
    restart、RepositoryControl clearing、strict-derived network summary、host fatal
    classification、typed panel state、real restart E2E。
  - 验证：`make desktop-test` 通过；Panel `vp check --fix` 后 lint/type/format 通过，
    `vp test` 为 1 file / 6 tests passed，`vp build` 通过（682 modules）。
- [x] Phase 2：Dashboard layout
  - Dashboard header is now `Local Runtime / Desktop Control Panel` with Refresh, the single
    forward-compatible Settings affordance, and canonical Open Lumilio gating from the typed
    runtime snapshot.
  - Added a responsive primary-service grid (`1` column at 640 px, `2` columns from 700 px) with
    a new Server card and the existing Lumen behavior reorganized into a peer card. Storage
    Locations remains the same business component and spans the full content width.
  - Server displays only strict-config-derived runtime facts: lifecycle phase/stage, canonical
    browser URL, network mode, TLS, proxy, and passkey availability. Failed startup presents the
    real error, Retry, Edit configuration, LKG availability, and an error-log recovery path;
    no health, uptime, or inferred service data was added.
  - Existing App/Error/Lumen logs and local paths now live in one native-details Support
    disclosure. It defaults closed and automatically opens when Server or Lumen enters failed.
  - Preserved Lumen enable/disable/update/configure/log/reveal behavior and surfaced previously
    silent action errors. The old presentation-only `PhotosCard` was removed.
  - Dev mock accepts
    `?mode=dashboard&runtime=stopped|starting|running|restarting|failed`, including a realistic
    strict-manifest failure, so the complete runtime matrix is deterministic.
  - Verification: `make desktop-test` passed; Panel `vp check --fix` passed for 28 files,
    `vp test` passed (1 file / 7 tests), and `vp build` passed (683 modules).
  - The in-app Browser skill could not execute the 760×720 / 640×620, theme, locale, and
    long-value visual matrix because the connected runtime reported no available browser
    (`agent.browsers.list() == []`). The deterministic mock states remain available and the
    visual matrix stays a final DoD item rather than being claimed as observed.
- [x] Phase 3：Unified Settings
  - Replaced the inline Settings panel and standalone Lumen Configure dialog with one
    focus-managed `SettingsDialog` containing General, Server, and Lumen tabpanels.
  - Header Settings opens General, both Server settings/recovery actions open Server, and both
    Lumen Configure affordances open Lumen. There is no second settings or Lumen form.
  - General reuses `RegionSelect`; ServerSettingsForm preserves the complete local/LAN
    HTTP/external HTTPS form, LAN acknowledgement, RP-ID warning, and the existing temporary
    `/__onb/network` apply route; LumenSettingsForm reuses BackendPicker, PresetPicker, PathPicker,
    save payload, and the two-step cache-move confirmation.
  - All forms seed once per dialog-open session so background runtime polls cannot overwrite an
    edit. Tab switches preserve edits; unsaved tabs carry a visible semantic status dot; overlay,
    Escape, Close, and Cancel use one dirty-close confirmation.
  - The footer follows the active tab: Save changes, Apply and restart, or Move and save. Save is
    disabled for a clean form, while saving, or until the LAN risk acknowledgement is checked.
  - Verification: `make desktop-test` passed with Svelte diagnostics at 0 errors / 0 warnings;
    Panel `vp check --fix`, `vp test` (1 file / 7 tests), and production build (685 modules)
    passed.
- [x] Phase 4：Runtime intent/TOML
  - Added explicit DesktopSettings v1/v2 disk structs. v2 persists only host/control-plane
    choices (including LAN acknowledgement and Lumen), while network compatibility fields are
    derived in memory from the resolved runtime intent and never serialized back to JSON.
  - First access migrates a v1 local/LAN/external profile by rendering the existing schema-v3
    template, strict-loading it, atomically creating `runtime.toml` and the migration baseline
    `runtime.last-known-good.toml`, then atomically replacing desktop-settings.json with v2.
    Focused coverage proves an external HTTPS origin/listen/trusted-proxy profile survives.
  - Added the declared paths for persistent intent, LKG, staged candidate, and apply journal.
    Startup now reads `runtime.toml`, applies HostProjection, strict-loads the materialized bytes,
    atomically writes generation-owned `server.toml` at mode 0600, and starts from that immutable
    config.
  - HostProjection owns every path listed in section 6.2 plus schema/environment, empty desktop
    CORS, TLS auxiliary fields, and the supervised Lumen static node/deployment ID. Candidate
    edits to those fields return structured `host_managed` issues; Desktop also explicitly rejects
    TLS ACME.
  - `server/config.LoadAppConfigBytes` is the single candidate validator and preserves unknown
    field rejection, presence checks, relative path resolution, deployment rules, manifest
    fingerprint, and the loaded invariant. `LoadAppConfig` now only resolves/reads before calling
    the same function.
  - Added `GET /__onb/runtime/config`, `POST .../validate`, and `POST .../patch-network`.
    They return a `sha256:` base fingerprint, canonical candidate, strict resolved network,
    field/code/message issues, and semantic changes for network mode/listen/origin, Passkey
    identity/enabled, logging, repository scan, and transcode acceleration. Stale drafts return
    `409 stale_fingerprint`; endpoints remain private and do not persist or restart.
  - Settings now includes Runtime Configuration with read-only Current, explicit Candidate editor,
    Validate, Reset, active-manifest reveal, host-managed field disclosure, issues, and semantic
    changes. The frontend never parses TOML. Its apply/restore actions remain for Phase 5.
  - The legacy `/__onb/network` compatibility route temporarily patches the same runtime intent
    before using its existing synchronous restart/rollback behavior; it no longer restores network
    source fields to v2 JSON and is still scheduled for complete removal in Phase 5.
  - Verification: `make server-test` passed; `make desktop-test` passed including v1 migration,
    host projection, stale fingerprint, ACME/host-field rejection, private endpoint, first/second
    launch, and existing network rollback tests; Panel `vp check --fix`, `vp test` (1 file /
    7 tests), and `vp build` (686 modules) passed.
- [x] Phase 5：Apply/Rollback
  - Added one serialized runtime apply engine for both raw TOML and structured Server settings.
    It validates the optimistic base fingerprint, stages `runtime.candidate.toml`, writes
    `runtime-apply.json`, proves the old generation stopped, promotes the candidate, waits for
    readiness, and only then replaces `runtime.last-known-good.toml`.
  - Journal phases are exactly `candidate_staged`, `candidate_promoted`, and `rolling_back`.
    `Prepare` reconciles an interrupted staged candidate by discarding it, and restores LKG for
    promoted/rolling-back states before starting a generation. Direct tests cover all three.
  - Candidate readiness failure restores LKG and starts it through the same generation lifecycle.
    A successful rollback returns to `RuntimeRunning` with a visible `candidate_rolled_back`
    recovery notice; rollback failure keeps the journal with both candidate and rollback errors
    and leaves the host in `RuntimeFailed`.
  - Stop timeout cannot promote the candidate or start a second generation. The apply operation
    remains active through readiness, LKG persistence, and journal cleanup, so observers cannot
    see a prematurely settled state.
  - Added `POST /__onb/runtime/config/apply` and `POST .../restore`: accepted work returns `202`,
    concurrent/stale requests return `409`, and invalid candidates return structured `400`.
    Restore projects the current host-owned fields onto LKG and stages it through the same apply
    engine.
  - Server and Runtime Configuration tabs now share one session-scoped candidate draft.
    Structured network edits use the backend patch endpoint and then the common apply endpoint;
    raw edits validate/apply the same bytes. The Server recovery card also exposes LKG restore.
  - Removed the old `/__onb/network`, frontend `saveNetwork`, and Supervisor
    `ApplyNetworkSettings` path. Unknown `/__onb/*` routes return 404 instead of falling through to
    the SPA, with a regression test proving the retired endpoint stays gone.
  - Existing real SQLite network rollback coverage is now a generic candidate-startup-failure /
    LKG-rollback E2E. Additional focused coverage proves success, stop-timeout no-promote,
    rollback failure, journal reconciliation, stale/host rejection, and the pre-existing
    first/second launch and v1 external-profile preservation.
  - Verification: focused apply and private-endpoint tests passed; full `make desktop-test`
    passed with Panel Svelte diagnostics at 0 errors / 0 warnings. Explicit Panel
    `vp check --fix`, `vp test` (1 file / 7 tests), and production build also passed.
- [ ] Phase 6：Mock/A11y/i18n/cleanup
- [ ] Phase 7：Optional P1/docs

## Decision Log

| Date | Decision | Reason | Consequence |
|---|---|---|---|
| 2026-07-26 | 基线固定为 `88a0836` | Schema v3和网络矩阵已经落地 | 不再将deployment-origin作为待实现依赖 |
| 2026-07-26 | 现有Network UI迁入Modal，不重写语义 | 当前已有Apply/Restart/Rollback和E2E | 实现重点转向统一candidate与lifecycle |
| 2026-07-26 | `runtime.toml`与`server.toml`分工 | generated server.toml不能成为用户持久化配置 | 前者是intent，后者是generation input |
| 2026-07-26 | DesktopSettings升级v2并移除network source-of-truth | 防止JSON、structured form、raw TOML drift | network由runtime intent解析 |
| 2026-07-26 | host lock跨Runtime generation持有 | Server失败时control panel仍需保持单实例 | restart不释放lock |
| 2026-07-26 | raw/structured共享一个candidate | 防止两套设置漂移 | network patch由Go backend修改TOML |
| 2026-07-26 | Lumen配置只抽取复用 | 现有配置流程成熟 | 不建立第二套form/backend |
| 2026-07-26 | Diagnostics export为P1 | 核心目标是配置、生命周期和恢复 | 不阻塞主实现 |
| 2026-07-26 | Host lock由`Prepare/Close`拥有，generation由`Start/StopRuntime`拥有 | failed Dashboard与restart不能释放单实例边界 | 只有Desktop Quit释放lock |
| 2026-07-26 | generation completion使用close-only `done`，不消费一次性error channel | startup waiter与stop waiter都需要观察同一退出事实 | timeout后ownership可安全保留并在真实退出后reap |
| 2026-07-26 | operation gate对panel mutation使用`TryLock`，shutdown使用阻塞`Lock` | panel需要确定的409，Quit必须等待当前安全操作收敛 | apply/restart并发拒绝，Close串行完成 |
| 2026-07-26 | 只有`ErrAlreadyRunning`是host-fatal startup error | 其他错误都可由同一Wails host恢复 | port/config/database失败不再直接Quit |
| 2026-07-26 | Dashboard cards use daisyUI semantic card/alert/button/status primitives and a single native-details collapse | Existing panel already uses daisyUI; these primitives preserve keyboard semantics and theme tokens without adding another design system | Runtime hierarchy is compact and responsive; Support stays one disclosure rather than nested panels |
| 2026-07-26 | Phase 2 Settings scroll target and Lumen Configure dialog are temporary compatibility bridges | Phase 2 must preserve all actions while Phase 3 owns the unified modal extraction | No duplicated forms are introduced; both bridges are removed in Phase 3 |
| 2026-07-26 | Keep all three Settings forms mounted for an open session | Tab navigation must not discard edits and background polls must not clobber them | Each form re-seeds only when the dialog session increments; dirty state spans tabs and guards close |
| 2026-07-26 | Keep `/__onb/network` only as the Phase 3 ServerSettings transport | Unified UI and runtime-intent replacement are intentionally separate commits | Existing Network behavior remains testable until Phase 5 atomically replaces and removes the legacy route |
| 2026-07-26 | Decode DesktopSettings through explicit v1 and v2 disk structs while retaining transient in-memory network compatibility fields | Migration must detect legacy fields intentionally, but Phase 3 callers still need a resolved NetworkInfo until cleanup | v2 JSON cannot drift from runtime.toml; Supervisor.Settings derives the temporary compatibility view from strict AppConfig |
| 2026-07-26 | HostProjection is applied to a generic TOML copy immediately before the real strict loader | User policy must survive while machine paths and supervised endpoints remain host-owned | runtime.toml stays complete/editable, server.toml is the projected immutable generation input, and TypeScript has no config parser |
| 2026-07-26 | Candidate fingerprint hashes the exact active runtime.toml bytes with a `sha256:` prefix | Raw and structured editors need optimistic concurrency without treating generated server.toml as source | Any relaunch/apply that changes intent makes an older draft deterministically stale |
| 2026-07-26 | Treat migration's strict-loaded initial intent as the initial LKG baseline | The v1→v2 sequence in the approved plan requires an immediately recoverable equivalent profile | Subsequent Phase 5 readiness applies replace LKG only after the new generation is proven ready |
| 2026-07-26 | Keep apply ownership active until LKG persistence and journal cleanup finish | RuntimeRunning alone does not prove the transaction metadata has converged | Panel polling and tests cannot observe active intent with a stale LKG |
| 2026-07-26 | Restore LKG by projecting current host fields and using the normal candidate engine | Machine-owned paths may have changed since an older LKG was written, while recovery needs identical safety semantics | Restore validates, stages, stops, promotes, starts, and journals exactly like any other apply |
| 2026-07-26 | Reserve `/__onb/` as an API namespace with a 404 fallback | The SPA fallback otherwise turns removed or misspelled private endpoints into misleading HTML 200 responses | Retired `/__onb/network` is observably absent and future client mistakes fail closed |

## Surprises & Discoveries

- 任务附件目录只包含目标文本，没有计划中提到的
  `lumilio-desktop-dashboard-uiux.html`。P0 视觉实现以计划已批准的信息层级、明确窗口矩阵
  和当前成熟组件行为为准，不臆造缺失原型细节。
- Panel baseline 没有测试文件，因此 `vp test` 的 code 1 是已知覆盖缺口，而非断言回归；
  后续阶段必须添加 panel tests，使同一命令成为真正通过的质量门。
- `make desktop-test` 的第一次失败是 sandbox 禁止 loopback listener (`bind: operation not
  permitted`)；提权重跑通过，说明现有 network E2E 本身为绿。
- `server/app` 已在 Repository manager ready hook 的 defer 中发布 `nil`，但那发生在
  generation shutdown 后段；Supervisor 仍需在发出 cancel 前主动清空，才能保证整个
  restart 窗口不暴露旧 manager。
- 原 `serverErr chan error` 同时承担 startup/stop waiter，消费后无法让另一个观察者可靠
  判断同一 generation 是否退出；改为 `done` close + close 前写入 `err` 后，所有 waiter
  共享同一个 happens-before 事实。
- Browser connector bootstrap succeeded far enough to inspect bindings, but the runtime exposed no
  browser instance. Per the Browser skill, visual claims were not substituted with an unrelated
  automation stack; the mock query matrix records the intended observable states for a later
  connected-browser pass.
- The first Phase 3 full gate found one stale dropdown assignment to the deleted
  `configureOpen` variable that the faster `vp check --fix` command did not report. The embedded
  panel's `svelte-check` did report it; routing that item to the shared Lumen tab fixed the only
  failure, and the complete rerun passed.
- The first Phase 4 desktop run compiled only after removing a now-unused template `paths` local,
  then correctly failed three tests whose assertions still encoded v1 JSON as the network source.
  Replacing those assertions with v2 host-only round trips and explicit v1 migration coverage made
  the changed ownership executable rather than weakening validation.
- The first Phase 5 success E2E exposed two observation issues rather than an apply failure:
  `OperationActive` cleared as soon as readiness returned, before LKG/journal convergence, and the
  test assumed the TOML encoder always used double quotes. Keeping the operation claimed through
  transaction cleanup and asserting the parsed `logging.level` made both the runtime contract and
  the test semantic.

## Outcomes & Retrospective

完成后填写：

- Final commit:
- Final component tree:
- Runtime state model:
- Host lock/generation model:
- DesktopSettings migration:
- Runtime file layout:
- Host-managed fields:
- Private API:
- Apply journal:
- Lumen parity:
- Storage parity:
- Removed components/endpoints:
- Panel validation:
- Desktop validation:
- Server validation:
- Known limitations:
- P1 follow-ups:
- Merge recommendation:

### Critical Files for Implementation

- `desktop/app.go`
- `desktop/onboarding.go`
- `desktop/control_panel.go`
- `desktop/supervisor/config.go`
- `desktop/supervisor/manifest.go`
- `desktop/supervisor/supervisor.go`
- `desktop/panel/src/components/dashboard/Dashboard.svelte`
- `desktop/panel/src/components/dashboard/SettingsPanel.svelte`
- `desktop/panel/src/lib/types.ts`
- `desktop/panel/dev/mock-api.ts`
