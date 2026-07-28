# Lumilio Photos SQLite-only 迁移执行计划

> 仓库路径：`site/docs/internal/exec-plans/completed/sqlite.md`
> 工作分支：`experimental/sqlite`
> 状态：Completed
> 迁移策略：允许 destructive migration；没有 PostgreSQL 数据导入义务
> 停止条件：完成本文“核心 Definition of Done”。标记为 Hardening / Deferred 的规模 benchmark、持续 stress、全量 fault injection、完整跨平台运行时 E2E 不阻塞 `experimental/sqlite` goal 完成。

## 1. Goal、边界与架构决策

### 1.1 Goal

将 Lumilio Photos 从“Go 服务 + 独立 PostgreSQL 进程 + River/pgvector + SPA”迁移为：

```text
一个 Lumilio 应用进程
├── Go API、业务逻辑与媒体处理编排
├── SQLite 应用数据库
├── River Queue SQLite tables（同一数据库）
├── FTS5 与本地向量检索
├── 可选的、可重建的向量派生索引
└── SPA 静态资源或 Vite 开发服务器

外部工具
├── ffmpeg / ffprobe
├── exiftool
├── libvips / libraw
└── 可选 Lumen 节点
```

Desktop 最终不再启动、管理或打包 PostgreSQL。Docker 最终不再包含 `db` service、PostgreSQL image、数据库密码 secret、数据库端口与 PostgreSQL client。开发环境启动不再依赖 Docker 数据库。

SQLite 是 `experimental/sqlite` 分支唯一支持的数据库。迁移完成后，业务代码、配置、测试、打包和文档中均不保留运行时 PostgreSQL 路径。

### 1.2 产品级成功状态

1. 用户下载 Desktop 后，首次启动只需要创建本地目录和一个 SQLite 文件，不需要初始化数据库集群、等待数据库 readiness 或管理数据库子进程。
2. Docker 默认只需要 Lumilio 应用容器及两个持久化挂载：
   - 应用状态：SQLite、密钥、日志、云会话与派生索引；
   - 媒体存储：原图、缩略图、转码文件和 repository 数据。
3. `server` release artifact 本身能够同时提供 API 与已构建 SPA。反向代理或 Caddy 可以存在，但只能是可选部署层，不能成为运行 Lumilio 的必要组件。
4. River 与业务数据共享同一 SQLite 数据库，关键业务状态和任务入队能够使用同一个数据库事务提交。
5. 备份得到一个一致的 SQLite snapshot；恢复通过完整 runtime restart 完成，不在仍被旧 service/handler 持有的连接下热替换数据库。
6. 当前主要能力保持可用：首次设置、认证、repository、导入、扫描、元数据、缩略图、转码、相册、人物、位置、重复项、搜索、语义向量、Agent、云导入、队列管理、备份恢复。
7. 开发、测试、Docker 与 Desktop 都不需要安装或启动 PostgreSQL。

### 1.3 明确不做

- 不实现 PostgreSQL 与 SQLite 双后端。
- 不建立以“未来可能恢复 PostgreSQL”为目的的通用数据库抽象层。
- 不实现 PostgreSQL 数据文件或现有数据库到 SQLite 的转换工具。
- 不保证旧 migration 能在 SQLite 上逐条重放；使用新的 squashed SQLite baseline。
- 不在本次迁移中实现多节点写入、共享数据库、远程 SQL 接入或高可用数据库。
- 不把活动中的 WAL 数据库放到 SMB、NFS、云盘同步目录或其他网络文件系统。
- 不以 SQLite alpha ANN 扩展作为完成迁移的前置条件。
- 不引入独立 Qdrant、Milvus、Weaviate 等服务来代替 pgvector。
- 不为“一个物理文件”牺牲可靠性；可重建的派生索引允许作为 sidecar。
- 不同时重写前端产品体验。除首次设置中 PostgreSQL 特有步骤消失外，尽量保持 API 与 UI 行为稳定。
- 不引入 SQLCipher。数据库文件安全依赖本机账户、目录权限、Desktop sandbox/OS 权限和现有认证密钥策略。

### 1.4 不变量

- 原始媒体文件不是数据库事务的一部分，数据库不得假装可以对文件系统提供 ACID。
- ffmpeg、ExifTool、libvips、文件复制、哈希、模型推理和网络请求不得运行在 SQLite 写事务中。
- 所有写事务应当短小、可解释；目标是正常情况下个位数毫秒，超过 25 ms 必须记录结构化 warning。
- 所有 SQLite 写入与 River 写入由一个 writer pool 协调。
- River schema 与 Lumilio schema 位于同一数据库文件。
- 一个 Library 数据库可以关联多个 Repository；不得为每个媒体 Repository 创建独立数据库。
- 活动数据库默认位于 machine-local app state。可迁移性通过安全 snapshot、restore 和“移动 Library”语义实现，而不是运行时直接复制 `.sqlite3`。
- 数据库、`-wal`、`-shm`、密钥、日志、云会话和备份目录必须与媒体 repository 语义分离。
- 派生索引损坏、缺失或版本不匹配时必须可以从 SQLite 中的权威数据重建。
- 配置继续使用完整、严格、schema-versioned TOML manifest；不重新引入隐式默认、配置搜索或普通环境变量覆盖。
- 业务层不得继续暴露 `pgx`、`pgtype`、`pgconn`、`pgvector` 或 PostgreSQL error code。
- 不能用“捕获 `SQLITE_BUSY` 后无限重试”掩盖长事务和错误的连接拓扑。
- 不修改 River 自己的 migration 或 table schema。

### 1.5 核心架构决策

#### AD-1：SQLite-only，直接迁移

删除 PostgreSQL implementation，而不是在现有 `db.DB` 下增加第二个 backend。保留的 `db` package 是 SQLite 运行时边界，不是多数据库接口。

允许在迁移阶段对 schema、生成代码、构造函数和 service 依赖做破坏性修改。API DTO 与产品行为应尽量保持稳定，但数据库内部兼容不是目标。

#### AD-2：数据库位置代表 Library catalog，不代表单个 Repository

默认布局：

```text
app-state/
├── library.sqlite3
├── library.sqlite3-wal       # 运行时临时文件
├── library.sqlite3-shm       # 运行时临时文件
├── derived/
│   └── vector/               # 仅在需要 ANN sidecar 时存在
├── auth/
├── cloud/
└── logs/

backups/
├── <timestamp>-library.sqlite3
└── <timestamp>-manifest.json

storage/
├── .lumilioroot
└── repositories/...
```

建议默认路径：

```text
Development: <repo>/.local/lumilio/library.sqlite3
Docker:      /data/app-state/library.sqlite3
Desktop:     <OS application-data>/Lumilio Photos/library.sqlite3
Test:        t.TempDir()/library.sqlite3
```

`database.path` 必须是显式 manifest 字段，并且：

- 解析为绝对、清理后的路径；
- production 中不能是 `:memory:`；
- 不能位于 `storage.path`、任一 repository root 或 backups 目录内；
- parent directory 由启动流程以私有权限创建；
- Desktop parent 使用仅当前用户可访问的权限，数据库文件使用 `0600` 等价权限；
- 路径可被用户通过受控的 snapshot/move 操作迁移，但活动 WAL 数据库不支持直接热复制。

#### AD-3：SQLite driver 与扩展策略

首选目标：

- `database/sql`
- `github.com/mattn/go-sqlite3`
- 静态注册的 `sqlite-vec`
- FTS5 build tag
- 不依赖系统安装的 SQLite CLI、动态扩展文件或运行时 `load_extension`

选择原因：现有 server/desktop 已经具有 CGO 和 native media dependencies；`mattn/go-sqlite3` 能与静态 sqlite-vec 绑定形成单个应用二进制。

Phase 1 必须先完成最小兼容性 spike，验证：

1. River SQLite migration、启动、入队、执行一个 job 和 `InsertTx` commit/rollback；
2. `mattn/go-sqlite3` 下单 writer connection；
3. sqlite-vec 在实际使用连接上可用，或能够启用 AD-3 fallback；
4. FTS5 可用；
5. 当前开发平台的 server 与 Desktop CGO 链能够构建；
6. 当前 Docker target 能够构建。

Linux amd64/arm64、macOS arm64、Windows amd64 的完整构建与运行时矩阵属于 best effort；应保持现有 CI/packaging 配置合理，但不阻塞 experimental goal。

若出现无法在合理范围内修复的实际兼容问题，整个分支统一切换到 `modernc.org/sqlite`，向量暂用 SQLite BLOB + Go 精确检索或可重建 sidecar。不得同时保留两套 driver，也不得为了保留 sqlite-vec 强行动态加载不可靠扩展。最终仓库只能有一个 SQLite driver。

选择依赖时固定一个 stable、非 alpha 版本。任何 alpha ANN 实现不得成为迁移完成条件。

#### AD-4：连接拓扑

初始目标是一个 writer `*sql.DB`：

```go
type DB struct {
    SQL     *sql.DB
    Queries *repo.Queries
    Path    string
}

func Open(ctx context.Context, cfg config.DatabaseConfig) (*DB, error)
func (d *DB) WithTx(
    ctx context.Context,
    fn func(tx *sql.Tx, q *repo.Queries) error,
) error
func (d *DB) Close(ctx context.Context) error
```

writer 设置：

```text
MaxOpenConns = 1
MaxIdleConns = 1
ConnMaxLifetime = 0
ConnMaxIdleTime = 0
```

River、业务写入、migration 和一般查询先共享这个 pool。这样可以先得到最小、可验证的单写者系统。

只有在基准测试证明长读取阻塞 UI 或队列后，才增加一个明确只读的 bounded reader pool。reader pool 只能交给经过审计的长读取路径，例如语义检索、人物最近邻和大批量导出；不得把它包装成新的全仓库 repository abstraction，也不得让写 query 混入 reader pool。

固定 SQLite 策略由代码应用，不暴露为普通产品配置：

```text
foreign_keys = ON
journal_mode = WAL
synchronous = NORMAL
busy_timeout = 5000 ms
temp_store = MEMORY
wal_autocheckpoint = 1000 pages
```

要求：

- 启动时读取并验证实际 pragma 值；
- extension 和 pragma 对 migration connection 与 runtime connection 一致；
- 不使用连接 lifetime 回收导致新连接丢失 pragma/extension；
- graceful shutdown 在 River 和 HTTP 停止后执行 checkpoint 与 `PRAGMA optimize`；
- unclean shutdown 依赖 WAL recovery，随后执行快速完整性检查；
- `SQLITE_BUSY` 发生时输出可定位的 operation、transaction duration 和 caller，不进行无限重试。

#### AD-5：配置 schema v2

将 manifest `schema_version` 提升到 `2`。

数据库配置收敛为：

```toml
[database]
path = "/data/app-state/library.sqlite3"
```

删除：

```text
database.host
database.port
database.user
database.name
database.ssl
database.bootstrap_password_file
database.rotated_password_file
database.tools_bin_dir
```

数据库内部 pragma 不进入 manifest。媒体工具继续位于 `[tools]`。

同步修改：

- `server/config/config.go`
- 所有 example/local/container manifest
- Desktop `server.template.toml`
- Desktop manifest generator
- Docker image manifest
- test fixtures
- setup/dev scripts
- runtime info DTO 与文档

新的验证要求：

- `database.path` 非空且可解析；
- production/test 语义明确；
- database path 位于 machine state，而非 media storage；
- database file、backup destination、auth secret、cloud state 和 log path 不发生危险重叠；
- 不再读取数据库密码 secret；
- first-run readiness 不再依赖数据库 role password rotation。

