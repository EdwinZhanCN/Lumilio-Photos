# Lumilio Photos Canonical Origin、TLS 与 Trusted Proxy 执行计划

> - 路径：`site/docs/internal/exec-plans/completed/deployment-origin-tls-plan.md`
> - 目标分支：`experimental/sqlite`
> - 状态：Implementation complete；真实公网 ACME release smoke 待外部条件
> - 配置版本：Runtime Manifest Schema v2 → v3
> - 数据库：不涉及数据库后端变更
> - 执行原则：先统一安全语义，再实现 Desktop 与 Docker 的部署入口

## 1. Goal

将 Lumilio Photos 的网络部署行为收敛为四个互相独立、含义稳定的概念：

```text
server.listen
    决定 Lumilio 在哪些本机网络接口上接受连接

server.primary_origin
    决定 Lumilio 唯一正式的浏览器 Origin、Cookie/CSRF 基准、
    WebAuthn Origin 和由其推导出的 RP ID

server.tls.mode
    决定 HTTPS 由 Lumilio、外部反向代理，还是完全不提供

server.proxy
    决定是否要求请求经过可信反向代理，以及哪些直接对端可以提供代理头
```

实施完成后，Lumilio 必须明确支持以下五个产品场景：

```text
Desktop local
Desktop LAN HTTP
Desktop external HTTPS
Docker built-in ACME HTTPS
Docker external reverse proxy
```

另设一个仅用于开发与测试的 profile：

```text
Development Vite origin + API origin
```

WebAuthn 配置不再允许用户分别配置 RP mode、RP ID 和 RP origins：

```text
Passkey Origin = normalized server.primary_origin
RP ID          = normalized primary_origin.hostname
Allowed Origin = exactly [server.primary_origin]
```

这是一条协议不变量，不是隐式默认值。

## 2. 设计评估与最终修正

用户提出的场景矩阵整体合理，直接采用，但实施时必须包含以下修正。

### 2.1 `primary_origin` 是唯一 canonical origin

接受：

```text
primary_origin 决定正式浏览器地址
primary_origin 决定 Passkey Origin
primary_origin 的 hostname 决定 RP ID
```

不接受：

```text
运行时根据任意请求动态生成 WebAuthn RP
允许多个普通生产 Origin
根据不可信 X-Forwarded-* 改写 WebAuthn Origin
用 CORS origins 代替 primary_origin
```

`cors_allowed_origins` 只处理明确的跨 Origin 浏览器客户端，例如本地 Vite 开发服务器。它不定义 Lumilio 的正式身份。

### 2.2 删除 `server.tls.domain`

ACME certificate name 必须从 `primary_origin.hostname` 推导。额外的 `tls.domain` 会产生两个可以漂移的域名来源。

最终 ACME 配置只保存：

```text
email
storage_path
http_listen
```

### 2.3 ACME 必须拥有 HTTP challenge/redirect listener

`server.listen` 在 ACME 模式下是 HTTPS application listener。另增加：

```toml
server.tls.http_listen
```

用于：

```text
ACME HTTP-01 challenge
普通 HTTP → canonical HTTPS 308 redirect
```

Docker 内部不要求绑定特权端口。推荐：

```text
container 0.0.0.0:8080  ← host 80
container 0.0.0.0:8443  ← host 443
```

浏览器看到的仍然是：

```text
https://photos.example.com
```

P0 不实现 DNS challenge。无法让公网 CA 访问 80/443 的部署应使用外部反向代理，或者等待后续 DNS provider 支持。

### 2.4 Passkey availability 不只读取 HTTP `Origin`

必须区分三个概念：

```text
Target Origin
    浏览器正在访问的 Lumilio 目标地址。
    直接请求由 TLS + Host 得到；
    可信代理请求由受信任代理头得到。

Browser Origin
    有 Origin header 时使用规范化 Origin；
    没有时回退到 Target Origin。

Primary Origin
    配置中的唯一 canonical origin。
```

普通页面导航通常没有 `Origin` header，因此不能把“缺少 Origin”解释为 Passkey 不可用。

Passkey 环境可用条件：

```text
auth.passkey.enabled
AND Browser Origin == Primary Origin
AND Browser Origin 是 HTTPS 或精确的 http://localhost[:port]
AND 请求没有违反 proxy-required policy
```

WebAuthn credential response 中签名的 `clientDataJSON.origin` 仍由 WebAuthn server library严格验证为唯一 `primary_origin`。HTTP header 判断只用于提前阻止错误 ceremony 和改善 UI，不替代密码学验证。

### 2.5 代理信任必须作用于 immediate peer

当前任何客户端都不能因为发送：

```http
X-Forwarded-Proto: https
X-Forwarded-Host: photos.example.com
```

就被当作可信 HTTPS 请求。

只有当 TCP immediate peer 的 IP 位于 `trusted_cidrs` 时，Lumilio 才能读取代理头。`proxy.mode = required` 时，普通应用请求若不是来自可信代理，必须被拒绝。

### 2.6 Docker “默认 ACME”是部署 profile，不是无配置启动

严格 Manifest 在应用启动前必须完整有效；域名和 ACME 邮箱不能通过启动后的网页向导补齐。

P0 采用：

```text
完整配置生成命令
→ 严格 validate
→ 写入 /data/app-state/server.toml
→ docker compose up
```

不在本次引入未认证的网络 bootstrap server，也不让 Runtime 使用普通环境变量覆盖 Manifest。

### 2.7 Desktop LAN HTTP 必须显式警告

Desktop LAN HTTP 会让密码、TOTP、session cookie 和媒体流量在局域网中以明文传输。开启该模式必须经过明确确认，并持续显示：

```text
局域网连接未加密。
Passkey 仅在本机 localhost 可用。
远程设备请使用密码和 TOTP。
```

应用不把“家庭网络”视为可信传输。

## 3. 最终配置 Schema v3

```toml
schema_version = 3
environment = "production"

[server]
listen = "127.0.0.1:6680"
primary_origin = "http://localhost:6680"
cors_allowed_origins = []
web_root = "/resolved/web"

[server.tls]
mode = "off"                  # off | acme | external
http_listen = ""              # 仅 acme 模式非空
email = ""                    # 仅 acme 模式非空
storage_path = ""             # 仅 acme 模式非空

[server.proxy]
mode = "disabled"             # disabled | required
trusted_cidrs = []

[auth]
secret_key_file = "/resolved/auth.key"
access_token_ttl = "15m"
refresh_token_ttl = "720h"
media_token_ttl = "15m"

[auth.passkey]
enabled = true
name = "Lumilio Photos"

[auth.rate_limit]
# 保留现有字段
```