#### AD-6：schema 与 Go 类型

使用一份新的 SQLite baseline migration，覆盖当前所有 PostgreSQL migrations 的最终 schema。先制作“旧 migration/table/query → 新 baseline”清单，确认每个当前能力都被保留，再删除旧 migrations。

推荐表示：

| 领域值 | SQLite 表示 | Go 表示 |
|---|---|---|
| UUID | canonical lowercase `TEXT` | `google/uuid.UUID`；nullable 使用 `uuid.NullUUID` 或项目自定义 nullable type |
| 时间 | UTC Unix microseconds `INTEGER` | `dbtypes.Timestamp`，实现 `sql.Scanner` / `driver.Valuer` |
| bool | `INTEGER CHECK(value IN (0,1))` | `bool` |
| enum | `TEXT` + `CHECK` | 当前 domain enum |
| JSON | `TEXT` + `CHECK(json_valid(value))` | `json.RawMessage` 或现有 typed JSON wrapper |
| embedding | little-endian float BLOB | `[]float32` / typed vector wrapper |
| money/精确数值 | integer smallest unit | domain integer |
| nullable scalar | `NULL` | `sql.Null*`、pointer 或项目 typed nullable |

要求：

- 普通应用表优先使用 `STRICT`；
- virtual tables 依照 SQLite 扩展要求，不强求 `STRICT`；
- 所有 ID 由 Go 生成，不依赖数据库 UUID function；
- 所有时间由 Go 的 UTC clock 生成；数据库 trigger 只用于简单且确定的维护；
- 非必要不使用 `AUTOINCREMENT`；
- foreign key、unique、partial index 和 check constraint 保留；
- 每个 nullable/zero-value 行为有测试；
- schema 中不得混用多种时间或 UUID 表示；
- River schema 由 River migration 管理，不复制到应用 baseline。

PostgreSQL 特性替换：

| PostgreSQL | SQLite |
|---|---|
| enum type | `TEXT CHECK(...)` |
| `JSONB` | JSON text + JSON1 |
| `TIMESTAMPTZ` | Unix microseconds |
| `uuid_generate_v4()` | Go `uuid.New()` |
| `pgcrypto` | Go crypto |
| `tsvector` / GIN | FTS5 external-content table |
| `pg_trgm` | FTS5 trigram tokenizer，或显式 normalized search column |
| `vector(n)` | authoritative BLOB + sqlite-vec table |
| HNSW index | sqlite-vec exact KNN；必要时 derived ANN sidecar |
| PL/pgSQL trigger | SQLite trigger 或 Go service logic |
| `ILIKE` | normalized field、`COLLATE NOCASE` 或 FTS5 |
| `jsonb_agg` | `json_group_array/json_object` |
| `DISTINCT ON` | window function |
| `ANY(array)` | generated `IN` query、temp table 或 JSON1 |
| PostgreSQL error code | SQLite-specific typed error helper |
| advisory lock / `SKIP LOCKED` | 单进程 mutex、短事务状态机；River 自己负责 queue claims |

`sqlc.yaml` 改为 SQLite + `database/sql`。生成文件只能由 `sqlc generate` 产生。保留现有 query name 时优先保留，以减少业务层改动；SQL 语义不正确时重写 query，不在 Go 中做大量 N+1 兼容拼接。

#### AD-7：River 与业务数据库共用事务

使用 `riverdriver/riversqlite`。River migrations 在应用 migrations 后、queue client 创建前执行。

目标：

```text
Open SQLite
→ apply/verify pragmas and extensions
→ run Lumilio migrations
→ run River migrations
→ construct sqlc queries
→ construct River client with same writer *sql.DB
→ register workers
→ start River
```

所有 queue 名称、worker 数量、periodic job、重试语义和管理 API 保持当前产品行为，除非 SQLite 单写者测量要求降低默认并发。媒体计算 worker 仍可高并发，因为长工作发生在数据库事务外。

重点修复 SourceMaterializer：

##### staging upload

```text
1. 验证、哈希、去重检查                         无写事务
2. 短事务创建 prepared/staged asset             COMMIT
3. staging → final/inbox 文件提交                无写事务
4. 一个短事务：
   - 写入最终 storage path
   - 设置 processing/task 状态
   - InsertTx(metadata)
   - InsertTx(thumbnail，按类型)
   - InsertTx(transcode，按类型)
   COMMIT
5. 返回成功
```

##### in-place repository scan

文件已经存在时，资产记录、最终路径、初始状态和所有 pipeline jobs 在一个短事务中写入。

##### crash recovery

启动或定期 reconcile 必须处理：

- asset 是 prepared、staging 文件仍存在；
- asset 是 prepared、final 文件已存在；
- asset 是 prepared、两个文件都不存在；
- final 文件存在但 DB transaction 未提交；
- DB 与 River transaction 已提交但 HTTP response 未返回；
- job 执行中进程被 kill；
- 重复 scan/upload 导致相同内容并发发现。

恢复必须幂等。不得通过删除未知原始文件来“修复”状态。无法安全判断时标记为可见的 failed/recoverable 状态并保留文件。

#### AD-8：搜索与向量

##### 文本搜索

- 使用 FTS5 替换 `tsvector`/GIN；
- 对需要 substring 的名称、地点或 metadata 路径评估 FTS5 trigram tokenizer；
- 使用 external-content table + trigger，或显式 service dual-write；
- 提供 rebuild 命令/函数；
- 结果排序与当前 API contract 保持稳定，必要差异记录在 plan 的 Decision Log。

##### 权威 embedding

普通表保存完整权威记录：

```text
search_embeddings
- embedding_id
- asset_id
- model
- dimensions
- frame_time / segment identity
- vector_blob
- created_at
- updated_at
- generation/version metadata

face_embeddings
- face_id / embedding_id
- asset_id
- model
- dimensions
- vector_blob
- metadata
```

sqlite-vec virtual table只作为可重建查询结构。普通表与 vec table 的新增、更新和删除必须在同一 SQLite transaction 内执行，或者通过明确的 generation/rebuild protocol 保证最终一致。

使用稳定 exact KNN 先恢复正确性。不要为了迁移采用 alpha DiskANN/IVF。

##### 性能观察与后续扩展（本次迁移非阻塞）

本次 experimental 迁移优先验证正确性、部署简化和真实开发数据可用性，不要求先构造 100k/1M 数据集，也不要求建立严格的 PostgreSQL 对照 benchmark。

本次必须完成：

- 使用现有开发库、现有 fixture，或一个容易生成的小型数据集验证 768D image、video frame 与 512D face vector 的插入、删除、过滤和 top-k 正确性；
- 至少记录一次代表性 exact KNN 查询耗时、数据库体积和 rebuild 行为，结果只用于发现明显不可用的实现，不设置硬性 p95 门槛；
- 使用 brute-force reference 或可解释的小样本断言验证 top-k 结果，而不是只验证“查询返回了数据”；
- sqlite-vec 不可用时按 AD-3 fallback 完成迁移，不让向量扩展阻塞数据库切换。

Hardening backlog，不阻塞本 goal：

- 100k assets、视频 frame 扩张和 1M 个 768D rows 的规模测试；
- cold/warm cache、top-10/top-50/top-100、float32/int8 的系统比较；
- 与 PostgreSQL/pgvector 的严格同机对照；
- 基于规模结果决定 mmap ANN sidecar。

未来若增加 sidecar，它仍不得成为权威数据，必须带 model/schema/generation fingerprint，并可从 SQLite authoritative embeddings 重建。sidecar 不可用时系统必须降级为正确的 exact search，或明确表示索引正在重建。

#### AD-9：备份与恢复

删除 `pg_dump`、`pg_restore`、PostgreSQL client 与数据库连接凭据。

备份使用 SQLite Online Backup API 或等价的一致 snapshot 机制。不得复制运行中的主 `.sqlite3` 文件。

每次备份输出：

```text
<timestamp>-library.sqlite3
<timestamp>-manifest.json
```

manifest 至少包含：

- app version；
- config schema version；
- application migration version；
- River migration version；
- SQLite version；
- sqlite-vec version（如启用）；
- 创建时间；
- database size；
- SHA-256；
- quick/integrity check 结果；
- library identity。

备份流程：

```text
create temp snapshot
→ close/finalize snapshot
→ quick_check + foreign_key_check
→ compute checksum
→ fsync where supported
→ atomic rename
→ apply retention
```

恢复不得在旧 handlers/services 仍持有旧 `*sql.DB` 时原位替换。将 `app.Run` 改造成可重建 runtime generation：

```text
host lifecycle
└── generation loop
    ├── open DB
    ├── migrations
    ├── wire services/River/router
    ├── serve
    ├── receive normal shutdown OR restore-restart request
    ├── drain HTTP and River
    ├── close DB
    ├── apply staged restore atomically
    └── start next generation
```

恢复流程：

1. 上传或选择 snapshot；
2. 在独立连接中校验 checksum、manifest、SQLite header、quick check、foreign key 和 schema compatibility；
3. 生成当前 DB restore point；
4. 写 pending-restore marker；
5. 请求 runtime generation restart；
6. drain HTTP/queue，关闭所有 DB handles；
7. 原子替换主 DB，保留旧 DB 直到新 generation 健康；
8. 运行 migrations、完整性检查和应用 health verification；
9. 成功后删除 marker并保留策略内 restore point；
10. 失败时自动 rollback 到旧 DB，并再次重建 runtime；
11. 恢复期间 API 进入 maintenance mode，不能接受新 mutation。

River tables包含在 snapshot 内。恢复后 periodic jobs 必须重新注册，River 对 interrupted/running jobs 的恢复语义必须通过测试。

#### AD-10：Docker 目标

删除：

- `server/db.Dockerfile`；
- `db` service；
- `db_data` volume；
- `db_bootstrap_password` secret；
- `depends_on: db`；
- PostgreSQL healthcheck；
- PostgreSQL host port；
- `docker-compose.release.dbport.yml`；
- server runtime image 中的 PostgreSQL apt repository/client；
- 与 PostgreSQL bootstrap secret 相关的 helper 与文档。

默认 mounts：

```yaml
volumes:
  - ${LUMILIO_STORAGE}:/data/storage
  - ${LUMILIO_STATE}:/data/app-state
```

数据库：

```text
/data/app-state/library.sqlite3
```

release server image应构建并包含 SPA dist，设置 `server.web_root`，使以下方式成立：

```text
docker run/compose 一个 Lumilio container
→ 同一端口提供 SPA + API
```

为保持当前 UI 入口，release compose 可将 `${WEB_HTTP_PORT:-6657}` 映射到 server `6680`。内置 HTTPS/Caddy 若继续保留，移动到 optional profile 或单独的 proxy compose，不得成为默认必须项。

要求：

- `docker-compose.yml`、release、E2E、CI compose 全部改为 SQLite-only，不得再引用 PostgreSQL service；
- 当前开发架构的 server image 必须构建；其他 Linux 架构保持 Dockerfile/CI 可表达并做 best-effort compile；
- state mount 权限错误要给出明确诊断；
- healthcheck 在 migration 与 River ready 后才成功；
- 当前架构完成一次 container restart 后 state 保留与数据库 reopen smoke；完整 WAL/River/reconcile 故障矩阵延后；
- 不在媒体 storage mount 中创建 active database；
- Docker docs说明备份必须使用应用 snapshot，而不是复制运行中的文件。

#### AD-11：Desktop 目标

保留 Desktop supervisor 作为：

- single-instance lock；
- OS paths；
- manifest materialization；
- in-process `server/app.Run` lifecycle；
- browser opening；
- tray/menu；
- Lumen 下载和 supervision；
- graceful quit。

删除其 PostgreSQL职责：

- `postgres.go` 与对应 tests；
- initdb/pg_ctl/pg_isready/createdb/postgres lifecycle；
- PG socket、port、data dir、password file 和版本检查；
- bundled PostgreSQL resources、licenses、下载和 packaging scripts；
- `LUMILIO_PG_BIN_DIR`；
- PostgreSQL quarantine/permission handling；
- PostgreSQL start/stop status UI。

`Paths` 增加或保留清晰的：

```text
DatabasePath
AppStateDir
BackupsDir
LogsDir
CloudStateDir
WebRoot
MediaRoot
```

Desktop 首次启动：

```text
resolve private app paths
→ create directories
→ materialize schema v2 manifest
→ app.Run in-process
→ SQLite creates/migrates file
→ health ready
→ open browser
```

P0 必须验证：

- 当前开发平台的 clean first launch 与 second launch；
- 当前平台 final bundle/package 不包含 PostgreSQL 文件；
- 一个 database path 或 permission failure 能给出明确错误；
- 一次 SQLite reopen，必要时通过代表性的 force-quit 或等价非正常退出 smoke 完成；
- backup/restore 所需的 runtime restart 路径至少在当前平台或 standalone 中验证一次。

Best effort / Hardening：

- app data path带空格和非 ASCII；
- macOS arm64 与 Windows amd64 双平台运行时矩阵；
- update 后多版本 schema migration；
- disk full/readonly 的完整平台矩阵；
- force quit/kill 的重复压力测试。

### 1.6 需要优先清除的当前耦合点

Agent 在修改前应完成全仓库 inventory，并把结果记录到本文 Progress：

- `server/sqlc.yaml` 的 PostgreSQL engine 与 pgx package；
- `server/internal/db/db.go` 的 pool、password self-heal 和 pg errors；
- `server/internal/db/migration.go` 的 Postgres migration driver 与 `riverpgxv5`；
- `server/app/app.go` 中传播到 services/handlers 的 `*pgxpool.Pool`；
- `server/internal/queue/queue_setup.go`；
- `server/internal/sourcing/materializer.go` 的非事务性 pipeline enqueue；
- `server/internal/db/backup/` 与 backup service；
- setup/bootstrap 中数据库 password rotation；
- queue handler 对 pgx pool 的依赖；
- face、semantic、asset、location、stack、duplicate、auth、user、agent 等 service 的直接 raw pgx；
- migrations 中 enum、JSONB、tsvector、trigram、PL/pgSQL、vector/HNSW；
- Docker Compose、Dockerfile、Makefile、Dev Container、GitHub Actions；
- Desktop supervisor、resources、manifest template 和 release packaging；
- README、install docs、architecture docs、troubleshooting docs。

完成 inventory 后建立一份 machine-readable 或 Markdown parity checklist，列出每个 PostgreSQL table、index、trigger、query 和 feature 的 SQLite 去向。不得仅以“服务能启动”判断 schema 已迁完。

---

## 2. 分阶段执行

### Phase 0：建立分支护栏与迁移清单

任务：

1. 确认当前分支为 `experimental/sqlite`，不得在 `main` 直接实施。
2. 阅读 `AGENTS.md` 规定的架构、后端、测试与 plan 文档。
3. 运行当前环境中容易执行的快速 build/test，记录与迁移无关的既有失败；不要求为 PostgreSQL 旧实现补齐测试环境。
4. 若现成命令可以直接得到，则记录 startup、idle memory、binary/image size 或一条代表性 vector query；不得为建立严格基线延迟迁移。
5. 全仓库搜索 PostgreSQL 耦合并写入 parity checklist。
6. 将 destructive migration 决策写入当前 plan，注明旧开发数据必须 reset。
7. 为后续提交建立小步 commit strategy。

Exit criteria：

- 已知的既有 build/test 失败被记录，后续能够区分迁移回归与原有问题；
- migration/query/service/deployment 耦合清单足以指导迁移；
- destructive reset 路径明确；
- 未创建双后端 interface。

### Phase 1：SQLite/River/vector 最小技术 spike

只写足以解除架构风险的隔离验证，不先建立完整集成测试矩阵。

必须验证：

- driver open/close 与 fixed pragmas；
- application migration 和 River SQLite migration；
- `STRICT`、JSON1、FTS5；
- River start/stop、enqueue、执行一个 job 与一次 retry；
- app transaction + `InsertTx` 的 commit/rollback；
- sqlite-vec static registration及小样本 768D/512D insert/delete/query，或 AD-3 fallback；
- `sqlc` SQLite type mapping；
- 当前开发平台的 server 与 Desktop module build；
- 当前 Docker target 能够编译。

Best effort / Deferred：

- process kill 后所有 River 状态的完整恢复矩阵；
- cancel、periodic、所有 queue admin 行为的隔离测试；
- Linux/macOS/Windows 全部运行时 smoke；
- Docker multi-arch 运行时验证。

做出并记录唯一 driver 决策。删除未采用的 spike implementation 和依赖。

Exit criteria：

- driver、River transactional enqueue、sqlc、FTS5 和 vector/fallback 的关键路径有聚焦测试；
- 选定一个 driver；
- 不存在“先支持两种，之后再决定”的代码。

### Phase 2：配置、DB runtime 与 migration runner

任务：

1. manifest 升级 schema v2；
2. 将 database config 改为 path；
3. 实现 SQLite `db.Open/Close/WithTx`；
4. 统一 pragma/extension registration；
5. 保留 embedded migrations，替换 database migration driver；
6. 接入 River migration runner；
7. 实现 startup preflight、quick check、foreign key check；
8. 实现 graceful checkpoint/optimize；
9. 重写 config tests、DB tests、migration tests；
10. 移除数据库 password/self-heal logic；
11. 调整日志，输出 SQLite/extension version与经过安全处理的 DB location。

Exit criteria：

- 空目录可创建数据库并完成两类 migrations；
- 重启幂等；
- corrupt/readonly/permission error可诊断；
- config v1 被明确拒绝；
- PostgreSQL连接代码已从 `internal/db` 消失。

### Phase 3：fresh SQLite baseline、sqlc 与 domain types

任务：

1. 根据 parity checklist 写新的 squashed baseline；
2. 建立统一 UUID、timestamp、nullable、JSON 和 vector types；
3. 将 `sqlc.yaml` 改为 SQLite；
4. 逐目录改写 queries；
5. 生成 repo code；
6. 实现 SQLite constraint/error helpers；
7. 为关键约束与代表性表添加 schema tests，不要求为每张表复制同构测试；
8. 为 UUID/time/JSON roundtrip、foreign key 和至少一个关键 unique/partial-index path 写代表性测试；
9. 实现 FTS5 schema；
10. 删除旧 PostgreSQL migrations。

执行顺序建议：

```text
identity/settings
→ repositories/roots
→ assets/derivatives
→ albums/tags/relations
→ people/faces
→ locations
→ duplicates/stacks
→ cloud/backups/security
→ search embeddings/video
→ agent runtime
```

Exit criteria：

- baseline 创建完整 schema；
- `sqlc generate` clean；
- schema parity checklist全部有明确落点；
- migration idempotency与fresh boot test通过；
- 旧 Postgres migrations 已删除，而非保留为“参考运行时”。

### Phase 4：业务层去 pgx 化

任务：

1. 将 service/handler/worker 构造函数中的 `*pgxpool.Pool` 改为所需的具体 SQLite dependency：
   - `*db.DB`；
   - `*sql.DB`；
   - `*repo.Queries`；
   - `*sql.Tx`；
   - 或窄的 domain collaborator。
2. 用 `google/uuid`、`dbtypes.Timestamp`、`json.RawMessage` 等替换 pg types。
3. 将 raw pgx SQL迁入 sqlc query 或窄的 SQLite helper。
4. 替换 PostgreSQL error code分支。
5. 删除数据库 role/password setup。
6. 重新定义 bootstrap gates：系统设置、owner 用户、primary repository 和必要 secret；不再包含数据库 credential rotation。
7. 重写 queue/admin handlers，不再直接依赖 pgx pool。
8. 保持 API DTO 与 OpenAPI contract；运行 `make dto` 处理真实 contract变化。
9. 修复迁移直接影响的 unit/integration tests；与本迁移无关的既有失败应记录，不得通过禁用测试掩盖。

优先按 `app.go` wiring 从外向内处理，确保最终没有一个“临时 pgx adapter”残留。

Exit criteria：

- `server/app` 不导入 pgx；
- `server/internal` 业务包不导入 pgx/pgvector；
- server clean build；
- setup、owner login、repository 和 basic CRUD 的聚焦 smoke 或 service test 通过；
- 不存在名为 `PostgresStore`、`SQLBackend` 等仅用于双支持的接口。

### Phase 5：River SQLite 与事务性 ingest

任务：

1. `queue.New` 切到 `riversqlite`；
2. 所有 River generic transaction type切换为 `database/sql`；
3. 保持现有 workers、queues、periodic jobs 的注册与编译；
4. 保持媒体计算在事务外，并在需要时调整短 DB write batch；
5. SourceMaterializer按 AD-7 重构；
6. 将业务状态更新 + pipeline enqueue放到同一 transaction；
7. 实现最小 prepared/orphan reconcile，使已知中间态不会永久卡死；
8. 对 scan/upload 的核心路径保持幂等；cloud import 只要求迁移后不因数据库类型失效；
9. 为 enqueue commit、enqueue rollback、一个 worker 执行和一个 retry 路径编写聚焦测试；
10. 手工或自动验证一次进程重启后的数据库与 River 可重新启动。

本次最低故障验证：

```text
transaction rollback after one or more River InsertTx calls
restart after committed asset + jobs
restart with one prepared/recoverable asset state
```

Hardening backlog，不阻塞本 goal：

```text
每一个文件移动边界的 kill -9
metadata/thumbnail/transcode 中途 kill
全部 queue admin 与 periodic registration E2E
持续并发导入下的重复竞争与完整故障矩阵
```

Exit criteria：

- 业务状态与核心 pipeline jobs 能够共同 commit 或共同 rollback；
- 经过测试的中间态在重启后可以继续、重试或进入明确 recoverable/failed 状态；
- retry 不会在核心路径产生明显的不可控重复副作用；
- 聚焦测试中没有未处理的 `SQLITE_BUSY`；其他已知并发限制记录到 Hardening backlog；
- River queue handler 能编译，核心管理 API 至少有一条 handler/service smoke。

### Phase 6：文本、语义与人脸搜索

任务：

1. 完成 FTS5 index与基础 rebuild；
2. 将 PostgreSQL全文/模糊查询切换到SQLite；
3. 实现 authoritative embedding BLOB；
4. 接入 stable sqlite-vec exact KNN，或使用 AD-3 fallback；
5. 实现同事务 dual-write或一个明确、可重建的同步协议；
6. 迁移 image、video frame、face embedding；
7. 尽量保留 filter、ranking、pagination contract；
8. 使用小样本 brute-force reference test校验top-k；
9. 用现有开发数据记录一次代表性查询和rebuild观察；
10. 除非现有真实图库已经明显不可用，否则本 goal 不实现derived ANN sidecar。

Exit criteria：