删除 Schema v2 字段：

```text
server.port
auth.webauthn_rp_name
auth.webauthn_rp_mode
auth.webauthn_rp_id
auth.webauthn_rp_origins
```

新增 resolved runtime types，具体命名可以按项目习惯调整：

```go
type ServerConfig struct {
    Listen             string
    PrimaryOrigin      string
    CORSAllowedOrigins []string
    WebRoot            string
    TLS                TLSConfig
    Proxy              ProxyConfig
}

type TLSConfig struct {
    Mode        TLSMode
    HTTPListen  string
    Email       string
    StoragePath string
}

type ProxyConfig struct {
    Mode         ProxyMode
    TrustedCIDRs []netip.Prefix
}

type PasskeyConfig struct {
    Enabled bool
    Name    string
}
```

构造一个不可变的 resolved policy：

```go
type OriginPolicy struct {
    PrimaryOrigin      string
    PrimaryURL         *url.URL
    RPID               string
    PasskeyEnabled     bool
    TLSMode            TLSMode
    ProxyMode          ProxyMode
    TrustedProxyCIDRs  []netip.Prefix
    CORSAllowedOrigins map[string]struct{}
}
```

`OriginPolicy` 在严格配置加载完成后构造。Router、Auth、Cookie、CSRF、Rate Limiter 和 Runtime Info 使用同一个实例。

## 4. Schema v3 验证规则

### 4.1 `server.listen`

必须：

- 是完整 `host:port` 或 `:port`；
- 端口合法；
- 不允许 URL；
- 不在加载后偷偷补端口；
- 由 `net.Listen` 使用原值。

说明：

```text
127.0.0.1:6680  → IPv4 loopback
0.0.0.0:6680    → 所有 IPv4 interface
[::1]:6680      → IPv6 loopback
[::]:6680       → 所有 IPv6 interface
192.168.1.20:6680 → 指定 LAN interface
```

Desktop LAN P0 默认使用 IPv4 `0.0.0.0:6680`，不暗示它自动覆盖 IPv6。

### 4.2 `server.primary_origin`

必须是一个精确 Origin：

- scheme 只能是 `http` 或 `https`；
- 必须有 hostname；
- 不允许 userinfo；
- 不允许 path、query、fragment；
- 规范化 scheme、hostname 和默认端口；
- hostname 执行 IDNA/ASCII canonicalization；
- 保存最终 normalized origin；
- RP ID 精确取 normalized hostname；
- RP ID 不使用父域，不包含 scheme 和 port。

当 `auth.passkey.enabled = true`：

- `https` hostname 必须能作为 WebAuthn domain；
- `http` 只允许 hostname 精确为 `localhost`；
- 不允许把 HTTP LAN IP 当作 Passkey origin；
- 不允许 HTTP `127.0.0.1` 替代产品约定的 `localhost`。

### 4.3 TLS 组合

#### `mode = "off"`

要求：

```text
primary_origin.scheme = http
http_listen = ""
email = ""
storage_path = ""
proxy.mode = disabled
```

允许：

- Desktop local；
- Desktop LAN HTTP；
- Development。

#### `mode = "acme"`

要求：

```text
primary_origin.scheme = https
primary_origin.hostname 不是 localhost
primary_origin.hostname 不是 IP literal
proxy.mode = disabled
http_listen 非空
email 非空且格式合理
storage_path 是持久化本地路径
listen 与 http_listen 不冲突
```

证书域名仅从 `primary_origin.hostname` 得到。

ACME 初始化失败时拒绝启动，不得降级为 HTTP。

#### `mode = "external"`

要求：

```text
primary_origin.scheme = https
proxy.mode = required
trusted_cidrs 至少一个
http_listen = ""
email = ""
storage_path = ""
```

Lumilio 在 `listen` 上提供内部 HTTP，外部代理提供浏览器可见 HTTPS。

### 4.4 Proxy 组合

#### `mode = "disabled"`

要求：

```text
trusted_cidrs = []
```

忽略所有 Forwarded 和 X-Forwarded-* header。

#### `mode = "required"`

要求：

- 至少一个合法 CIDR；
- 使用 `net/netip` 解析；
- ordinary application traffic 的 immediate peer 必须匹配；
- 可信代理必须提供可解析且一致的 public proto/host；
- public target origin 必须精确等于 `primary_origin`；
- 不可信直连返回结构化错误；
- 只给 loopback health/recovery endpoint 保留窄例外。

不允许默认信任：

```text
0.0.0.0/0
::/0
```

若确有高级需求，必须要求显式危险确认，而不是普通配置。

### 4.5 CORS

- 所有 origin 规范化并去重；
- 禁止 `*` 与 credentialed auth 共用；
- `primary_origin` 不需要放入 CORS；
- production SPA/API 同 Origin 时应为空；
- Development 可允许 Vite Origin；
- CORS 不扩大 WebAuthn allowed origins。

### 4.6 路径

`server.tls.storage_path`：

- ACME 模式必须存在或可创建；
- 必须位于 machine-local state；
- 不得位于媒体 repository；
- Docker 默认 `/data/app-state/tls`；
- 证书文件权限遵循应用私有状态权限。

## 5. 场景矩阵与实际配置

### 5.1 Desktop local：默认

```toml
[server]
listen = "127.0.0.1:6680"
primary_origin = "http://localhost:6680"
cors_allowed_origins = []
web_root = "/resolved/desktop/web"

[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""

[server.proxy]
mode = "disabled"
trusted_cidrs = []

[auth.passkey]
enabled = true
name = "Lumilio Photos"
```

行为：

```text
Desktop 打开 http://localhost:6680
只有本机可连接
本机 Passkey 可注册和登录
```

### 5.2 Desktop LAN HTTP

```toml
[server]
listen = "0.0.0.0:6680"
primary_origin = "http://localhost:6680"
cors_allowed_origins = []
web_root = "/resolved/desktop/web"

[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""

[server.proxy]
mode = "disabled"
trusted_cidrs = []

[auth.passkey]
enabled = true
name = "Lumilio Photos"
```

行为：

```text
本机 http://localhost:6680
    Passkey 可用

远程 http://192.168.x.x:6680
    Passkey 不可用
    密码/TOTP 可用
    持续显示未加密警告
```

开启 LAN 不修改 `primary_origin`，因此不会改变 localhost RP ID。

### 5.3 Desktop external HTTPS：同机代理

```toml
[server]
listen = "127.0.0.1:6680"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = "/resolved/desktop/web"

[server.tls]
mode = "external"
http_listen = ""
email = ""
storage_path = ""

[server.proxy]
mode = "required"
trusted_cidrs = [
  "127.0.0.1/32",
  "::1/128",
]

[auth.passkey]
enabled = true
name = "Lumilio Photos"
```

Caddy：

```caddyfile
photos.example.com {
    reverse_proxy 127.0.0.1:6680
}
```

行为：

```text
Desktop 打开 https://photos.example.com
所有设备使用同一正式地址
所有设备可使用 Passkey
直接 localhost 应用入口不作为普通产品入口
```

### 5.4 Desktop external HTTPS：远端代理

```toml
[server]
listen = "192.168.1.20:6680"
primary_origin = "https://photos.example.com"

[server.tls]
mode = "external"
http_listen = ""
email = ""
storage_path = ""

[server.proxy]
mode = "required"
trusted_cidrs = ["192.168.1.10/32"]
```

行为：

```text
192.168.1.10 代理 → 接受
其他 LAN 客户端直接访问 192.168.1.20:6680 → 拒绝
loopback health/recovery → 仅窄例外
```

### 5.5 Docker built-in ACME HTTPS

容器内：

```toml
[server]
listen = "0.0.0.0:8443"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = "/app/web"

[server.tls]
mode = "acme"
http_listen = "0.0.0.0:8080"
email = "admin@example.com"
storage_path = "/data/app-state/tls"

[server.proxy]
mode = "disabled"
trusted_cidrs = []

[auth.passkey]
enabled = true
name = "Lumilio Photos"
```

Docker mapping：

```yaml
ports:
  - "80:8080"
  - "443:8443"
```

行为：

```text
80 → ACME challenge 或 308 到 primary_origin
443 → Lumilio HTTPS SPA + API
证书与 ACME account state持久化
申请/续期失败不会降级为明文服务
```

运行前提：

- 用户控制域名；
- A/AAAA 正确指向部署入口；
- 公网 CA 可访问 80 或 443；
- Docker state volume持久化。

### 5.6 Docker external reverse proxy

```toml
[server]
listen = "0.0.0.0:6680"
primary_origin = "https://photos.example.com"
cors_allowed_origins = []
web_root = "/app/web"

[server.tls]
mode = "external"
http_listen = ""
email = ""
storage_path = ""

[server.proxy]
mode = "required"
trusted_cidrs = ["172.30.0.0/24"]

[auth.passkey]
enabled = true
name = "Lumilio Photos"
```

默认 compose 不发布 Lumilio 的 6680 到宿主机：

```yaml
services:
  lumilio:
    expose:
      - "6680"
    networks:
      - lumilio_proxy
```

反向代理加入同一个专用网络。CIDR trust 是第二层保护，网络隔离是第一层保护。

不得把所有业务容器放进一个宽泛且全部可信的 subnet。

### 5.7 Development profile

保留当前 Vite 与 API 分离的开发方式：

```toml
[server]
listen = "127.0.0.1:6680"
primary_origin = "http://localhost:6657"
cors_allowed_origins = ["http://localhost:6657"]
web_root = ""

[server.tls]
mode = "off"
http_listen = ""
email = ""
storage_path = ""

[server.proxy]
mode = "disabled"
trusted_cidrs = []

[auth.passkey]
enabled = true
name = "Lumilio Photos"
```

此时：

```text
Browser Origin = http://localhost:6657
API Target     = http://localhost:6680
RP ID          = localhost
```

WebAuthn 使用浏览器 Origin `6657`，不是 API target `6680`。

不得为了 development 恢复 origin-derived WebAuthn。

## 6. OriginPolicy 与请求解析

建立单一 package，建议：

```text
server/internal/httporigin/
```

或符合现有结构的等价位置。

### 6.1 核心结构

```go
type RequestContext struct {
    PeerIP             netip.Addr
    ClientIP           netip.Addr
    ViaTrustedProxy    bool
    TargetOrigin       string
    BrowserOrigin      string
    IsPrimaryOrigin    bool
    IsSecureForPasskey bool
}

func (p *OriginPolicy) Resolve(r *http.Request) (RequestContext, error)
func (p *OriginPolicy) PasskeyAvailability(ctx RequestContext) Availability
func (p *OriginPolicy) RequireProxy(ctx RequestContext) error
func (p *OriginPolicy) CookiePolicy(ctx RequestContext) CookiePolicy
func (p *OriginPolicy) ValidateSessionOrigin(ctx RequestContext) error
```

### 6.2 Direct request

当 proxy disabled：

```text
PeerIP = RemoteAddr
ClientIP = PeerIP
忽略所有 Forwarded/X-Forwarded-*
Target scheme = https if r.TLS != nil else http
Target host = r.Host
Browser Origin = normalized Origin header when present, else Target Origin
```

### 6.3 Trusted proxy request

先解析 `RemoteAddr`。只有匹配 `trusted_cidrs` 才读取代理头。

P0 约束：

- 支持一个明确的 immediate trusted proxy 边界；
- proxy 必须覆盖，而不是透传客户端提供的 target proto/host；
- proto 与 host 必须各自只有一个明确值；
- malformed、多值歧义或不同 header family 冲突时拒绝请求；
- 支持 `Forwarded` 与 `X-Forwarded-Proto/Host` 时，若两者同时存在，规范化结果必须一致；
- 不从不可信 peer 读取任何代理头。

Target Origin：

```text
forwarded proto + forwarded host
```

Browser Origin：

```text
Origin header when present
else Target Origin
```

Client IP：

- 使用可信 proxy chain parser；
- 从右向左跳过可信 proxy；
- 第一个非可信地址是 client IP；
- Auth rate limiting、audit log 和 security log 使用同一个结果；
- 不再让 Gin 默认的宽松 proxy inference单独决定安全身份。