- FTS、image semantic、video frame 和 face nearest 的代表性查询返回正确结果；
- insert/delete/reindex 的聚焦测试不会留下可复现的幽灵向量；
- index可以从authoritative data重建；
- 小样本正确性与观察结果写入plan；
- 大规模 benchmark 和 ANN 决策进入 Hardening backlog，不阻塞迁移。

### Phase 7：SQLite backup、restore 与 runtime generation

任务：

1. 删除 PostgreSQL backup tools；
2. 实现 SQLite 一致 snapshot；
3. 实现最小 manifest、checksum 与 integrity check；retention 保持现有简单语义；
4. 重写 periodic backup worker 和 admin backup APIs；
5. 引入 staged restore/pending marker；
6. 确保恢复前 drain 并关闭旧 DB handles，再替换并重新构造 runtime；
7. 保留一个 restore point，避免验证失败时覆盖唯一可用数据库；
8. 对有效 snapshot 完成一次 temp/current-host restore smoke；
9. 对 checksum 错误或损坏 snapshot 完成一次拒绝测试。

Hardening backlog，不阻塞本 goal：

- import高并发期间的重复在线备份压力测试；
- interrupted restore 的每阶段 fault injection；
- disk full、fsync失败和原子rename平台差异矩阵；
- Desktop与standalone所有生命周期组合的完整E2E；
- 自动rollback的多重失败注入。

Exit criteria：

- snapshot可独立打开并通过quick/integrity check，不需要复制 WAL/SHM；
- 一次有效restore后核心 settings、assets、repositories 和 River schema 可读；
- 明显损坏或不兼容的snapshot在替换主数据库前被拒绝；
- restore路径不会让新runtime继续持有旧 `*sql.DB`；
- 更完整的故障恢复矩阵已记录为 Hardening backlog。

### Phase 8：Docker SQLite-only

任务：

1. 删除 DB image/service/secret/volume；
2. 修改 server Dockerfile，删除 PostgreSQL package；
3. 将 SPA dist加入 release server image；
4. 配置 `/data/app-state/library.sqlite3`；
5. 简化 dev/release/E2E compose；
6. 将 Caddy/TLS转为optional proxy；
7. 更新 healthcheck；
8. 在当前环境测试一种持久化 mount，并对另一种至少完成 compose/config review；
9. 构建当前架构；其他架构由现有 CI 能力做 best-effort compile，不要求本 goal 完成运行时 smoke；
10. 更新 install/upgrade/reset/backup docs。

Exit criteria：

- fresh `docker compose up` 不启动数据库容器；
- 一个 Lumilio container可提供完整 UI + API；
- restart/kill/recreate container保留状态；
- 删除 app container但保留 state volume后可恢复；
- media root与state root明确分离；
- release image中没有 PostgreSQL binaries/client。

### Phase 9：Desktop SQLite-only

任务：

1. 删除 PostgreSQL supervisor代码与tests；
2. 简化 paths/config/resources；
3. 删除 bundled PostgreSQL artifacts与license entries；
4. 修改 macOS/Windows packaging；
5. 让 Desktop直接运行 in-process server；
6. 更新 tray状态，不再出现database child process阶段；
7. 在当前开发平台测试 first-run 与 second launch；force quit/restore restart 选择一条代表性路径验证；
8. 对 database path/permission failure 写一个聚焦测试或手工 smoke；
9. 验证当前平台 final package contents；
10. 更新 Desktop README与release docs。

Exit criteria：

- Desktop不启动任何数据库子进程；
- package不包含 PostgreSQL；
- 当前开发平台 build 与 first/second launch smoke通过；另一平台至少保持构建配置不被显式破坏；
- 当前平台完成一次非正常退出或等价 reopen smoke，并能重新打开SQLite；
- startup流程明显简化且错误可诊断。

### Phase 10：开发流程、CI、文档与清理

任务：

1. `make setup` 不再生成DB password secret；
2. `make dev` 不再启动Docker DB；
3. 删除或替换 `make db`；
4. `make db-reset` 安全删除已知dev SQLite、WAL、SHM与derived index；
5. 更新 Dev Container；
6. 更新所有 GitHub Actions；
7. 所有 backend integration tests使用 `t.TempDir()` DB；
8. 更新 README、中文 README、安装、Docker、Desktop、backup、troubleshooting、architecture和BACKEND docs；
9. 添加architecture guard，阻止 PostgreSQL依赖回归；
10. 删除dead code、unused configs、obsolete tests、images与scripts；
11. 运行本文 P0 必须质量门；对 full-suite、stress、browser E2E 和跨平台测试做 best effort；
12. 将未完成的硬化项整理为明确 backlog，再将本plan结果移至completed。

Architecture guard至少检查 active runtime中不存在：

```text
github.com/jackc/pgx
github.com/pgvector
riverpgxv5
postgres://
pg_dump
pg_restore
initdb
pg_ctl
pg_isready
createdb
POSTGRES_
LUMILIO_PG_
db_bootstrap_password
```

迁移计划和历史文档可以通过精确 allowlist保留“PostgreSQL”文字，运行时代码、config、Docker和packaging不得保留。

Exit criteria：

- fresh clone按README即可启动；
- CI不需要PostgreSQL service；
- 所有文档与实际命令一致；
- active code PostgreSQL grep为零；
- plan完成记录包含可获得的观察数据、风险、最终driver/vector决策和未完成Hardening backlog。

---

## 3. 验证、完成定义与执行纪律

### 3.1 分层质量门

遵守仓库 `AGENTS.md`，优先复用现有 Make targets，但不要为了本次迁移先建设一套庞大的测试基础设施。

#### P0：本 goal 必须通过

- 所有修改过的 Go 文件完成格式化，生成文件由正式命令生成；
- server 全量 compile，迁移直接影响的 Go packages 测试通过；
- fresh SQLite database migration、close、reopen 和 migration idempotency 测试通过；
- River migration、一个 job 执行、一个 retry，以及 app transaction + `InsertTx` commit/rollback smoke通过；
- setup、owner login、repository/basic asset CRUD 有聚焦 service/integration smoke；
- FTS 与 vector/fallback 有小样本正确性测试；
- Docker compose config有效，当前架构 image能够build，并完成一次启动/重启持久化 smoke；
- Desktop在当前开发平台能够build，并完成first/second launch smoke；
- 若 API/embedded SPA 构建路径被修改，Web typecheck/build通过；
- PostgreSQL architecture guard通过。

具体命令由仓库现有 targets 决定，例如 `make server-test`、`make desktop-test`、`make web-test`、`make dto`；只要求与本迁移直接相关且当前环境可执行的门为绿色。既有、无关失败必须记录，不能通过删除或禁用测试伪造绿色。

#### P1：Best effort，不阻塞 goal

- 根目录完整 `make test`；
- browser-mode全量 E2E；
- Docker完整E2E；
- macOS与Windows双平台运行时smoke；
- CI multi-arch runtime验证；
- 全量queue admin、cloud provider和外部media tool集成测试。

#### P2：Hardening backlog

- `sqlite-stress`；
- 完整 `sqlite-fault-test`；
- 100k/1M vector benchmark；
- 持续导入 + UI + backup + River maintenance并发测试；
- 每个文件/事务边界的kill -9矩阵。

### 3.2 核心功能验收矩阵

| 领域 | P0：本 goal 必须验证 |
|---|---|
| First run | 空state启动、application/River schema、owner setup、一个primary repository |
| Auth | owner login与当前基础session流程；MFA/passkey只要求数据库迁移与相关unit/service test不回归 |
| Repository | 一个local root、一次scan或basic CRUD；multiple/offline/remount完整矩阵延后 |
| Ingest | upload或in-place scan至少一条happy path，业务状态与jobs共同提交 |
| Processing | 至少一个metadata/thumbnail代表worker执行；依赖外部工具的完整矩阵延后 |
| Organization | album/tag/favorite等至少一个代表性关系CRUD；不要求所有组织能力逐项E2E |
| Search | 一个filter、FTS、image semantic、video/face各自可用的小样本查询；无对应fixture时至少覆盖存取和query层 |
| Agent | 相关schema/query可用，至少一个依赖数据库的agent路径smoke；不要求完整工具链E2E |
| Queue | enqueue、execute、retry、事务rollback；admin surface至少handler/service smoke |
| Backup/Restore | 一次有效snapshot、一次有效restore、一次损坏snapshot拒绝 |
| Docker | 当前架构clean start、UI/API ready、restart后state保留 |
| Desktop | 当前开发平台build、first launch、second launch、package不含PostgreSQL |
| Migration | fresh baseline与reopen幂等；不要求兼容旧PostgreSQL数据 |
| Failure | readonly/corrupt/prepared-state中选择有代表性的聚焦测试，不要求完整故障矩阵 |

Hardening backlog包括：MFA/passkey浏览器E2E、cloud import、duplicate race、multiple repositories、所有媒体worker组合、全部queue admin操作、interrupted restore、disk full、完整权限矩阵和跨平台运行时E2E。

### 3.3 核心可靠性验收

本 goal 必须证明：

- 业务 writer pool限制为一个连接；
- 外部工具、文件IO、模型调用和网络请求不位于SQLite写事务中；
- app state与核心River jobs可以同事务commit/rollback；
- fresh start、graceful restart和一次非正常reopen后数据库可打开，`quick_check`通过；
- prepared/recoverable state至少有一条可重复执行的恢复路径；
- backup snapshot可独立打开；损坏snapshot在替换前被拒绝；
- vector virtual/derived data可以从authoritative embedding重建；
- 聚焦测试中出现的 `SQLITE_BUSY` 必须修复或作为有明确复现条件的已知限制记录，不能无限重试或吞掉错误。

以下属于Hardening backlog：持续导入/UI/backup/River并发压力、完整kill -9矩阵、所有interrupted jobs恢复组合、WAL checkpoint/truncate长时间测试、prepared/orphan全状态空间、restore自动rollback多重故障注入。

### 3.4 性能观察（非阻塞）

本次不设置百分比、p95、100k或1M规模硬门槛。Agent应在成本较低时记录：

- clean startup的数量级；
- 当前开发库上的一个常用browse/filter query；
- 一个小样本vector top-k query；
- SQLite database、Desktop package或Docker image中至少一项体积变化；
- idle memory对比（仅在已有可复用命令时）。

只有出现“现有真实开发库上明显不可交互”、持续锁死、事务跨越外部工作或单次普通查询达到秒级且可直接定位的严重回归，才阻塞本goal。其余性能问题记录为Hardening backlog。不得为通过性能观察恢复多写连接或重新引入PostgreSQL。

### 3.5 核心 Definition of Done

以下全部成立后，`experimental/sqlite` goal可以结束；P1/P2 Hardening backlog不阻塞：