### 6.4 `proxy.required` middleware

对 normal application routes：

```text
peer 不可信
    → 403 trusted_proxy_required

可信 peer，但 public target 不完整/非法
    → 400 invalid_forwarded_target

可信 peer，但 Target Origin != Primary Origin
    → 421 misdirected_request
```

允许的直连例外严格限制为：

```text
GET /api/v1/health/live
GET /api/v1/health/ready
```

且 immediate peer 必须是 loopback。

不要给登录、setup、SPA、static assets、backup、restore 或 WebSocket 留直连例外。

## 7. Passkey 运行时改造

### 7.1 静态 WebAuthn identity

在 `AuthService` 初始化时创建唯一 WebAuthn config：

```text
RPDisplayName = auth.passkey.name
RPID          = OriginPolicy.RPID
RPOrigins     = [OriginPolicy.PrimaryOrigin]
```

删除：

- per-request WebAuthn instance；
- origin-derived mode；
- request hostname推导 RP；
- runtime accepted origins list。

### 7.2 Browser capability endpoint

增加不依赖账户的 endpoint：

```http
GET /api/v1/auth/browser-capabilities
```

响应示例：

```json
{
  "primary_origin": "http://localhost:6680",
  "current_origin": "http://192.168.1.20:6680",
  "passkey_available": false,
  "passkey_unavailable_reason": "secure_origin_required",
  "insecure_transport": true,
  "proxy_required": false
}
```

允许的 reason code：

```text
disabled
secure_origin_required
non_primary_origin
trusted_proxy_required
invalid_request_origin
```

这些字段只描述当前部署和请求环境，不泄露账户是否存在或是否注册 Passkey。

### 7.3 Login options

现有 login options 应把：

```text
账户具有 Passkey
AND browser capability 可用
```

合并为最终可展示的 `passkey`。

对未知账户保持当前 enumeration 防护；环境 reason 对所有账户一致。

### 7.4 Ceremony hard gate

以下 endpoint 在开始任何 ceremony 前调用同一个 policy：

```text
Begin registration
Finish registration
Begin login
Finish login
```

要求：

```text
Passkey enabled
Browser Origin == Primary Origin
Origin secure
proxy policy通过
```

Finish 阶段仍必须依赖 WebAuthn library 验证 signed `clientDataJSON.origin` 和 `rpIdHash`。

### 7.5 Credential UX

当配置变化导致 hostname/RP ID 改变：

```text
localhost → photos.example.com
photos.example.com → new.example.com
```

Desktop 设置 UI 必须提示：

```text
现有 Passkey 仍保留，但不能用于新的 RP ID。
请使用密码 + TOTP 登录后重新注册。
```

仅修改端口：

```text
http://localhost:6680 → http://localhost:7780
```

RP ID 仍是 `localhost`，现有 credential 可以继续用于同一 RP ID；server 必须期待新的精确 Origin，不要求无条件删除或重注册 credential。

仅修改 TLS provider 或 certificate、hostname 不变：

```text
acme ↔ external
```

不影响 RP ID 和已有 Passkey。

可选 P1：为 credential record增加 enrollment RP ID/Origin metadata，以便 UI 标识哪些 credential 与当前 Origin兼容。它不阻塞 P0。

## 8. Cookie、Session、CSRF 与 CORS 统一

删除各自独立重建 Origin 的实现。以下模块必须使用同一个 `OriginPolicy` 和 resolved request context：

```text
Cookie Secure / SameSite
Refresh token session cookie
CSRF / trusted session origin
Passkey availability
Passkey ceremony gate
Auth rate-limit client IP
Security audit client IP/origin
CORS decision
Runtime request diagnostics
```

规则：

- `primary_origin` 为 HTTPS 时，正常浏览器 session cookie必须 `Secure`；
- external 模式不能因为内部连接是 HTTP 就生成非 Secure cookie；
- 未受信任 forwarded proto不能影响 cookie；
- CORS credential requests只允许精确规范化 origins；
- Origin header存在时必须符合 primary/CORS policy；
- proxy-required direct requests在进入 auth handler前拒绝；
- ACME mode由应用负责 HSTS，先使用保守值，不默认 `includeSubDomains` 或 preload；
- external mode可以由反向代理负责 HSTS，文档明确代理责任。

## 9. Server Transport

新增 transport 层，不把 CertMagic细节散落在 `app.go`：

```text
server/internal/servertransport/
├── transport.go
├── http.go
├── acme.go
└── external.go
```

建议接口：

```go
type Runtime struct {
    Servers   []*http.Server
    Listeners []net.Listener
}

func Start(
    ctx context.Context,
    cfg config.ServerConfig,
    handler http.Handler,
    logger *zap.Logger,
) (*Runtime, error)

func (r *Runtime) Shutdown(ctx context.Context) error
func (r *Runtime) Wait() error
```

### 9.1 off

```text
net.Listen(server.listen)
http.Server.Serve
```

### 9.2 external

```text
net.Listen(server.listen)
http.Server.Serve
OriginPolicy proxy-required middleware负责安全边界
```

应用本身不加载证书。

### 9.3 acme

```text
CertMagic FileStorage = tls.storage_path
certificate hostname = primary_origin.hostname
HTTPS listener = server.listen
HTTP challenge/redirect listener = tls.http_listen
```

HTTP handler顺序：

```text
ACME HTTP challenge
→ 若已处理则返回
→ 其余请求 308 到 primary_origin，保留 path/query
```

HTTPS server：

- 使用 CertMagic提供的 TLS config；
- 支持 TLS-ALPN challenge；
- certificate acquisition失败则启动失败；
- certificate storage持久化；
- shutdown停止两个 server和renewal runtime；
- 所有 listener确认关闭后，才允许 SQLite restore generation替换数据库。

当前 runtime generation可以继续拥有 transport。恢复数据库时：

```text
drain HTTP/HTTPS
→ stop CertMagic runtime/listeners
→ stop River
→ close SQLite
→ swap restore
→ next generation重新打开
```

不要求本次把 listener提升到 generation loop之外。

### 9.4 Timeouts

为所有面向网络的 server设置显式：