- [x] SQLite 是唯一数据库实现和唯一运行时依赖。
- [x] PostgreSQL migrations、driver、pool、types、backup tools、Docker service和Desktop runtime全部删除。
- [x] config schema v2在dev、Docker、Desktop与tests统一。
- [x] fresh SQLite baseline覆盖当前应用所需schema，允许清空旧开发数据。
- [x] sqlc使用SQLite/`database/sql`并可重复生成。
- [x] River使用SQLite；核心业务状态与pipeline enqueue共享事务。
- [x] SourceMaterializer及关键queue入口具备事务性enqueue，并有最小prepared/recoverable恢复路径。
- [x] 所有当前worker和periodic job定义能够注册/编译；至少一个代表worker和retry路径实际执行通过。
- [x] FTS、semantic、video和face search完成小样本正确性smoke，或对缺少fixture的路径完成存取/query层验证。
- [x] vector方案通过小样本正确性验证；规模benchmark和ANN决定已进入Hardening backlog。
- [x] backup/restore改为SQLite snapshot，并完成一次有效restore和一次损坏snapshot拒绝。
- [x] Docker无DB service；当前架构单Lumilio container能服务SPA+API并在restart后保留state。
- [x] Desktop无PostgreSQL子进程和bundle；当前开发平台build与first/second launch通过。
- [x] 开发流程与主要CI配置不再要求PostgreSQL；无法在当前环境执行的跨平台runtime测试已记录。
- [x] P0 changed-area tests、migration tests和smoke通过；无关既有失败已明确记录且未被禁用。
- [x] 聚焦路径不存在被吞掉或无限重试的 `SQLITE_BUSY`；已知并发限制进入Hardening backlog。
- [x] README与内部架构文档更新。
- [x] 全仓库runtime PostgreSQL guard通过。
- [x] 已记录能够低成本获得的启动、查询、体积或内存观察；没有硬性benchmark门槛。
- [x] plan中的Progress、Decision Log、Surprises、Outcomes和Hardening backlog完整。
- [x] active plan移动至completed，且最终commit保持clean working tree。

### 3.6 Agent执行纪律

- 将本文作为 living execution plan。每完成一个phase，更新Progress、Decision Log、Surprises与验证结果。
- 在实际代码与本文细节冲突时，先记录事实与决策，再采用满足Goal和不变量的最简单方案。
- 普通实现选择自主决定，不为可逆细节暂停等待确认。
- 不以大段占位TODO、disabled test或“之后再清理”的PostgreSQL dead path换取绿灯。
- 每个phase形成可审查commit；使用仓库约定的commit message。
- generated files必须通过正式命令生成，并在记录中写明命令。
- 不覆盖用户在分支上的无关改动。
- 不为修复测试降低数据完整性约束。
- vector extension若阻塞，按AD-3 fallback继续完成核心迁移；不得重新引入PostgreSQL。
- 任何失败都应留下可复现命令、日志摘要和下一步，不写含糊的“可能是环境问题”。
- 最终报告必须列出：删除项、架构变化、数据布局、备份语义、driver/vector选择、可获得的性能观察、剩余已知限制和明确的Hardening backlog。

## Progress

- [x] Phase 0：基线与inventory
- [x] Phase 1：driver/River/vector spike
- [x] Phase 2：config与DB runtime
- [x] Phase 3：SQLite baseline与sqlc
- [x] Phase 4：业务层去pgx
- [x] Phase 5：River与事务性ingest
- [x] Phase 6：FTS与vector
- [x] Phase 7：backup/restore与runtime generation
- [x] Phase 8：Docker
- [x] Phase 9：Desktop
- [x] Phase 10：dev/CI/docs/cleanup
- [x] Core Definition of Done
- [x] Hardening backlog已记录（不阻塞）

### Phase 0 执行记录（2026-07-25）

#### 分支、基线与 destructive reset

- 分支确认：`experimental/sqlite`，起点 `bb2f8df0`。
- 迁移前基线：`make server-test` 全量通过。后续 Server 测试失败默认按迁移回归处理，除非有更强证据证明与 SQLite 修改无关。
- 未为旧 PostgreSQL runtime 启动容器、构造 benchmark 数据或采集严格性能基线；这些都不是解除迁移风险所需的低成本观察。
- destructive reset 已确认：不转换 PostgreSQL 开发数据。Phase 2 后开发 reset 只允许删除仓库已知的 SQLite database、`-wal`、`-shm` 和可重建 derived index；不得接受任意路径。旧 Compose PostgreSQL volume 在 Phase 8 删除后成为不兼容的废弃开发状态。
- 实施保持 SQLite-only：不创建 backend interface、feature flag、PostgreSQL adapter 或 dead implementation。

#### Schema parity inventory

现有 PostgreSQL schema 由 14 组 migrations 组成，共 52 张应用表、125 个显式索引、6 个 trigger 和 6 个 trigger function。River schema 不在应用 baseline 中，仍由 River 自己的 SQLite migrations 管理。

| 领域 | 当前 PostgreSQL 表 | SQLite 去向 |
|---|---|---|
| identity/config | `users`, `registration_sessions`, `settings`, `user_mfa_recovery_codes`, `user_mfa_totp_credentials`, `user_webauthn_credentials`, `refresh_tokens`, `system_state`, `repository_defaults` | Phase 3 squashed `STRICT` baseline；UUID/time/JSON 使用 AD-6 统一类型 |
| repositories/assets | `repositories`, `repository_roots`, `repository_scan_runs`, `assets`, `thumbnails`, `tags`, `asset_tags` | Phase 3 baseline；保留 FK、unique、partial index 和 repository ownership |
| collections/locations/duplicates | `albums`, `album_assets`, `media_items`, `media_item_assets`, `asset_stacks`, `asset_stack_members`, `duplicate_groups`, `duplicate_group_assets`, `duplicate_group_edges`, `location_clusters`, `location_cluster_assets`, `reverse_geocode_cache` | Phase 3 baseline；PostgreSQL enum/JSON/array/query 语义改写为 SQLite |
| ML/analysis/search | `embedding_spaces`, `embeddings`, `ocr_results`, `ocr_text_items`, `face_results`, `face_items`, `face_clusters`, `face_cluster_members`, `species_predictions`, `classifier_definitions`, `asset_quality_scores`, `search_embeddings` | 权威数据进入普通 SQLite 表；FTS5/唯一 vector 实现是可重建查询结构 |
| cloud/sharing | `cloud_credentials`, `cloud_import_runs`, `cloud_sync_cursors`, `cloud_sync_files`, `repository_cloud_bindings`, `share_links` | Phase 3 baseline；secret 仍由现有加密边界负责 |
| agent runtime | `agent_checkpoints`, `agent_pins`, `agent_threads`, `agent_runs`, `agent_refs`, `agent_pending_effects` | Phase 3 baseline；保持现有审计、ownership 与恢复语义 |

125 个索引按源 migration 全量纳入迁移核对：`000002` 8 个、`000003` 34 个、`000004` 19 个、`000005` 43 个、`000006` 8 个、`000009` 2 个、`000011` 3 个、`000012` 4 个、`000014` 4 个。普通 unique/partial/access-path index 在 SQLite baseline 中保留；`tsvector`/GIN 迁移至 FTS5；HNSW 迁移至选定的 exact vector/fallback；只服务 PostgreSQL 实现细节的索引删除并在 Phase 3 Decision Log 说明。

| 当前 trigger/function | SQLite 去向 |
|---|---|
| `trg_assets_updated_at` / `set_assets_updated_at` | SQLite trigger 或同一短事务内显式更新时间 |
| `trg_location_clusters_updated_at` / `set_location_clusters_updated_at` | SQLite trigger 或同一短事务内显式更新时间 |
| `face_cluster_members_count_trigger` / `update_cluster_member_count` | SQLite trigger，保留成员计数不变量 |
| `face_items_update_trigger` / `update_face_results_updated_at` | SQLite trigger 或同一短事务内显式更新时间 |
| `ocr_text_items_update_trigger` / `update_ocr_updated_at` | SQLite trigger 或同一短事务内显式更新时间 |
| `trg_assets_create_media_item` / `create_media_item_for_asset` | Phase 5 ingest transaction 中显式创建，避免隐藏的 PostgreSQL PL/pgSQL 行为 |

#### Query parity inventory

当前 `server/internal/db/repo/queries` 有 33 个 SQL 文件、451 个 named queries。所有 query 保持在同名领域文件中逐个改写并由 SQLite/`database/sql` sqlc 重新生成；不以 Go N+1 拼接、pgx adapter 或手写 generated code 兼容。下表覆盖全部 named queries，数字是该文件必须迁移并通过 sqlc 的 query 数量。

| Query file | 数量 | SQLite 去向 |
|---|---:|---|
| `agent.sql`, `agent_facets.sql`, `agent_pins.sql`, `agent_tools.sql` | 25 / 10 / 8 / 12 | SQLite sqlc，同名 query |
| `albums.sql`, `relationships.sql`, `folders.sql`, `tags.sql` | 15 / 3 / 2 / 9 | SQLite sqlc，同名 query |
| `assets.sql`, `indexing.sql`, `asset_quality_scores.sql` | 81 / 14 / 2 | SQLite sqlc；全文与 array/JSON 聚合按 SQLite 语义重写 |
| `repositories.sql`, `repository_defaults.sql`, `repository_roots.sql`, `repository_scans.sql` | 18 / 2 / 8 / 8 | SQLite sqlc，同名 query |
| `users.sql`, `registration_sessions.sql`, `mfa.sql`, `passkeys.sql`, `settings.sql`, `system_state.sql` | 21 / 6 / 8 / 6 / 2 / 2 | SQLite sqlc，同名 query |
| `duplicates.sql`, `stacks.sql` | 22 / 16 | SQLite sqlc，同名 query |
| `locations.sql`, `stats.sql` | 10 / 6 | SQLite sqlc；`date_trunc`/search 改写 |
| `embeddings.sql`, `search_embeddings.sql`, `faces.sql`, `ocr.sql`, `people.sql`, `species.sql` | 17 / 5 / 52 / 12 / 6 / 8 | 权威 BLOB + 唯一 exact vector/fallback query；其余 SQLite sqlc |
| `cloud_credentials.sql`, `cloud_sync.sql`, `share_links.sql` | 22 / 4 / 9 | SQLite sqlc，同名 query |

#### Runtime、业务与交付耦合 inventory

- 直接 PostgreSQL import 文件数量：`server/app` 2、`internal/agent` 8、`internal/api` 20、`internal/cloud` 2、`internal/db` 38（含 generated repo）、`internal/processors` 8、`internal/queue` 11、`internal/search` 3、`internal/service` 31、`internal/sourcing` 1、`internal/storage` 7。
- DB runtime：`internal/db/db.go`, `dsn.go`, `migration.go` 使用 pgx pool、PostgreSQL DSN/password self-heal、golang-migrate Postgres driver 和 `riverpgxv5`。
- App/queue/ingest：`app/app.go` 传播 `*pgxpool.Pool`；`queue_setup.go` 使用 `river.Client[pgx.Tx]`；`sourcing/materializer.go` 尚未提供业务状态与 pipeline `InsertTx` 原子性。
- Backup：`internal/db/backup` 与 `backup_service.go` 使用 `pg_dump`/`pg_restore` 和数据库 credential。
- Config/setup：schema v1 包含 host/port/user/name/SSL/password files/tools bin；first-run readiness 包含数据库 password rotation。
- Docker/dev/CI：root/release/E2E/devcontainer Compose、Server Dockerfile、`db.Dockerfile`、Makefile 和 GitHub Actions 均有 PostgreSQL service/image/secret/client 路径。
- Desktop：supervisor 管理 PostgreSQL process、socket/port/data/password/resources/quarantine；macOS/Windows scripts 与 package/license 含 PostgreSQL。
- Docs/UI：README、安装/完整性/backup 文档和 Desktop control panel 仍描述或展示 PostgreSQL 生命周期。

#### 功能去向与提交纪律