```text
ReadHeaderTimeout
IdleTimeout
MaxHeaderBytes
合理的 shutdown timeout
```

不要使用无边界默认值。

## 10. Runtime Info 与诊断

扩展 immutable runtime info：

```json
{
  "server": {
    "listen": "0.0.0.0:6680",
    "primary_origin": "https://photos.example.com",
    "tls_mode": "external",
    "proxy_mode": "required",
    "trusted_proxy_cidrs": ["192.168.1.10/32"]
  },
  "passkey": {
    "enabled": true,
    "origin": "https://photos.example.com",
    "rp_id": "photos.example.com"
  }
}
```

管理员页面显示：

- listen；
- primary origin；
- TLS owner；
- proxy requirement；
- trusted proxy ranges；
- derived Passkey Origin；
- derived RP ID；
- 当前请求是否通过可信 proxy；
- 当前浏览器 Passkey availability/reason；
- LAN HTTP warning；
- ACME certificate hostname、expiry和last renewal status。

不得显示 ACME account private key或其他 secret。

启动日志至少包含：

```text
normalized listen
normalized primary origin
derived RP ID
TLS mode
proxy mode
trusted CIDR count
ACME storage path
```

## 11. Desktop 产品行为

### 11.1 Settings model

增加 versioned network settings：

```go
type NetworkMode string

const (
    NetworkLocal         NetworkMode = "local"
    NetworkLANHTTP       NetworkMode = "lan_http"
    NetworkExternalHTTPS NetworkMode = "external_https"
)
```

持久化字段：

```text
network_mode
primary_origin
listen
trusted_proxy_cidrs
lan_http_warning_accepted_version
```

Local 和 LAN HTTP 的 primary origin由 Desktop生成并保持 localhost。External HTTPS 要求用户输入正式 HTTPS origin和proxy位置。

Desktop 不支持 `tls.mode = acme`。

### 11.2 Local

默认：

```text
listen = 127.0.0.1:6680
primary_origin = http://localhost:6680
```

### 11.3 LAN HTTP switch

开启：

```text
只把 listen切换为0.0.0.0:6680
primary_origin保持localhost
```

UI：

- 显示局域网IP地址；
- 显示未加密警告；
- 明确远程 Passkey不可用；
- 需要一次危险确认；
- 不承诺自动修改OS firewall。

关闭：

```text
listen恢复127.0.0.1
primary_origin不变
```

### 11.4 External HTTPS

UI收集：

```text
primary_origin
proxy位于本机还是远端
远端proxy IP/CIDR
可选listen address
```

同机 proxy：

```text
listen = 127.0.0.1:6680
trusted = loopback
```

远端 proxy：

```text
listen优先为指定LAN IP
trusted优先为proxy精确/32或/128
```

Desktop launcher打开：

```text
server.primary_origin
```

而不是硬编码：

```text
http://localhost:6680
```

readiness probe使用内部监听地址和loopback health例外，不依赖DNS或外部proxy成功后才知道server进程已启动。产品页面可另行检查primary origin可达性并显示诊断。

### 11.5 编辑、验证、重启和回滚

Desktop 工作流：

```text
编辑候选 settings
→ 使用共享 schema/policy validator生成候选Manifest
→ 展示网络与Passkey变化
→ 原子写settings与Manifest
→ 重启runtime
→ internal readiness
→ optional primary-origin reachability check
→ 失败恢复last-known-good settings/Manifest并重启
```

高风险变化必须提示：

```text
listen变为wildcard
primary hostname/RP ID改变
proxy trust扩大
TLS external但primary origin不可达
secret/database path改变（仍使用现有受控流程）
```

## 12. Docker 产品行为

### 12.1 配置文件

Runtime继续只接受完整严格Manifest：

```text
/data/app-state/server.toml
```

增加 CLI：

```bash
lumilio config validate --config /path/server.toml

lumilio config init \
  --profile docker-acme \
  --origin https://photos.example.com \
  --email admin@example.com \
  --output /path/server.toml

lumilio config init \
  --profile docker-external-proxy \
  --origin https://photos.example.com \
  --trusted-proxy 172.30.0.0/24 \
  --output /path/server.toml
```

CLI flags只用于一次性生成完整Manifest，不是Runtime环境覆盖。

若目标文件存在，默认拒绝覆盖；需要显式 `--force`。

`config validate` 输出：

```text
normalized primary origin
derived RP ID
listener topology
TLS owner
required host ports
security warnings
```

### 12.2 Compose profiles

交付：

```text
docker-compose.yml
    文档主入口，指向HTTPS部署选择

docker-compose.acme.yml
    Lumilio内置CertMagic
    ports 80/443
    state/storage mounts

docker-compose.proxy.yml
    外部reverse proxy
    默认不publish 6680
    dedicated proxy network

docker-compose.dev.yml
    仅开发/测试的HTTP部署
```

不要把明文 HTTP profile写成普通生产默认值。

### 12.3 ACME compose

要求：

```yaml
ports:
  - "80:8080"
  - "443:8443"
volumes:
  - lumilio_state:/data/app-state
  - ${LUMILIO_STORAGE}:/data/storage
```

healthcheck必须能在容器内使用正确SNI访问HTTPS listener，或调用一个明确的本地healthcheck命令。不得因为证书hostname不匹配而长期使用宽泛的产品请求TLS跳过验证。

CI不连接真实Let's Encrypt。使用fake issuer/local TLS fixture测试transport，真实ACME作为手工release smoke。

### 12.4 External proxy compose

要求：

- Lumilio与proxy使用专用Docker network；
- Lumilio默认仅 `expose`，不publish；
- proxy覆盖forwarded proto/host/client IP headers；
- proxy subnet与trusted CIDR一致；
- 提供Caddy、Traefik、Nginx最小示例；
- 文档警告不要信任共享给不相关容器的宽CIDR；
- healthcheck通过loopback例外。

### 12.5 Docker配置更新

P0：

```text
operator编辑/生成candidate TOML
→ lumilio config validate
→ 原子替换server.toml
→ docker restart
→ healthcheck
→ 失败由operator恢复last-known-good
```

Web admin自动改宿主机文件和重启容器不属于P0。它需要独立host control plane，列为后续工作。

## 13. 前端行为

### 13.1 启动能力查询

登录页加载时调用：

```text
GET /api/v1/auth/browser-capabilities
```

行为：

- available → 显示Passkey入口；
- disabled/non-primary/insecure → 隐藏Passkey入口；
- LAN HTTP → 显示明确警告；
- proxy-required direct access → 页面通常已被server拒绝，不显示可绕过的登录UI。

### 13.2 登录选项

输入账户后，只有：

```text
account supports passkey
AND browser capability available
```

才显示最终Passkey登录动作。

不要先显示按钮，点击后再依赖浏览器抛出`SecurityError`。

### 13.3 Settings/Runtime 页面

显示：

- primary origin；
- RP ID；
- current browser origin；
- Passkey可用性；
- network mode；
- TLS mode；
- proxy status；
- 对改变RP ID的明确解释。

### 13.4 LAN HTTP banner

在所有authenticated页面持续显示可关闭但可恢复的警告。关闭仅隐藏当前session，不代表连接被加密。

## 14. 实施阶段

### Phase 0：Inventory 与决策冻结

任务：

1. 在最新 `experimental/sqlite` 上建立工作分支。
2. 阅读 `AGENTS.md` 和当前Active exec plans。
3. 全仓库检索：
   - `ServerConfig.Port`
   - `":" + port`
   - `localhost:6680`
   - `RequestTargetOrigin`
   - `X-Forwarded`
   - `Forwarded`
   - `ClientIP`
   - `WebAuthnRP`
   - `RPOrigins`
   - Cookie `Secure/SameSite`
   - CORS
   - Desktop `ServerURL`
   - Docker port mappings。
4. 把所有独立Origin推导点写入plan Progress。
5. 记录当前Desktop与Docker smoke结果。
6. 确认P0不包含DNS challenge和web bootstrap wizard。

Exit：

- Origin/Proxy/TLS调用点清单完整；
- 所有配置模板与部署入口已定位；
- 未开始在多个handler复制新逻辑。

### Phase 1：Schema v3 与 OriginPolicy

任务：

1. Schema version提升到3。
2. 实现新的server/tls/proxy/passkey manifest结构。
3. 删除旧WebAuthn配置字段。
4. 实现Origin normalization、IDNA、RP ID推导。
5. 实现组合验证表。
6. 使用`netip.Prefix`解析trusted CIDR。
7. 建立OriginPolicy与request context。
8. 更新example、test、Desktop和Docker manifests。
9. 更新Runtime Info基础DTO。
10. 写表驱动单元测试。

必须覆盖的有效profile：

- Desktop local；
- Desktop LAN HTTP；
- Desktop same-host external HTTPS；
- Desktop remote external HTTPS；
- Docker ACME；
- Docker external proxy；
- Development。

必须覆盖的无效组合：

- acme + http primary；
- acme + IP hostname；
- acme + proxy required；
- external + proxy disabled；
- external + empty CIDR；
- off + https primary；
- passkey + HTTP LAN/IP primary；
- malformed origin；
- trusted CIDR with disabled proxy；
- listen/http_listen collision。

Exit：

- strict loader只接受v3；
- resolved policy字段唯一且可审计；
-所有模板完成更新；
- 无旧RP mode字段参与runtime。

### Phase 2：Trusted Proxy 与请求Origin统一

任务：

1. 替换当前无条件读取X-Forwarded的实现。
2. 实现immediate peer trust。
3. 实现Target Origin与Browser Origin解析。
4. 实现proxy-required middleware。
5. 把auth rate limiter和security logs切到resolved ClientIP。
6. 把Cookie/session/CSRF切到同一policy。
7. 定义健康检查例外。
8. 对malformed/conflicting proxy headers fail closed。
9. 增加结构化security logs。
10. 删除旧独立Origin helper或使其仅代理到OriginPolicy。

聚焦测试：

- untrusted peer伪造`X-Forwarded-Proto=https`无效；
- disabled模式完全忽略forwarded headers；
- trusted proxy可以恢复canonical target；
- required模式拒绝direct LAN client；
- trusted proxy forwarded target不等于primary时拒绝；
- loopback health允许；
- normal loopback login在required模式仍拒绝；
- IPv4/IPv6 CIDR；
- rate limiter得到正确client IP；
- conflicting Forwarded/X-Forwarded被拒绝。

Exit：

- 全仓库只有一个security-sensitive request origin resolver；
- cookie/passkey/session/rate-limit不再各自信任header；
- 直连不能伪装成external HTTPS。

### Phase 3：Passkey 与前端能力

任务：

1. AuthService使用静态primary origin/RP ID。
2. 删除per-request WebAuthn config。
3. 增加browser-capabilities endpoint。
4. login options合并环境availability。
5. 四个ceremony endpoint增加policy hard gate。
6. 确认WebAuthn library仍精确验证signed origin。
7. 更新DTO/OpenAPI/client。
8. 登录页隐藏不可用Passkey。
9. 添加LAN HTTP banner。
10. Runtime页面展示Origin/RP ID。

聚焦测试：

- local localhost可用；
- LAN HTTP IP不可用且reason正确；
- trusted external HTTPS可用；
- direct bypass不可用；
- Vite dev 6657可用；
- signed wrong origin被拒绝；
- passkey disabled；
- unknown account不因capabilities泄露额外状态；
- primary hostname改变时Desktop提示；
- localhost端口改变不被误报为RP ID改变。

Exit：

- 浏览器不会在已知不可用场景显示Passkey按钮；
- backend独立阻止错误ceremony；
- manifest中不再有可漂移的RP字段。

### Phase 4：Transport 与 CertMagic

任务：

1. 引入并固定CertMagic稳定版本。
2. 建立servertransport package。
3. 迁移现有HTTP listener到transport。
4. 实现off/external。
5. 实现ACME HTTPS listener。
6. 实现HTTP challenge + 308 redirect listener。
7. 使用persistent FileStorage。
8. 设置server timeouts。
9. 与runtime generation shutdown/restore顺序集成。
10. 增加证书状态runtime diagnostics。

聚焦测试：

- off mode HTTP smoke；
- external mode internal HTTP smoke；
- ACME transport使用fake issuer/local fixture；
- HTTP non-challenge重定向到primary；
- challenge handler优先；
- certificate init failure拒绝启动；
- shutdown关闭两个listener；
- restore restart不会留下被占用port；
- storage path创建/权限错误可诊断。