| 功能面 | SQLite 完成证据 |
|---|---|
| setup/auth/repository/assets | schema v2 + focused owner login/repository/basic asset CRUD smoke |
| organization/location/duplicate | SQLite sqlc 生成与代表性关系 CRUD/search test |
| queue/ingest/processing | River SQLite migration、execute/retry、InsertTx commit/rollback、prepared reconcile、代表 worker |
| FTS/semantic/video/face | FTS5 与小样本 brute-force top-k 正确性、insert/delete/rebuild |
| Agent/cloud | schema/query 编译和至少一个 DB-backed Agent smoke；cloud 不因 DB 类型失效 |
| backup/restore | Online Backup snapshot、有效 restore、损坏 snapshot 拒绝、runtime generation restart |
| Docker/Desktop | 单应用容器 restart 持久化；当前 macOS first/second launch 与 package guard |
| dev/CI/docs | PostgreSQL architecture guard、SQLite-only commands/manifests/docs |

每个 Phase 单独形成可审查 commit；生成代码的提交记录生成命令。Phase 0 只提交 living plan 与 inventory，不混入实现。

### Phase 1 执行记录（2026-07-25）

#### 唯一 driver、扩展与连接决策

- 唯一 SQLite driver 选择 `github.com/mattn/go-sqlite3 v1.14.48`。生产迁移不保留第二 driver、双后端 interface 或 fallback；`go list -deps` 证明 spike 的已编译依赖只有该 SQLite driver。
- vector 选择 `github.com/asg017/sqlite-vec-go-bindings/cgo v0.1.6` 静态注册的 `vec0` exact search。当前 Go CGo binding 的可用稳定版本对应 `vec_version() = v0.1.6`；不在运行时加载外部 extension。
- River 固定为 `github.com/riverqueue/river v0.24.0` 与 `riverdriver/riversqlite v0.24.0`。应用表和 River 表共用一个 `*sql.DB`，writer pool 固定为 1，`InsertTx` 使用同一个 `*sql.Tx`。
- FTS5 通过统一 `sqlite_fts5` build tag 编译；root Make targets、Server Docker builder、Desktop release scripts 和直接 Go build 的 CI job 均传播该 tag。
- fixed pragmas spike 已验证 `foreign_keys=ON`、WAL、`synchronous=NORMAL`、`busy_timeout=5000`、`temp_store=MEMORY`、`wal_autocheckpoint=1000`，且连接没有 lifetime recycling。Phase 2 将此逻辑移入唯一 production DB boundary。

#### 聚焦兼容性结果

- `go test -tags=sqlite_fts5 ./internal/db/dbtypes ./internal/db/sqlitespike -count=1 -v` 通过：open/close、fixed pragmas、`STRICT`、JSON1、FTS5 trigram、768D/512D `vec0` insert/top-k/delete、sqlc UUID/null UUID/time/JSON/BLOB mapping 全部正确。
- application migration 与 River migration 在同一 database file 中通过；River migration 第二次执行不产生版本变化。River client start/stop、enqueue、一次有意失败后的 immediate retry、最终 `completed`/attempt 2 均通过。
- app row 与 River `InsertTx` 的 rollback 同时回滚，commit 同时可见。聚焦测试未观察到未处理或无限重试的 `SQLITE_BUSY`。
- `make server-test` 使用统一 FTS5 tag 全量通过；`make desktop-test` 通过 Desktop module test/build gate。
- `docker build --target builder -f server/Dockerfile -t lumilio-server-sqlite-spike:phase1 .` 在 Linux/arm64 通过。随后 image 内 `go test -tags=sqlite_fts5 ./internal/db/sqlitespike -count=1 -v` 在 0.344s 内通过全部 River/FTS/vector/sqlc tests。这是小样本兼容性观察，不代表真实 library 性能 benchmark。
- sqlc fixture 通过 `cd server/internal/db/sqlitespike/sqlcfixture && sqlc generate` 生成。第一次用 `json.RawMessage` 时 SQLite `STRICT TEXT` 拒绝其 BLOB `driver.Value`；最终引入验证 JSON 且明确返回 `string` 的 `dbtypes.JSON`。UTC Unix microseconds 由 `dbtypes.Timestamp` 集中处理。

#### Phase 1 Hardening backlog

- River process kill 后的完整状态恢复、cancel、periodic 和全部 queue admin 行为留到 P1/P2。
- Windows runtime smoke、Docker multi-arch runtime 和完整 macOS packaged runtime 留到各交付 Phase/P1；本 Phase 已完成当前 macOS module gate和 Linux/arm64 container runtime。
- macOS SDK 对 `sqlite3_auto_extension` 给出 deprecated/process-global auto-extension warning，但同进程多连接的 extension registration 与 vector query 实测通过。升级 SQLite/vector binding 或切换显式 connection hook 前必须保留该兼容性测试。
- Desktop gate 暴露现有 Homebrew `objects`/`vips`/`libraw` deployment target warning；它与 SQLite 链接无关且不阻塞本迁移。
- 代表性 library 规模的 vector latency、内存、WAL checkpoint 行为和 ANN/sidecar 阈值留到 Phase 6/P2；本 Phase 只证明 exact search 正确性。

### Phase 2–7 执行记录（2026-07-26）

#### Phase 2：config 与唯一 DB runtime

- manifest schema 已提升为 v2；`[database]` 只接受完整、显式的持久化 `path`。旧 host/port/user/name/SSL、数据库 password file、tool bin 和环境 override 已删除，v1 会被明确拒绝。
- `db.Open` 是唯一生产数据库边界：一个 `database/sql` writer connection，固定 pragmas、静态 sqlite-vec 注册、application id、启动 quick/integrity check、`WithTx`、shutdown optimize 与 WAL truncate checkpoint。`dsn.go` 和 password self-heal 已删除。
- embedded Lumilio migration 与 River SQLite migration 在同一文件、同一 pool 上按顺序运行。新增 runtime 测试覆盖 fresh migrate、重复 migrate、close/reopen、pragma、catalog identity、持久路径拒绝和 corrupt catalog 拒绝。

#### Phase 3：fresh baseline、sqlc 与 domain types

- 14 组旧 migration 已替换为单一 `000001_sqlite_baseline.up.sql`：52 个应用表、STRICT/foreign key/check/partial index、同步 aggregate trigger、FTS5 trigger 和 sqlite-vec derived index 均在 fresh catalog 创建。旧 migration 文件与旧 generated 聚合文件已删除。
- `server/sqlc.yaml` 已切换到 SQLite + `database/sql`；451 个原 query 按 SQLite parser 限制拆分但保持 domain/name 语义，UUID、nullable UUID、Unix microseconds、JSON TEXT、collection 和 vector BLOB 通过集中 `dbtypes` 映射。生成命令：`cd server && sqlc generate`。
- baseline/schema、UUID/time/JSON/vector round-trip、foreign key/unique/partial-index 和 fresh asset insert 均由测试覆盖。真实 ingest transaction test 发现并修复了 `assets.asset_id`、`upload_time`、`updated_at` 从旧 engine defaults 脱落的问题；ID 现由 Go 显式生成，时间由 SQL 显式写入。

#### Phase 4：业务层 SQLite 化

- `server/app`、`server/internal`、handler、service、worker、Agent、cloud、storage 和 benchmark tooling 已移除直接 pgx/pgtype/pgconn/pgvector 使用，统一为 `*db.DB`、`*sql.DB`、`*sql.Tx`、`*repo.Queries`、`google/uuid` 与 `dbtypes`。
- setup/bootstrap gate 现在只检查应用设置、owner、primary repository 和 secret；queue admin、auth、asset、people、stats、repository scan 等 raw SQL 均已改为 SQLite 语义。
- `go mod tidy` 删除 direct pgx、pgvector、riverpgxv5、Postgres migrate driver 和 lib/pq。River v0.24 自身的 production `testsignal → riversharedtest` import 曾把 pgx 作为不可调用的 upstream transitive code 拉入 compiled graph；Phase 9 package gate 已用只保留生产 package 的窄本地 fork/replacement 消除，Phase 10 architecture guard 将防止回归。

#### Phase 5：River SQLite 与事务性 ingest

- River client、workers、periodic jobs 和 admin handler 使用 `riversqlite` 与 `river.Client[*sql.Tx]`；应用表和 River jobs 共享单 writer transaction。
- staging ingest 先提交 prepared asset + logical media item，再移动文件，最后用一个短事务更新 target/status 并 `InsertTx` 全部 metadata/thumbnail/transcode jobs。in-place create/update 与全部 core jobs 在一个短事务内完成。
- prepared status 保存 deterministic inbox target；job retry可识别“staging 已移动、final 已存在”并补完 transaction；两侧文件都不存在会进入可见 failed/retryable 状态，不删除未知文件。
- `TestAssetMediaAndPipelineCommitOrRollbackTogether` 在真实 fresh catalog 上证明 asset、media item、relation 和多个 River jobs 同时 commit；在一个 job 已插入后制造错误时四者全部 rollback。Phase 1 worker/retry/reopen 测试继续覆盖 River execute/retry。

#### Phase 6：FTS5 与 sqlite-vec

- location、filename/label/species search 已改为 FTS5（content sync trigger + `bm25`）；OCR 后续由 revisioned outbox 驱动的 Bleve 双 analyzer sidecar 取代。aggregate filter 使用 positional bind、`json_each`、SQLite date/JSON/LIKE 语义。
- semantic 768D 与 face 512D 权威向量保存在 STRICT ordinary tables，`vec0` 只作为同 transaction trigger 维护的可重建 exact-search structure；video frame vectors与默认 search space在短 transaction 中替换。
- spike 的 768D/512D insert/top-k/delete 与 production search/service tests 全部通过。代表性真实 library 规模 latency、内存和 ANN threshold 仍是 Hardening，不阻塞 experimental goal。

#### Phase 7：native snapshot、restore 与 runtime generation

- backup 使用 mattn SQLite Online Backup API 创建独立 snapshot，不复制 live WAL 文件。数据库与 paired manifest 都经过 temp-file、fsync、atomic rename；manifest记录 library identity、SHA-256/size、app/config/application/River migration、SQLite/vector version与完整性结果。
- restore 先独立 read-only 校验 identity、checksum、quick check、foreign key、migration/version compatibility，再写 staged marker。`app.Run` 仅在旧 HTTP/River/DB generation 完全 drain/close 后替换文件并启动新 generation。
- 原 catalog 保留为 restore point；新 listener、wiring、settings/users health 成功后才完成 marker，启动失败会 rollback 原 catalog。测试覆盖 standalone snapshot、checksum拒绝、staged apply、restore point、rollback/retention和forced scheduler。
- `go test -tags=sqlite_fts5 ./...` 在上述 Phase 2–7 完成后全量通过。

### Phase 8 执行记录（2026-07-26）

#### 单容器交付与数据边界

- 删除 `server/db.Dockerfile`、`docker-compose.release.dbport.yml`、所有 `db` service/volume/bootstrap password secret、独立 `web` service 和数据库依赖关系。root、release、E2E、CI overlay 与 host-mDNS compose 现在只把 `lumilio` 作为应用 service；E2E 另保留独立 Lumen fixture。
- release Server image 在独立 Node stage 构建 SPA，并与 Go Server 一起交付；`server.web_root=/app/web`，同一 `6680` listener 同时提供 SPA 与 API。release compose 保持外部默认端口 `6657`，TLS 由部署者的可选可信反向代理提供。
- active catalog 固定为 `/data/app-state/library.sqlite3`。媒体挂载 `/data/storage` 与 machine-local state 挂载 `/data/app-state` 明确分离；日志、backup 与私有状态也在 app-state。
- root entrypoint 只在启动边界创建/检查两个挂载，给不可写挂载提供明确诊断，将 app-state 收紧到 `0700`，再通过 `gosu` 降为 UID 10001。Server binary、媒体处理和请求处理仍以非 root 身份运行。
- runtime image 删除 PostgreSQL apt repository/client 与 `secretinit`，不包含 PostgreSQL binary、client 或 libpq package。public Docker configurator、README、双语安装/BreakGlass 与 backup guidance 已改为单镜像、双挂载、schema v2 和应用 snapshot 语义。