不要求：

- CI访问真实公网CA；
- DNS provider；
- wildcard certificate；
- on-demand TLS；
- multi-domain certificate。

Exit：

- ACME profile能在手工真实域名测试中申请证书；
- CI不依赖公网ACME；
- failure不降级HTTP；
- restore lifecycle保持SQLite ownership不变量。

### Phase 5：Desktop network modes

任务：

1. version Desktop settings。
2. 添加NetworkMode。
3. 更新manifest generator。
4. 更新`ServerURL()`为primary origin。
5. readiness使用internal health路径。
6. 实现local/LAN HTTP/external HTTPS UI。
7. LAN warning确认。
8. trusted proxy编辑与验证。
9. primary/RP ID change warning。
10. candidate→validate→atomic save→restart→rollback。

聚焦测试：

- clean Desktop defaults local；
- LAN toggle只改listen；
- LAN toggle不改primary/RP ID；
- external same-host生成loopback trust；
- remote proxy生成窄CIDR；
- Desktop拒绝ACME；
- invalid origin/CIDR不会写入settings；
- failed restart恢复last-known-good；
- launcher打开primary origin；
- second launch保持mode。

Exit：

- Desktop默认仍零配置；
- Desktop不管理证书；
- LAN和external行为与矩阵一致；
- 设置失败不会留下不可启动manifest。

### Phase 6：Docker profiles 与config CLI

任务：

1. 增加`config init`与`config validate`。
2. Runtime继续只读完整manifest。
3. 交付acme/proxy/dev compose。
4. ACME profile映射80/443。
5. proxy profile默认不publish app port。
6. 提供专用network。
7. 修正healthchecks。
8. 更新image内web root与config路径。
9. 文档说明domain/DNS/port前提。
10. 提供Caddy/Traefik/Nginx示例。

聚焦测试：

- 两个production profile生成的config通过strict loader；
- compose config可解析；
- Docker image build；
- proxy profile单容器内部health；
- app端口默认未公开；
- ACME profile两个端口正确；
- state volume包含TLS storage；
- restart后配置和证书storage保留；
- architecture guard防止重新引入origin-derived配置。

Exit：

- Docker用户必须明确选择ACME或external proxy；
- 不再把明文HTTP作为普通生产入口；
- 严格manifest原则未被env override破坏。

### Phase 7：Docs、清理与最终验证

更新：

- README；
- Docker deployment；
- Desktop network settings；
- Reverse proxy guide；
- Passkey behavior；
- Runtime manifest；
- Troubleshooting；
- ADR/architecture；
- migration note v2→v3。

删除：

- `server.port`旧路径；
- `origin-derived`；
-旧WebAuthn RP fields；
- hard-coded Desktop localhost URL；
- 无条件forwarded header trust；
- 旧Docker HTTP default文档；
- 重复Origin helpers。

运行：

```bash
make server-test
make web-test
make desktop-test
make test
make dto
```

以及新增的聚焦targets，名称按仓库约定：

```bash
make origin-policy-test
make deployment-config-test
make docker-config-test
```

不把真实ACME公网申请放入CI。把一次真实域名ACME smoke结果记录在plan Outcomes。

## 15. Definition of Done

- [x] Runtime Manifest为Schema v3。
- [x] `server.listen`替代`server.port`。
- [x] `primary_origin`是唯一canonical browser origin。
- [x] RP ID只从primary origin hostname推导。
- [x] 旧RP mode/ID/origins配置已删除。
- [x] CORS与primary origin语义分离。
- [x] 所有请求Origin判断使用一个OriginPolicy。
- [x] 不可信客户端不能伪造forwarded HTTPS。
- [x] proxy-required阻止直接绕过。
- [x] Cookie、Session、CSRF、Passkey与rate-limit共享request context。
- [x] Passkey capability endpoint完成。
- [x] LAN HTTP隐藏Passkey并显示未加密警告。
- [x] WebAuthn后端仍精确验证signed Origin与RP ID。
- [x] off、external、acme transport完成。
- [x] ACME拥有HTTP challenge/redirect listener。
- [x] CertMagic storage持久化。
- [x] ACME失败不会降级HTTP。
- [x] Desktop local/LAN/external modes完成。
- [x] Desktop默认local且零配置。
- [x] Desktop launcher使用primary origin。
- [x] Desktop配置失败可回滚。
- [x] Docker ACME与external proxy profiles完成。
- [x] external proxy profile默认不发布app端口。
- [x] config init/validate生成完整Manifest而非runtime env override。
- [x] Runtime Info展示Origin、TLS、Proxy、Passkey identity。
- [x] 开发Vite profile可用。
- [x] 聚焦安全测试通过。
- [x] Server/Web/Desktop现有相关测试通过。
- [x] README、部署与反向代理文档同步。
- [x] plan Progress、Decision Log、Surprises、Outcomes完整。
- [ ] working tree clean，提交按阶段可审查。

最后一项保留未勾选：本轮没有得到提交或清理工作树的授权；所有改动仍作为
一个可检查的工作树交付。

## 16. 不阻塞本次Goal的后续项

- DNS-01 provider插件；
- wildcard certificates；
-多个primary origins；
- related-origin WebAuthn；
-自动公网DNS检查；
-网页bootstrap server；
- Web admin修改宿主机Manifest并重启Docker；
-完整五场景浏览器E2E矩阵；
-所有反向代理产品的真实集成测试；
-IPv6 LAN自动发现；
- credential enrollment RP metadata；
- multi-hop proxy trust chain；
-动态证书on-demand TLS。

## 17. Agent执行纪律

- 本plan是living document；每阶段更新Progress、Decision Log、Surprises和验证命令。
- 开始前先inventory，不在handler中直接补临时Origin判断。
- 安全逻辑只允许一个resolved policy source。
- 遇到现有命名/目录与plan不一致时，选择符合项目结构的最小改动并记录。
- 不通过信任所有proxy、接受多个Origin或关闭cookie security来让测试变绿。
- 不为ACME测试连接真实生产CA。
- 不把环境变量重新变成Runtime Manifest override。
- 不以“按钮隐藏了”代替后端ceremony验证。
- 不以“反向代理通常会覆盖header”为由跳过peer trust。
- 每阶段形成小型可审查commit。
- P0完成后停止扩展DNS challenge、multi-hop proxy等后续能力。
- 最终报告列出：
  - Schema变化；
  - 五个profile；
  - OriginPolicy边界；
  - Proxy防绕过策略；
  - CertMagic生命周期；
  - Desktop UX；
  - Docker配置流程；
  - 手工ACME smoke；
  - 已知限制。

## Progress

- [x] Phase 0：Inventory
  - 原独立Origin推导点：`api/origin.go`（CORS/session）、`auth_session.go`
    （cookie）、`auth_passkeys.go`（per-request WebAuthn）、Gin `ClientIP`
    （rate-limit/log）、Desktop硬编码`ServerURL()`、Docker端口/healthcheck。
  - 模板/入口：server example/container、Desktop embedded template、E2E
    manifest、root/release/E2E Compose、Dockerfile、standalone CLI。
  - 基线：`make server-test`、`make desktop-test`、Compose config通过；P0
    明确不含DNS challenge或未认证Web bootstrap。
- [x] Phase 1：Schema v3与OriginPolicy
- [x] Phase 2：Trusted Proxy与请求Origin
- [x] Phase 3：Passkey与前端能力
- [x] Phase 4：Transport与CertMagic（真实公网ACME smoke仍需release域名）
- [x] Phase 5：Desktop network modes
- [x] Phase 6：Docker profiles与config CLI
- [x] Phase 7：Docs、清理与最终验证

## Decision Log

| Date | Decision | Reason | Consequence |
|---|---|---|---|
| 2026-07-26 | `primary_origin`是唯一canonical browser/WebAuthn origin | 消除RP配置漂移 | 删除origin-derived与手写RP字段 |
| 2026-07-26 | RP ID取primary hostname exact value | 最小credential scope | 不自动扩大到父域 |
| 2026-07-26 | 删除`tls.domain` | 避免双域名source of truth | ACME证书名由primary origin推导 |
| 2026-07-26 | ACME增加独立`http_listen` | 支持HTTP-01和redirect | Docker映射80/443到非特权内部端口 |
| 2026-07-26 | proxy trust以immediate peer CIDR为边界 | 防止forwarded header伪造 | direct bypass在required模式被拒绝 |
| 2026-07-26 | Docker配置由一次性CLI生成完整Manifest | 保留严格runtime配置 | P0不做未认证bootstrap server |
| 2026-07-26 | DNS challenge不进入P0 | provider credential与插件面过大 | 无公网80/443者使用external proxy |
| 2026-07-26 | Development是独立profile | Vite/API跨Origin是有意行为 | 不污染production Origin规则 |
| 2026-07-26 | 固定CertMagic `v0.25.3`并使用独立cache/runtime | 避免修改package-global默认值，允许generation shutdown | ACME失败关闭双listener且不降级HTTP |
| 2026-07-26 | Desktop网络设置schema version为1 | 旧设置无网络字段时可确定迁移 | clean install与升级用户都落到local |
| 2026-07-26 | Docker生产镜像不内置可启动的HTTP manifest | 强制operator明确选择TLS owner | config写入持久化app-state后才能启动 |
| 2026-07-26 | ACME容器health使用配置感知CLI | 保留证书hostname验证，避免`-k` | healthcheck以primary SNI连接内部listener |

## Surprises & Discoveries

- CertMagic最新pkg.go.dev页面为`v0.25.3`，GitHub releases搜索结果仍把
  `v0.25.2`标成Latest；实现固定实际可获取的`v0.25.3`并以官方API为准。
- macOS sandbox禁止Vitest/Vite绑定loopback（`EPERM`）；同一完整Web gate
  在允许本地listener后通过，不是测试断言失败。
- 直接`go test`会遗漏Makefile的libraw CGO allowlist与`sqlite_fts5`tag；
  正式证据必须使用`make server-test`/`make desktop-test`。
- 本机Docker Compose v5可执行文件支持`docker-compose -f ...`，但
  `docker compose -f ...`把`-f`错误交给顶层Docker；四份Compose文件使用
  standalone兼容入口完成静态解析。
- OrbStack完整runtime image构建通过；镜像内ACME config init + strict
  validate smoke通过，证书公网申请没有可用真实域名，因此未伪造结果。
- OrbStack external-proxy smoke使用专用`172.31.77.0/24`网络：可信peer
  携带canonical forwarded target返回200；第二个不可信网络直连返回403；
  `HostConfig.PortBindings={}`。临时容器与两个测试网络随后已删除。

## Outcomes & Retrospective

实现结果：

- 最终Schema v3结构：`server.listen`、`server.primary_origin`、
  `[server.tls]`、`[server.proxy]`、`[auth.passkey]`。
- 删除的旧WebAuthn字段：RP mode、手写RP ID、手写allowed origins。
- OriginPolicy所在package：`server/internal/httporigin`。
- proxy header支持范围：RFC `Forwarded`或完整X-Forwarded
  proto/host，可选XFF；两族冲突、列表歧义、非可信immediate peer均拒绝。
- CertMagic版本：`v0.25.3`，FileStorage位于显式持久化storage path。
- Desktop settings migration：settings v1；缺失字段确定迁移到local。
- Docker profile文件：`docker-compose.acme.yml`、
  `docker-compose.proxy.yml`、`docker-compose.dev.yml`。
- 真实ACME smoke：未执行；需要真实域名、DNS和公网80/443。
- Passkey场景验证：local/Vite/external available；LAN/direct bypass/
  disabled fail closed；静态RP origins与challenge RP ID精确断言。
- 自动化验证：
  - `make server-test`通过；
  - `make web-test`通过（61 files passed / 2 skipped，249 tests passed /
    6 skipped）；
  - `make desktop-test`通过；
  - `make dto`完成并更新OpenAPI、TypeScript DTO与Redoc；
  - `pnpm build`在`site/`通过；
  - root、ACME、proxy、dev、release Compose静态解析通过；
  - OrbStack最终runtime image `lumilio-network-test`构建通过；
  - external-proxy容器smoke验证可信peer 200、非可信direct peer 403且
    app无host port binding。
- 已知限制：P0不含DNS-01、wildcard、multi-domain、related-origin或自动DNS检查。
- 是否建议合并到main：实现与自动化门禁完成；真实ACME release smoke后建议合并。