#### Compose、image 与持久化验证

- `docker compose config` 对 root、release、E2E、CI overlay 与 host-mDNS 组合均通过；resolved root/release service 只有 `lumilio`，E2E 只有 `lumilio` 与 `lumen-fixture`。
- `docker build --secret id=npmrc,src=/Users/zhanzihao/.npmrc --build-arg VERSION=sqlite-phase8 -f server/Dockerfile -t lumilio-sqlite:phase8 .` 在当前 Linux/arm64 OrbStack builder 通过。最终 image 为 319,084,043 bytes；`command -v psql/postgres/pg_dump` 与 `dpkg-query` PostgreSQL/libpq package guard 均为空，SPA index 与 Server binary 均存在。
- fresh bind-mount smoke 在 API ready 后返回 `{"status":"ok","version":"sqlite-phase8"}`，`/` 返回 Lumilio SPA。catalog mode 为 `0600`，位于 state mount；media mount中不存在 active database。
- 同一容器 restart 与删除/recreate application container 后都恢复 healthy；`system_state.library_id=dde4a8e1fd47c74b13beb121ca63c1af` 三次保持一致。root `docker compose up -d --build --wait` 的隔离 smoke 只创建一个 healthy `lumilio` container，并从同一公开端口返回 UI + API。
- public Docker configurator 与双语文档通过 `vitepress build docs` production build；依赖安装与 source build 分为 networked manifest-only container和 `--network none` read-only source container，未把 host Corepack signing-key故障绕进仓库配置。

### Phase 9 执行记录（2026-07-26）

#### Desktop lifecycle 与本地数据布局

- Desktop supervisor 不再发现、初始化、启动、等待或终止数据库子进程。manifest 固定为 schema v2，active catalog 为 `<application-data>/Lumilio Photos/library.sqlite3`；supervisor 写出完整资源路径后直接调用 in-process `app.Run`，stop 顺序由应用 drain HTTP、River 与 SQLite。
- machine-local parent、catalog、config、logs、secrets、backups、cloud 与 media storage 的边界显式保留；数据库 parent 诊断、目录权限和 database `0600` 等价权限由测试覆盖。媒体工具仍是显式解析的外部/打包资源，不允许 Server 在 immutable config 之后偷偷 fallback。
- 删除 Desktop PostgreSQL supervisor/process/smoke、resource relocation、license、release artifact workflow和控制面板 DB phase/log；macOS/Windows packaging、installer、resource fetch 与 release workflow 只交付应用、SPA、media libraries/tools 和 notices。
- `make desktop-test` 全量通过。真实 first/second launch integration test 在独立 application-data 上启动 API + SPA、创建 catalog、停止并重新打开，证明 `system_state.library_id` 稳定且 catalog mode 为 `0600`；这条 close/reopen path 是本 Phase 对 abnormal/reopen 风险的聚焦门槛，完整 kill matrix 留在 Hardening。
- 首次真实 Desktop launch 暴露 baseline `system_state.bootstrap_phase` CHECK 漏掉运行时合法值 `db_rotated`；baseline 与 schema test 已同步修复，而不是在 bootstrap 代码中绕过约束。

#### 依赖图与 package guard

- 最终 bundle audit 发现 River v0.24 的生产 `testsignal` package 通过 `riversharedtest` 把 pgx 实际链接进 Desktop binary。仓库现在包含基于上游 MPL-2.0 v0.24、只保留 reachable production packages 的 `server/third_party/river` 与 `third_party/river-rivershared`；唯一行为调整是缺失 driver 的 backend-neutral diagnostic，以及 `testsignal` 直接使用上游 10 秒默认 timeout。Server 与 Desktop 用显式 local replacements，`go mod tidy` 后 module graph 不再含 pgx/pgvector/riverpgx/libpq/GORM PostgreSQL driver。
- Server Docker builder 显式复制两个 local replacement module，避免 Desktop cleanup 破坏 Linux delivery build cache/source边界。第三方 notices 通过 `node desktop/scripts/generate-third-party-notices.mjs` 重新生成，共 200 项；generator统一 CRLF、尾空白与单一文件末换行。
- `make server-test`、`make desktop-test` 与 `make desktop-build` 均通过。最终 ad-hoc signed macOS app 通过 deep/strict `codesign`；bundle 为 321,476 KiB，main binary 为 80,890,048 bytes。文件名、`otool`、Go build metadata、binary strings、module graph 与 shipped notices 的 package guard 都没有 PostgreSQL/pgx/pgvector/riverpgx/libpq。
- Windows release configuration完成静态审查：workflow、PowerShell resource fetch、build script、installer source与文档均不再引用数据库 distribution/path/artifact。Windows runtime smoke仍属于Hardening。
- 本机缺少 web依赖时，普通 locked install 在 sharp optional native postinstall 上失败；`vp install --ignore-scripts` 完成依赖恢复，随后生产 SPA build 与 Desktop bundle build通过。现有 Homebrew media library deployment-target warnings仍是独立 packaging hardening，不是 SQLite blocker。

### Phase 10 执行记录（2026-07-26）

#### 开发流程、CI、文档与回归防线

- `make setup` 现在用 `CI=1 VITE_GIT_HOOKS=0 vp install` 确定性安装前端依赖，并安装从仓库 root 正确进入 `web/` 执行 `vp staged` 的 hook。`make dev` 只启动本地 Server 与 Vite，不再依赖 Docker 数据库；`make db-reset` 只删除经过精确解析的 catalog、WAL、SHM 与 derived state。
- devcontainer、CI、release workflows、Make targets、ignore rules 和文档均删除旧数据库 service、password secret、secret initializer、旧 web image/Caddy 与 PostgreSQL 工具链。release workflow 只构建一个 multi-arch `lumilio-server` image。
- `scripts/check-architecture.sh` 扫描活动源码、完整 Go compiled dependency graph、sqlc 残留宏、危险的动态 slice/fixed placeholder 组合、数据库侧随机 UUID 和旧 PostgreSQL package/binary。CI 无条件执行该 guard；UUID 与随机数据只能在 Go 中生成，所有 required-column insert contract 由 schema test 覆盖。
- 首次 setup UI/API 中已经没有数据库准备或密码轮换阶段；状态只表达 admin/repository bootstrap，backup UI 与双语文档只描述 SQLite snapshot。OpenAPI、前端 DTO、Redoc、i18n extraction 与 feature docs 已重新生成或同步。
- sqlc 宏迁移后的真实测试暴露并修复 typed CTE/JSON collection、NULL bool、timestamp、UUID、queue timestamp scan、location cluster transaction、sqlite-vec `k <= 4096`、snapshot sidecar、forced backup、video semantic seek、恢复 fixture 权限等边界。生成命令为 `cd server && sqlc generate`、`make dto`、`cd web && vp exec i18next-cli extract`；生成文件未被手工修补。

#### 最终验证与可获得的观察

- `make server-test`、`make web-test`、`make desktop-test`、`make desktop-build`、Web production build、VitePress production build、两个 Go module 的 `go mod tidy -diff`、sqlc regeneration、DTO generation、architecture guard 与 diff/YAML/Compose checks 全部通过。
- fresh final E2E stack 共 12/12 browser tests 通过：smoke 4、auth hardening 5、video semantic 2、backup recovery 1。真实 worker 完成 location clustering；备份目录只含 manifest 与独立 `.sqlite3` snapshot，没有 WAL/SHM。
- final Linux/arm64 release image 为 318,888,485 bytes（image ID `sha256:a7c7f8990a783bc65ae49a3a1e7018afcdcdd9966f07797a2abed49f0757a31a`），包含 Server 与 SPA，不含 PostgreSQL binary/client/libpq package。fresh local container 从进程日志开始到 listener ready 约 43 ms；这是 ML disabled 的单次环境观察，不是 benchmark。
- final ad-hoc signed macOS app 为 321,456 KiB，main binary 为 80,874,016 bytes，通过 deep/strict codesign；linked libraries、Go build metadata、strings、module graph、notices 与 bundle filenames 不含 PostgreSQL/pgx/pgvector/riverpgx/libpq。
- release image 的 fresh、restart 与 container remove/recreate 均保持同一 `system_state.library_id`；每次 graceful stop 后由同一 container runtime 执行的离线 `PRAGMA quick_check` 均返回 `ok`。不能用 host SQLite tool 打开仍在 container mount 上运行的 catalog。
- 小型 E2E fixture 的 browser 首屏观察约 523 ms，三个 video case 约 3.5–5.6 s 并正确定位 semantic frame。它们只证明交互路径和 seek 行为；没有测量 representative library 性能或严格 idle memory，不能据此宣称规模或内存改善。
- draft PR 远端 CI 将本机无法执行的 Windows CGo path 纳入完整 Server/Desktop tests。它暴露 drive-letter path 被编码成无效 URI authority、Unix mode bits 被误用于 Windows privacy validation、read-only handle `Sync()`/directory fsync 与 rename durability 的平台差异，以及 Desktop E2E tool fixture 缺少 `.exe` suffix；最终实现集中 `file:///D:/...` URI builder，让 Windows catalog/backup 使用 protected DACL，用 read-write file flush 与 `MOVEFILE_WRITE_THROUGH` 完成 snapshot/restore 原子 finalization，并让 test resources 经过 production `toolExe` resolution。后续 Windows CI 必须通过才视为远端验证完成。

#### Consolidated Hardening backlog

- 对持续并发 import、UI browse、backup、River execution 与 WAL checkpoint/truncate 做长时间压力测试，并补全 kill -9、disk-full、prepared/orphan、interrupted job、restore rollback 多故障注入矩阵。
- 在代表性 100k/1M vector library 上测量 exact-search latency、内存和 rebuild 成本，再决定 ANN/sidecar 阈值；当前 exact `vec0` 是正确性基线，不是无条件规模结论。
- 完成 Windows installer 安装/启动 smoke、非 CI architecture runtime 与发布后的 multi-arch image 观察；持续跟踪 macOS `sqlite3_auto_extension` deprecation 和 Homebrew media library deployment-target packaging warnings。
- 当 upstream River production dependency closure 不再链接 pgx 时升级并删除窄 fork；在此之前保持 license、差异说明和 architecture/package guard。
- 不得跨 host/container VFS 边界用 host SQLite tool 打开活动 catalog、WAL 或 SHM；live inspection 必须使用应用 Online Backup snapshot，直接离线检查只能在应用 graceful stop 后于 owning runtime 内进行。
- 将正常 client cancellation 映射为非 500/可识别的 audit status，避免 E2E reseed 取消请求产生误导日志；这不影响 catalog 正确性或迁移完成状态。

## Decision Log

| Date | Decision | Reason | Consequence |
|---|---|---|---|
| 2026-07-25 | `experimental/sqlite`采用SQLite-only destructive migration | 当前无生产数据，避免双后端长期成本 | 不提供PostgreSQL数据转换或runtime fallback |
| 2026-07-25 | active DB默认位于machine-local app state | 多repository语义与WAL网络文件系统限制 | 通过snapshot/move实现可迁移性 |
| 2026-07-25 | River与业务表共享同一SQLite文件和writer | 保留transactional enqueue | 事务必须短，writer pool为1 |
| 2026-07-25 | stable exact vector先行，ANN为derived fallback | 降低sqlite-vec pre-v1/ANN风险 | 正确性优先，规模benchmark决定sidecar |
| 2026-07-25 | benchmark、stress、全量fault injection和跨平台runtime E2E不作为experimental goal硬门槛 | 当前目标是先完成可运行的SQLite-only架构实验 | 使用P0聚焦验证完成迁移，P1/P2进入Hardening backlog |
| 2026-07-25 | Phase 0 inventory 以 schema object、query 文件和业务/交付边界分层追踪 | 451 个 query 与 125 个 index 需要可审计覆盖，又不能把 PostgreSQL 兼容层带入实现 | 每个 Phase 更新本表的实际落点；Phase 3 以 sqlc 与 baseline tests 证明逐项完成 |
| 2026-07-25 | 唯一 driver 固定为 `mattn/go-sqlite3 v1.14.48`，FTS5 使用统一 build tag | CGo 路径同时通过当前 macOS 与 Linux/arm64，且满足 River `database/sql`、FTS5 与静态 vector extension | build/package/CI 必须保持 CGo 和 `sqlite_fts5`；禁止引入第二 driver fallback |
| 2026-07-25 | vector 固定为 `sqlite-vec` CGo binding v0.1.6 的 exact `vec0` | 768D/512D insert/delete/top-k 与静态注册均通过；正确性风险低于迁移期引入 ANN sidecar | authoritative embedding 仍存普通表；规模 benchmark 与 ANN 决策保留为 Hardening |
| 2026-07-25 | sqlc JSON 字段使用 `dbtypes.JSON`，时间使用 Unix microseconds `dbtypes.Timestamp` | `STRICT TEXT` 会拒绝 `json.RawMessage` 产生的 BLOB，且时间需要跨 driver 的唯一语义 | Phase 3 overrides 复用集中 Scanner/Valuer，不允许 query-local cast/fallback |
| 2026-07-26 | Docker release 固定为一个同时服务 SPA + API 的 application image 与两个显式 bind mounts | 删除 DB/Caddy 必需 service，并让 catalog ownership、备份与恢复边界可见 | `/data/storage` 只放媒体；`/data/app-state` 放 SQLite catalog 与 private state；TLS 变为外部可选 proxy |
| 2026-07-26 | River v0.24 使用只保留 reachable production packages 的本地 MPL-2.0 fork | 上游 production `testsignal` 经 test helper 将 pgx 链接进最终 binary；升级会扩大本次迁移风险 | fork 保持窄差异、携带上游 license/说明并由 package/architecture guard约束；后续升级应优先删除 fork |
| 2026-07-26 | 所有 UUID 与随机数据由 Go 生成，不使用 SQLite `randomblob()` | required-column contract 与 transaction orchestration 应在业务边界显式可测，数据库侧随机表达式也会绕过生成 DTO | query params 必须包含 ID；schema/architecture tests 防止遗漏和数据库侧随机 fallback |
| 2026-07-26 | 活动 catalog 只能由 owning runtime 打开，跨 container mount 的 host 工具不属于支持的诊断路径 | SQLite WAL/VFS locking 语义不能假设跨 Linux container 与 macOS host 一致 | live 检查使用 Online Backup snapshot；离线检查先 graceful stop，并在 owning runtime 内完成 |

## Surprises & Discoveries

- Phase 0 的迁移前 `make server-test` 在本机完整通过；后续没有“已知既有 Server gate failure”可豁免。
- 当前 PostgreSQL 泄漏远超 DB package：排除 generated repo 后仍遍布 app、Agent、API、cloud、processors、queue、search、service、sourcing 和 storage。Phase 4 必须按 app wiring 从外向内迁移，不能只替换 connection package。
- active plan 输入最初以未跟踪的 `sqlite-experimental-plan.md` 存在，但 goal 指定 `active/sqlite.md`；Phase 0 已将其归位并作为 living plan 纳入版本控制。
- `json.RawMessage` 的 `driver.Value` 是 `[]byte`，在非 STRICT SQLite 中容易被宽松 affinity 隐藏；`STRICT TEXT` 在首次 sqlc round-trip 直接暴露 BLOB/type mismatch。集中 JSON type 比放宽 schema 更符合不变量。
- Linux builder 最初能编译 Server binary，但在 image 内重新编译 spike test 时因缺少 `sqlite3.h` 失败；builder 增加 build-only `libsqlite3-dev` 后，Linux/arm64 binary 与全部 runtime spike 均通过。应用不依赖 runtime SQLite CLI 或动态 vector extension。
- OrbStack 一度报告 running 但 Docker socket `_ping` 无响应；重启 OrbStack 后恢复，Lumen Hub 进程未退出。这是本机 container runtime 状态，不是 SQLite 失败。
- macOS SDK 会对 sqlite-vec 使用的 `sqlite3_auto_extension` 发出 deprecated warning；实际 `vec_version()`、多 database open 和 exact KNN 在当前平台通过，风险已进入 Hardening backlog。
- root commit hook 当前从 repository root 执行 `vp staged`，但 `staged` config 只存在于 `web/vite.config.ts`，因此任何 root commit 都会在检查文件前失败。Phase 1 已手工通过 `gofmt -d`、`git diff --check`、`go mod tidy -diff` 和对应完整 gates，并以 `--no-verify` checkpoint；Phase 10 必须修正 hook 安装/working directory。
- `node:24-trixie-slim` 没有 system CA bundle，Vite+ 即使本地 build 也会初始化 HTTP client并 panic；web builder显式安装 `ca-certificates` 后生产 bundle通过。
- fresh host bind directory通常是 `0755`；DB runtime正确拒绝 group/world-readable SQLite parent。Docker entrypoint因此必须在降权前把 app-state收紧为 `0700`，不能只检查“UID 10001可写”。
- Desktop first-launch smoke 发现 SQLite baseline 的 `bootstrap_phase` CHECK 少了应用实际写入的 `db_rotated`；旧单元测试未覆盖真实 bootstrap sequence，现由真实 first/second launch 与 baseline test共同守护。
- `go mod tidy` 不足以证明最终 package 不含旧后端：River v0.24 的 production helper把 pgx拉进实际 Desktop binary。最终 bundle的 Go metadata/string/linked-library audit促成了窄 production fork，并要求 Phase 10 增加 repository architecture guard。
- host web依赖恢复时 sharp optional native postinstall失败，但跳过 install scripts 后同一 lockfile 的 production SPA build成功；这暴露的是本机依赖安装路径差异，未通过修改 lockfile 或降低 build gate隐藏。
- SQLite baseline required-column contract 在 broaden 后发现 registration session、duplicate group、Agent pin、share link 与 location cluster 的 UUID insert 都依赖了 PostgreSQL 时代的隐式/default 行为。统一改成 Go UUID 后，guard 同时排除了数据库侧 `randomblob()` fallback。
- sqlc 对同一 statement 中 slice expansion 与固定 numbered parameters 的组合会生成位置错误但能编译的代码；JSON collection/typed CTE 与 generation guard 把这类问题从 runtime 移到 CI。
- 在 macOS host 上直接用 `sqlite3` 打开 OrbStack/Linux bind mount 中仍活动的 catalog 会跨 VFS/WAL boundary 干扰数据库，并曾导致下一次启动报告 malformed。隔离目录重做后，所有检查只在 graceful stop 后由 container runtime执行，三次 quick check 与 identity persistence 均通过；文档现在明确禁止该诊断方式。
- Windows CI 证明 macOS/Linux 的绝对路径、tool fixture 和 fsync 测试不能代表 drive-letter URI、`.exe` resolution、DACL 或 Windows handle semantics：`url.URL` 直接接收反斜杠 path 会生成 SQLite 拒绝的 authority，`os.FileMode` 在 Windows 不表达 ACL privacy，read-only handle 不能 `Sync()`，且 directory fsync 没有 Windows 等价物。跨平台 URI helper、production tool suffix resolution、已有 `fsprivacy` protected-DACL path、read-write file flush 与 write-through rename 现在由 Windows Server/Desktop job 实测。

## Outcomes & Retrospective

- 最终 SQLite driver 是 `mattn/go-sqlite3 v1.14.48`，统一使用 `sqlite_fts5` build tag、单 writer pool 与静态 sqlite-vec 注册；仓库没有第二 driver 或 runtime extension fallback。
- 最终 River 是 v0.24.0 `riversqlite`，应用 transaction 与 queue insert 共享 `*sql.Tx`。为消除 production binary 中不可达但实际链接的 pgx，当前使用只包含 reachable production packages 的窄 MPL-2.0 local fork。
- 最终 vector 方案是 sqlite-vec CGo binding v0.1.6 exact `vec0`。768D semantic 与 512D face authoritative vectors 保存在 ordinary STRICT tables，派生 `vec0` 可重建；ANN threshold 留给规模 benchmark。
- fresh baseline 创建 52 个 ordinary application tables、6 个 virtual tables 和 119 个 explicit indexes。63 个 query source files 生成 468 个 named sqlc queries；Phase 0 inventory 的 125 个 PostgreSQL-era indexes 均被保留、转换为 FTS/vector结构或按 engine-only 语义明确删除。
- 已删除旧 PostgreSQL migrations、pgx/pgtype/pgconn、pgvector、riverpgxv5、lib/pq、PostgreSQL migrate driver、DB password rotation/secret initializer、Docker DB service/volume/port/secret、旧独立 web/Caddy service，以及 Desktop database supervisor、resources、license 与发布 artifact。
- final ad-hoc signed macOS app 为 321,456 KiB，main binary 为 80,874,016 bytes；final Linux/arm64 单体 runtime image 为 318,888,485 bytes。迁移前没有同条件 artifact，因此不虚构体积改善百分比。
- 没有进行严格 idle-memory 或迁移前后 startup benchmark。唯一可报告的 fresh local container 观察约为 43 ms 到 listener ready（ML disabled）；小型 browser fixture 约 523 ms 到首屏、video semantic cases 约 3.5–5.6 s。exact vector correctness 已验证，但这些数据不代表大型 library 性能。
- P0 可靠性门槛通过：完整 Server/Web/Desktop/build/docs/generation/architecture gates、Linux image audit、macOS signed bundle audit、restart/recreate identity + integrity、以及 12/12 fresh browser E2E。长时间 stress、完整 kill/fault matrix、规模 vector benchmark 与完整跨平台 runtime E2E 明确 Deferred 到 Hardening backlog。
- 已知限制是 SQLite 单 writer、活动 catalog 必须位于 machine-local state、不能跨 VFS/host container 热打开或复制、exact vector 的规模阈值尚未决定、River narrow fork 待 upstream closure 修复，以及 Windows installer 与非 CI architecture runtime 尚未完整实测。
- 建议在通过远端 CI 后将 SQLite 合并为本项目默认架构：destructive fresh baseline、唯一 runtime、事务性 River、snapshot/restore、Desktop、Docker 与核心产品路径已经形成闭环。该建议不等同于提供 PostgreSQL 数据转换路径。

### Critical Files for Implementation

- `server/app/app.go`
- `server/config/config.go`
- `server/internal/db/db.go`
- `server/internal/db/migration.go`
- `server/internal/sourcing/materializer.go`
