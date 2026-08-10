# 流明集（Lumilio Photos）存储位置与资源库生命周期 ADR 集

> 决策状态：已采纳（Accepted）  
> 实现状态：完整（Complete）；12 项 ADR 均已通过逐项独立实现复审。  
> 决策日期：2026-08-08  
> 适用基线：`Lumilio-Photos-5163800e82be`  
> 目的：关闭《Storage Location And Repository Lifecycle》中的 12 组 Open Questions，并形成一套可以直接指导 Server、Desktop、Web、数据库迁移和文档落地的统一合同。

## 结论先行

实现保留了计划的核心方向，并补上四条不可再模糊的上位规则：

1. **Docker 与无桌面宿主的独立 Server 在首个正式版本中只有一个已配置存储位置。** 官方容器路径固定为 `/data/storage`；管理员可以把多个宿主目录分别挂载到它的直接子目录，每个子目录承载一个资源库。这些子挂载不是独立 `.lumilioroot` 身份。
2. **资源库显示名称与磁盘目录名彻底解耦。** 显示名称可修改；存储目录只在显式“重新定位”时变化。创建请求使用 `root_id + directory_name + name`，绝不再用 `name` 隐式推导唯一目录，也不接受任意绝对路径。
3. **所有新建、打开和恢复后的资源库都必须是某个存储位置的直接子目录。** 禁止任意深度后代和嵌套资源库；符号链接、junction 或别名不能绕过边界。Docker 子 bind mount 仍可作为直接子目录使用。
4. **缺失标记或离线挂载一律按“不可用”处理，不按“文件已删除”处理。** 在身份和可达性恢复前，不运行权威缺失扫描，不自动重建标记，不切换写入目标，也不清理目录数据。

推荐的逻辑模型如下：

```text
Desktop
└── Lumilio instance
    ├── Default Storage Location (.lumilioroot)
    │   ├── primary (.lumiliorepo)
    │   └── Repository A (.lumiliorepo)
    └── External Storage Location (.lumilioroot)
        └── Repository B (.lumiliorepo)

Docker / standalone Server v1
└── /data/storage (.lumilioroot)
    ├── primary (.lumiliorepo)
    ├── family-disk/  -> optional host bind mount; Repository “家庭照片”
    └── archive-2024/ -> optional host bind mount; Repository “旧照片归档”
```

这里的 `family-disk` 与“家庭照片”没有相等约束。前者是稳定的存储目录，后者是可修改的产品名称。

## 调研归纳

这组决策不是照抄某一款产品，而是吸收了几类反复出现的经验：

- Immich 和 PhotoPrism 都要求容器只能使用容器内可见路径，并允许把多个宿主目录映射为某个父路径下的子目录；相应用户讨论也反复出现“环境变量、宿主路径、容器路径和库名称究竟是什么关系”的困惑。[R1][R2][R3][R4]
- Nextcloud 明确把用户可见名称与后端挂载配置分开，并用在线、警告、错误状态呈现外部存储；这说明产品名称不应兼任路径协议。[R5]
- Immich 用户报告离线网络挂载会导致资产重新扫描、缩略图、转码和机器学习结果重建；Jellyfin 也明确警告在存储离线时运行维护任务可能从目录中移除项目。[R6][R7]
- Syncthing 使用目录标记来区分“磁盘真的为空”与“磁盘没有挂载”，缺失标记时停止同步；这与 `.lumilioroot` / `.lumiliorepo` 的安全职责高度一致。[R8]
- Lightroom 的用户心智是“离线仍可见、查找缺失文件夹、更新位置”，并把“从目录中移除”与“删除硬盘文件”严格分开。[R9][R10]

这些证据共同支持：**单一容器根可以成立，但名称与路径必须分离；离线必须 fail closed；恢复应采用“重新连接/重新定位”的目录心智；移除默认不触碰磁盘。**

## 部署能力矩阵

| 能力 | Desktop 部署 | Docker / 独立 Server v1 |
|---|---|---|
| 已登记存储位置 | 默认位置 + 多个外部位置 | 仅一个配置中的默认位置 |
| 创建资源库 | Web 持有任务；已有位置内直接创建；需要新路径时交给 Desktop 本机授权 | Web 在默认位置的直接子目录中创建 |
| 打开现有资源库 | Web 发起；Desktop 原生目录选择器授权并执行 | Web 只列出默认位置下未登记的直接子目录候选 |
| 添加外部存储位置 | 支持，必须本机确认 | 不支持；宿主挂载与 `storage.path` 由运维配置管理 |
| 重新定位资源库 | 支持，Desktop 选择新位置 | 仅支持默认位置内已经由管理员移动好的直接子目录 |
| 登记资源库副本 | 支持，需本机授权与显式确认 | 仅支持默认位置内的候选副本 |
| 重新定位默认存储位置 | 修改 Desktop runtime intent/TOML，验证后重启 | 修改宿主挂载或配置，验证后重启；不是 Web 运行时操作 |
| 从流明集中移除 | Web 管理员操作，磁盘文件保留 | 同左 |
| 物理删除原始文件 | 不提供 | 不提供 |

---

## ADR-001：工作流所有权与部署矩阵

**状态：Accepted**
**实现状态：完整。** `host_action` 已持久化并支持崩溃恢复、actor 隔离和跨会话发现；Desktop 仅保留受限的本机 capability broker 工作流。

### 考虑的方案

1. 所有操作都放到 Desktop Control Panel；实现简单，但远程管理员和 Docker 的体验分裂。
2. 让 Web 直接提交宿主绝对路径；会破坏现有安全边界，也无法证明本机用户授权。
3. Web 持有一个可恢复任务，Desktop 只提供受限的本机能力授权。

### 决策

采用方案 3。**Web 是唯一可见任务及恢复状态的所有者；Desktop 是本机文件系统 capability broker，不是第二套业务工作流。**

Web 创建持久化的 `host_action` 票据，至少包含：操作种类、发起管理员、实例与会话、请求摘要、`expected_version`、过期时间、取消状态和一次性 nonce。Server 通过新的窄型 in-process control 把待处理票据投影给 Desktop；Desktop Settings WebView 仍只调用 Wails/宿主控制面，不调用 Server HTTP API。

本机用户看到发起者和操作目的，显式确认后使用原生目录选择器。Desktop 执行一次操作并提交 receipt；Web 通过普通 API/SSE 观察结果并在原任务中继续。原始宿主路径不是共享 HTTP API 的输入。

票据只支持预先定义的少量动作：`authorize_storage_location`、`open_repository`、`locate_storage_location`、`locate_repository`。它不是通用远程文件选择或任意 Desktop RPC。

Docker/独立 Server 不发布 native-host capability，因此 Web 不显示这些动作；它只能操作已配置根中受控的直接子目录。Desktop 未运行、断开或票据过期时，任务保持可见并给出明确原因，不降级为任意路径输入。

### 后果

- 远程 LAN 管理员可以发起请求，但不能单独授予本机路径；必须有 Desktop 所在机器上的本地确认。
- 浏览器刷新和 Server 重启后任务仍可恢复；Desktop 进程内的临时 operation registry 不再是权威状态。
- 当前“Web 创建、Desktop 添加根、Server 独占冲突解决”的分裂被收敛为一个任务模型。

---

## ADR-002：存储位置授权、拓扑与身份

**状态：Accepted**
**实现状态：完整。** 根与资源库使用跨进程 OS 文件锁，恢复目标也在写入前加锁；扫描在嵌套 `.lumiliorepo` 边界报拓扑错误并停止。

### 考虑的方案

1. 允许资源库位于存储位置内任意深度，并在注册根时递归自动发现。
2. 规定资源库为直接子目录，发现只做只读预览，附加必须显式选择。
3. Docker 允许多个由环境变量提供的绝对根路径。

### 决策

采用方案 2，并明确部署差异：

- Desktop 可登记多个 `.lumilioroot`。
- Docker 与独立 Server v1 只有一个配置根；多个物理目录可作为它的直接子 bind mount，但不获得独立根身份。
- 所有新建、打开、复制登记和重新定位后的资源库必须满足：`canonical_parent(repository.path) == canonical(root.path)`。
- 嵌套资源库始终禁止。资源库扫描遇到内部 `.lumiliorepo` 必须报告拓扑错误，不跨越它继续扫描。

当 Desktop 打开一个不属于任何根的 `.lumiliorepo` 时，它的**直接父目录**是唯一合法的新存储位置候选。当前实现要求用户先通过“添加存储位置”显式授权这个直接父目录，再执行“打开现有资源库”；不会把对单个资源库目录的选择静默扩大为父目录授权。若用户希望使用更高层祖先，必须先整理目录，使资源库成为该祖先的直接子目录；产品不自动选择宽泛祖先。

登记存储位置只建立授权和身份，不自动附加子资源库。Server 可以只读检查一级子目录并返回候选列表，但每个“打开现有资源库”都需要显式确认。当前 `associateRepositoriesUnderRoot` 的自动关联只允许作为一次性迁移工具，并采用更严格的验证条件。

路径边界使用规范化后的真实路径：解析已有路径段的 symlink/junction、真实大小写和别名；指向根外的符号链接被拒绝。Docker bind mount 是内核挂载边界，不是符号链接，可以作为直接子目录。

每个根和资源库只允许一个活跃 Lumilio 写入者。根拓扑变更取得根级独占锁，资源库 I/O 取得资源库级 OS 文件锁；无法证明锁语义的网络文件系统不得启用多实例写入。

### Docker 目录 UX

创建表单分离两个字段：

- `资源库名称`：可修改的显示名称；
- `存储目录`：根下单一目录段，无 `/` 或 `\`，只在显式重新定位时变化。

Web 可以默认根据名称建议目录，但不能把两者相等作为不变量。对于已存在的直接子目录，Web 将其分类为：

- 已登记资源库；
- 可打开的有效 `.lumiliorepo`；
- 可用于创建的空且可写目录；
- 非空但无标记的不可用目录；
- 预期挂载缺失或身份错误。

官方 Docker 文档应优先展示 Compose long syntax，并对用户提供的 bind mount 设置 `create_host_path: false`，避免拼错宿主路径时 Docker 静默创建一个空目录。对于“使用已有空挂载目录”创建资源库，Server 还应从容器 mount information 验证该直接子目录确实是挂载点；普通“新建子目录”则不要求是挂载点。

### 后果

- 资源库名称改名不会破坏 Docker mount。
- 直接子目录牺牲少量布局自由，换来可解释的授权、发现、容量、迁移和冲突语义。
- Docker 的子挂载容量按具体资源库路径计算；不能把 `/data/storage` 的容量当作所有子挂载的容量。

---

## ADR-003：移动、复制与身份冲突

**状态：Accepted**
**实现状态：完整。** 重定位会拒绝仍在线的原身份，Primary Repository 不能作为普通副本登记；移动、复制、初始扫描及崩溃恢复均有持久合同。

### 考虑的方案

1. 看到同一 UUID 的新路径就自动更新登记位置。
2. 只根据用户选择区分“移动”和“副本”，不检查旧位置状态。
3. 使用旧位置可达性、标记身份和显式用户意图共同判定。

### 决策

采用方案 3。

`重新定位` 仅在以下条件全部满足时可用：

- 新路径内 `.lumiliorepo` 与登记 UUID 一致；
- 新路径是某个活跃根的直接子目录；
- 旧路径离线、不存在，或不再包含该 UUID；
- 所有扫描、导入、上传、处理器和 RepositoryFS lease 已进入安全屏障。

若两个不同规范路径同时在线且都含相同 UUID，视为**副本冲突**，不提供“重新定位”。用户只能取消，或对所选副本执行“作为独立资源库添加”。同一路径的不同别名在规范化后是 no-op，不构成副本。

资源库可以跨两个已登记存储位置重新定位，但 Lumilio v1 **不移动文件**。用户先在操作系统中完成移动，随后使用“重新定位资源库”更新目录记录。UI 禁止使用会让用户误以为 Lumilio 正在复制文件的“移动资源库”措辞。

整个存储位置的复制在 v1 不支持自动登记。若原位置离线且新位置持有相同 `.lumilioroot`，这是重新定位；若原位置仍在线，这是重复根冲突。批量为根和所有子资源库改写身份需要独立 ADR，不在本轮实现。

登记资源库副本时：

1. 为副本写入新的 `.lumiliorepo` UUID；
2. 把现有 `.lumilio` 的活动私有状态移动到 `.lumilio/recovery/copied-from-<old-id>/` 下保存，不直接删除；
3. 创建干净的活动 staging、派生文件、索引和临时目录；
4. 不自动复用当前按旧 asset UUID 关联的 sidecar、缩略图、转码、人物、向量或队列状态；
5. 创建新目录记录后自动安排完整初始扫描。

普通媒体树和 `inbox/` 保持原样。UI 必须说明：这是新的资源库身份，不是同步关系、备份链接或数据库克隆；目录内仅存储在旧目录记录中的相册、人物、描述和历史不保证迁移。

### 后果

- 插回原磁盘时不会因为一次错误“移动”而出现两个合法写入点。
- 副本登记更慢，但避免把旧实例私有状态误当作可携带身份。
- 当前只改写 `.lumiliorepo` 的实现必须升级为有 journal 的多阶段操作。

---

## ADR-004：主资源库与默认存储位置生命周期

**状态：Accepted**
**实现状态：完整。** Web 提供降级恢复路径，启动不会为缺失挂载重建身份；Desktop 默认位置迁移使用 runtime intent、受控重启与 LKG 回滚。

### 决策

默认存储位置和主资源库是引导锚点，不参加普通生命周期操作：

- 默认存储位置不可移除；
- 主资源库不可从流明集中移除、不可登记为副本、不可跨根重新定位；
- 主资源库相对路径固定为 `primary`，显示名称仍可修改；
- 默认位置移动必须修改 Desktop runtime intent/TOML 或 Server 配置，在提交前验证原 `.lumilioroot` UUID，并通过受控重启生效；不复用外部根的普通 relocate API；
- Docker 推荐保持容器内 `/data/storage` 不变，只调整宿主映射。若必须改变 `storage.path`，同样通过配置和重启完成。

首次设置完成后，`bootstrap_phase=ready` 是持久事实，不能因为主资源库临时离线而退回“尚未初始化”。默认位置或主资源库离线时，实例进入 `degraded / storage_recovery_required`：

- liveness、登录、目录状态和恢复 UI 仍可用；
- 活跃普通资源库仍可浏览和管理；
- 依赖主资源库或默认写入目标的操作被阻止；
- 不运行会把主资源库缺失解释为删除的权威扫描。

恢复默认位置后，Server 验证固定的 `primary/.lumiliorepo`。主资源库不允许通过修改目录记录指向任意其他相对路径；若用户在文件系统中改名，UI 指导其恢复为 `primary`。

Create 在派生目标发现 `.lumiliorepo` 时不得隐式 attach。它返回结构化 `existing_repository_found`：

- 未登记的有效身份：转入“打开现有资源库”；
- 已登记于其他路径：转入移动/副本冲突；
- 标记无效：转入修复诊断。

### 后果

- 存储暂时离线不会再次触发新手引导。
- 主资源库规则更严格，但引导、默认上传和灾难恢复都有唯一锚点。
- 当前“Create 遇到标记就自动 AddRepository”的行为必须删除。

---

## ADR-005：`root_id`、迁移与状态表达

**状态：Accepted**
**实现状态：完整，且旧迁移方案已由 generation-5 基线决策取代。** 所有资源库强制归属存储位置；写入前验证父身份，并提供后台/显式协调与全局恢复 UI。

### 决策

最终模式中，**每一个 `repositories` 行的 `root_id` 都为 `NOT NULL`，外键删除行为为 `ON DELETE RESTRICT`。** 离线不是脱离父位置；离线资源库继续保留原 `root_id`。

因为项目仍在 pre-release，不保留永久性的 nullable 逃生口。迁移采用预检门禁：

1. 对现有 `root_id IS NULL` 行，仅在根和资源库都在线、规范路径为直接父子、两类标记 UUID 正确且没有路径冲突时自动关联；
2. 其余行进入 `storage_migration_recovery_required`，Server 不启动 worker 或普通写入 API；
3. Desktop recovery UI 或 Server maintenance CLI 允许管理员选择正确根、重新定位或明确移除目录记录；
4. 所有行解决后重建 `repositories` 表，施加 `NOT NULL` 与 `RESTRICT`。

正常运行中的注册、附加、重新定位或复制绝不创建无 `root_id` 行。所谓“根外资源库”只能作为尚未提交的 recovery candidate 存在，不能进入活动目录。

状态拆成三层，不再用一个自由字符串混合：

- `StorageLocationState`：`active | offline | error | maintenance`；
- `RepositoryReachability`：`active | offline | identity_error | recovery_required | maintenance`；
- `RepositoryActivity`：`idle | scanning | importing | processing | paused`。

UI 的 effective state 由父位置优先决定：根离线时，子资源库显示“存储位置离线”；根在线但资源库标记错误时显示“资源库身份不匹配”；活动状态只在可达状态正常时展示。

协调检查在启动、写入前、显式“重新检查”、挂载事件和低频后台探测时运行。相同路径上的正确标记重新出现时可以自动恢复为 active；路径存在但 UUID 不同、标记缺失或目录变成普通空目录时绝不自动采用。

只有默认位置和主资源库的故障影响全局 degraded 状态；普通资源库离线不会阻止实例处理其他资源库。

### 后果

- 数据库约束与产品不变量一致，不再依赖每个调用点自律。
- 升级前可能要求一次明确恢复，但不会把不确定路径猜成合法父子关系。
- 需要把引导状态与实时存储可达性分离。

---

## ADR-006：原子性、并发、移除与崩溃恢复

**状态：Accepted**
**实现状态：完整。** maintenance/worker 屏障、Cloud 导入并发、队列清理、完整 impact、事务化 reprocess 与所有可达文件系统加目录变更的持久 journal 均已闭合。

### 决策：原子性与屏障

整个存储位置重新定位必须是 all-or-nothing：先取得根级锁和所有子资源库写锁，验证新根及每一个子 `.lumiliorepo`，再在**一个 SQLite 事务**中更新根与全部子路径。当前“逐个移动子行、失败后可续跑”的部分状态合同被废弃。

生命周期操作开始时把目标置为 maintenance：

- 拒绝新的上传、云导入、扫描和处理任务；
- 排队任务暂停或取消并可在操作后重排；
- 运行中任务收到取消信号并在安全检查点释放 RepositoryFS lease；
- 无法及时释放的操作返回 `resource_busy`，不强制改路径或删除记录；
- 根级操作按稳定 repo UUID 顺序取得子锁，避免死锁。

### 决策：持久化 journal 与幂等

仅修改 SQLite 的操作依靠数据库事务，不额外写 journal。任何同时改变文件系统和数据库的操作——创建资源库、创建根标记、登记副本、默认根配置切换——都使用持久化 `lifecycle_operations`：

- `request_id`、操作种类、payload hash、actor、phase、状态、结果和 rollback data；
- 同一 request ID 加相同 payload 返回原结果；不同 payload 被拒绝；
- Server 启动时根据 marker 与 DB 实际状态决定 roll forward 或 rollback；
- Desktop 的 operation receipt 只是该记录的投影，不是唯一事实来源。

### 决策：从流明集中移除

“从流明集中移除资源库”采用**目录清理、磁盘保留**语义，不采用 detached catalog：

- 仅普通资源库可移除；
- UI 先展示资产数、占用空间、相册关系、任务和仅存于目录中的元数据影响；
- 事务中取消/删除该资源库任务，删除其资产、派生目录行、搜索/向量/人物关系和其他 repo-scoped 状态；
- 跨资源库的用户相册本身保留，只移除指向被删除资产的关系；用户创建的空相册不自动删除；
- 最后删除 `repositories` 行；
- 原始媒体、`inbox/`、`.lumiliorepo`、`.lumilio/` 和父 `.lumilioroot` 全部保留在磁盘。

重新打开时可恢复资源库身份、名称、策略以及可重新扫描的原始媒体；标准文件内元数据或未来定义的可携带 sidecar 可以重新导入。若发现上一次目录登记留下的活动 `.lumilio` 私有状态，打开流程先把它移动到 `.lumilio/recovery/reopened-<timestamp>/` 保存，再建立干净活动状态并全量扫描，避免旧 asset UUID 与新目录记录混用。旧 asset UUID 对应的相册、人物、AI 结果、分享、历史和应用私有 sidecar **不保证恢复**，UI 必须直说。

外部存储位置只有在子资源库数为零、没有进行中操作时才能移除。移除根记录也保留 `.lumilioroot` 和所有磁盘内容。

### 后果

- 用户不会把“移除”误解为删除照片。
- 若未来需要无损保留全部目录元数据，应另行设计“归档/分离目录”，不能偷偷塞进 remove。
- 当前简单 `DELETE repositories` 需要替换为有影响预览、维护屏障和显式清理顺序的服务。

---

## ADR-007：创建时选择资源库存储布局

**状态：Accepted**
**实现状态：完整。** 所有 Web 创建入口均提供并提交 `storage_strategy`；默认 `date`，可选 `flat` / `cas`。创建后不提供布局 PATCH。

### 考虑的方案

1. 创建时完全使用 Server 默认布局，不让用户选择；
2. 创建时选择 `storage_strategy`，资源库创建后保持不变；
3. 创建时使用默认值，创建后通过 PATCH 修改布局。

### 决策

采用方案 2。创建资源库的主路径包含：

- 资源库名称；
- 存储位置；
- 存储目录（Docker 预挂载或用户显式选择时出现）；
- 存储布局 `storage_strategy`：`date`、`flat` 或 `cas`。

Web 创建流程必须展示存储布局选择并把选定值随 Create 请求提交。默认选中 `date`，但界面必须解释三种布局对磁盘目录的影响，不能只展示内部枚举值。Server 对省略字段的非 Web 客户端继续应用实例级确定性默认值。

`storage_strategy` 在资源库创建时写入 `.lumiliorepo`，创建后不提供 PATCH API 或设置页修改入口。改变既有布局需要未来单独定义、带容量预检和持久 journal 的重组/迁移工作流，不能通过普通配置更新完成。

`duplicate filename handling` 不恢复到创建表单，由 Server 使用实例级确定性默认值。Cloud source 也不属于创建参数，继续遵循 ADR-008 的创建后独立导入流程。

### 后果

- 用户在创建资源库时做出一次明确且不可在线修改的磁盘布局选择。
- 创建界面必须用用户可理解的名称、目录示例和不可修改提示解释 `date`、`flat` 与 `cas`。
- 不恢复 Repository PATCH 路由，也不需要实现“首次媒体写入后锁定”状态；布局从资源库创建成功起即为固定合同。
- 重复文件处理和 Cloud 不重新进入创建流程。

---

## ADR-008：Cloud 来源的位置

**状态：Accepted**
**实现状态：完整。** Cloud 是创建后的独立流程，绑定 API/UI、多来源模型、远端范围、隔离 cursor、可取消/恢复的持久任务及 staging 所有权均已实现。

### 决策

Cloud source 不是资源库类型，也不是创建参数。它成为资源库创建后的独立“从云端导入”向导。

一个资源库可以没有云来源，也可以先后绑定多个来源；每个连接保存其用户所有者、凭据、远端范围、目标资源库、断点和任务状态。OAuth、远端枚举、导入失败和重试不会回滚或污染资源库创建事务。

创建完成后的下一步页面可以提供“上传文件 / 扫描资源库 / 从云端导入”，但每一项都有独立 operation receipt、取消和恢复状态。

### 后果

- 消除“资源库已创建但 Cloud 导入失败”的半成功歧义。
- 用户心智变为“先有容器，再选择数据来源”，更符合后续多来源扩展。
- 当前创建 modal 中的 Cloud 选项和复合 mutation 应拆分。

---

## ADR-009：最终中英文产品词汇

**状态：Accepted**
**实现状态：完整。** Web、Desktop、OpenAPI、README、用户文档和能力标签均使用统一术语，并由带精确技术语境豁免的全仓门禁保护。

产品实体与字段：

| English | 简体中文 | 用途 |
|---|---|---|
| Repository | 资源库 | 唯一主产品名词，不使用 Library / 图库作同义词 |
| Primary Repository | 主资源库 | 默认位置下固定 `primary` |
| Storage Location | 存储位置 | 带 `.lumilioroot` 的父位置 |
| Default Storage Location | 默认存储位置 | `storage.path` 对应位置 |
| External Storage Location | 外部存储位置 | Desktop 授权的额外位置 |
| Repository name | 资源库名称 | 可修改显示名称 |
| Storage folder | 存储目录 | 根下稳定目录名；改名不随显示名称变化 |
| Container path | 容器内路径 | 仅 Docker 管理和诊断 |
| Mounted folder | 挂载目录 | 仅 Docker 文档和高级 UI |

用户动作：

| English | 简体中文 |
|---|---|
| Create Repository | 创建资源库 |
| Open Existing Repository | 打开现有资源库 |
| Add Storage Location | 添加存储位置 |
| Reconnect Storage Location | 重新连接存储位置 |
| Locate Storage Location | 重新定位存储位置 |
| Locate Repository | 重新定位资源库 |
| Update Location | 更新位置 |
| Add as Separate Repository | 作为独立资源库添加 |
| Remove from Lumilio | 从流明集中移除 |
| Remove Storage Location | 移除存储位置 |
| Check Again | 重新检查 |

UI 不直接显示 `attach`、`relocate`、`register-copy` 等协议 token。“复制资源库”也不作为按钮，因为该操作不复制文件；冲突文案使用“检测到同一资源库的另一个副本”。

任何安全移除确认都固定显示：**“磁盘中的文件将保留；流明集目录中的部分元数据可能无法在重新打开后恢复。”** 只有未来真正删除磁盘内容时才使用“删除”。

---

## ADR-010：容量、文件系统、可写性与风险信息

**状态：Accepted**
**实现状态：完整。** 已实现容量硬预检、低空间暂停与安全恢复、未知流持续采样、实际子挂载容量、跨实例锁、设备风险两阶段确认和诊断产品路径。

### 硬性资格检查

创建、打开或恢复前必须验证：

- 路径存在且是目录；
- 父子直接关系、非嵌套、非重叠和 canonical containment；
- `.lumilioroot` / `.lumiliorepo` 版本与 UUID；
- 所需读写权限；
- 目标未被另一实例锁定；
- 创建目标为空；打开目标具有有效标记；
- Docker 预挂载空目录确实是 mount point；
- 已知写入规模时，剩余空间覆盖预计写入量和安全余量。

不设置一个与操作无关的全局最低容量。已知来源大小时按实际字节预检；未知大小时允许开始，但持续监控空间并在低空间时暂停，而不是写到 ENOSPC 后继续破坏任务状态。

### 主 UI 信息

存储位置或目标选择器显示：

- 在线 / 离线 / 错误；
- 可写 / 只读；
- 可用与总容量；
- 子资源库数量；
- 管理员可见的规范路径。

Docker 子挂载可能位于不同文件系统，因此容量使用选定资源库目录的 `statfs`；根页面不能把不同设备简单相加为一个可承诺容量。

### 警告而非阻止

- 剩余空间偏低；
- 可移动磁盘或网络文件系统；
- 检测到 iCloud、OneDrive、Dropbox 等同步目录；
- 文件锁能力不确定；
- Docker mount 配置与登记指纹变化。

检测到在线占位文件、不可物化文件或锁测试失败时才升级为硬错误。普通“可能由同步软件管理”只要求管理员确认，避免错误检测导致完全不可用。

### 仅诊断信息

文件系统类型、mount source/ID、device/inode、大小写行为、符号链接解析结果、有效 UID/GID、marker UUID、锁持有者和最近协调时间只放在诊断抽屉或支持包中。支持包默认脱敏绝对宿主路径。

### 后果

- 普通用户先看到“可不可以用”，而不是一组底层字段。
- Docker 缺失子挂载不会被误判为空资源库。
- 需要为 Linux 容器实现 mountinfo 检查，并让非 Linux 平台安全降级为 marker + capability 检查。

---

## ADR-011：管理员授权、审计与额外确认

**状态：Accepted**
**实现状态：完整。** 生命周期审计默认持久化 actor、host、request、operation、source、confirmation 与结果；提供管理员历史和递归路径脱敏支持包。

### 决策

所有资源库和存储位置生命周期 mutation 都要求 Lumilio Administrator。涉及新宿主路径的 Desktop 操作还要求本机 OS 用户的显式确认；远程管理员不能替代本机 presence。

持久审计覆盖：创建、打开、添加根、重新连接、重新定位、登记副本、移除、默认位置配置变更和恢复决策。记录至少包括：

- actor account 或 `desktop_bootstrap/desktop_host`；
- host instance、request ID、operation ID；
- 对象 UUID、旧/新位置、结果和失败阶段；
- 时间、客户端来源与确认类型。

本地审计可保存完整规范路径以支持恢复；导出支持包默认脱敏。

确认强度按后果分级：

- 重新检查、同路径重新连接：普通按钮；
- 重新定位离线位置：摘要确认；
- 作为独立资源库添加：必须确认“会产生新身份，不是同步或备份”；
- 从流明集中移除有内容资源库：显示影响计数并要求输入资源库名称；
- 默认位置迁移：显示所有受影响资源库，要求输入默认位置名称并确认重启；
- 物理删除：本版本不存在。

### 后果

- 权限模型同时表达产品管理员授权与本机文件系统授权，两者不混为一谈。
- 可追踪“谁在什么机器上把哪个身份指向了哪里”。
- 不为安全动作增加无意义的重复确认。

---

## ADR-012：未来物理删除工作流

**状态：Accepted — 当前明确不实现**
**实现状态：完整。** 生命周期 API/UI 没有物理删除资源库原始媒体的路径或隐藏参数。

### 决策

本生命周期项目和首个正式版本不提供“删除资源库及其原始媒体”的 API、按钮、隐藏参数或复用 remove endpoint。现有“从流明集中移除”永远是目录级操作。

未来只有在单独 ADR 同时解决以下合同后才重新讨论：

- 精确删除清单与可预览字节数；
- symlink、junction、hardlink、bind mount 和跨文件系统边界；
- trash/quarantine、保留期、恢复和不可恢复阶段；
- 数据库关系与磁盘操作的持久 journal；
- 备份检查、管理员授权、本机 presence 和强确认；
- 部分失败、断电、重试和幂等；
- 原始媒体与仅由 Lumilio 生成文件的不同策略。

在这些条件之前，“未来可能支持”不应进入 UI 或 OpenAPI，以免用户和实现者把安全移除误认为尚未完成的删除功能。

---

## 对当前基线的关键变更

| 当前基线 | 采纳 ADR 后 |
|---|---|
| `resolveRepositoryCreatePath()` 用资源库名称直接生成目录 | Create 显式传 `directory_name`；名称仅作默认建议 |
| Create 发现 `.lumiliorepo` 时隐式 AddRepository | 返回结构化 existing/moved/copy conflict |
| `repositories.root_id` 可空且 `ON DELETE SET NULL` | `NOT NULL` + `ON DELETE RESTRICT` |
| 添加根后自动关联所有 null-root 后代 | 正常流程只返回一级候选；迁移才做严格自动关联 |
| 资源库可为任意 strict descendant | 新合同要求直接子目录 |
| 根 relocate 逐行更新，接受部分完成 | 预验证后单事务 all-or-nothing |
| copy registration 只重写 `.lumiliorepo` | journal + 隔离旧 `.lumilio` 活动状态 + 全量扫描 |
| Desktop 只投影 list/add/attach，冲突被压成 generic recovery | 投影结构化 host-action、conflict、allowed actions 和 receipt |
| Desktop operation registry 是内存事实 | SQLite `lifecycle_operations` 是权威，Desktop 仅投影 |
| RemoveRepository 直接删除 repo 行 | maintenance、影响预览、目录级清理、文件保留 |
| 创建页展示布局、重复策略和 Cloud | 创建页只选择 `storage_strategy`；重复处理使用 Server 默认值；Cloud 变为创建后独立导入 |
| 启动就以“活跃主库”推导是否初始化 | bootstrap 完成与当前 storage availability 分离 |

## 推荐实施顺序

1. **先锁合同与 schema**：目录名称解耦、直接子目录、`root_id` 迁移门禁、状态枚举、bootstrap/degraded 分离。
2. **再做生命周期基础设施**：持久 `lifecycle_operations`、根/资源库锁、maintenance barrier、结构化冲突、原子 root relocate。
3. **完成部署适配**：Desktop host-action 票据；Docker 一级目录候选、mountinfo 检查和 Compose 文档。
4. **实现 UI 任务**：Create、Open、Reconnect/Locate、Add as Separate、Remove impact preview。
5. **最后收敛产品面**：创建时选择存储布局、Cloud 独立向导、统一中英文文案、文档与恢复手册。

其中 1–3 是 release gate；7–9 号产品整理可以在同一 pre-release 周期后半段完成，但不应反过来阻塞底层不变量。

## 研究资料索引

- **[R1]** Docker Docs, *Bind mounts* 与 Compose service volumes：宿主/容器路径、遮蔽已有目录、read-only、long syntax 和 `create_host_path`。
- **[R2]** Immich 官方文档, *External Libraries*（2026-07-27 更新）：容器内 import path、多路径、可达性、删除和 metadata 风险。
- **[R3]** Immich Discussion #6912, *multiple external libraries in Docker*：用户对 env、父路径、子目录和多用户暴露范围的真实困惑。
- **[R4]** PhotoPrism 官方文档, *Volume Mounts*，及 Discussion #1795：单一 originals 父目录下增加子挂载，以及新手配置困难。
- **[R5]** Nextcloud 34 Administration Manual, *External Storage*：显示名称与后端配置分离、在线/警告/错误状态、只读与授权。
- **[R6]** Immich Discussion #17300, *External Libraries will disappear when mount goes offline*：离线后重新扫描、缩略图、转码与 ML 重建成本。
- **[R7]** Jellyfin 官方文档, *Storage*：存储离线时维护任务可能从目录中移除项目，以及网络文件系统锁风险。
- **[R8]** Syncthing FAQ, *folder marker missing*：标记用于区分未挂载与真实删除，缺失时停止工作。
- **[R9]** Adobe Lightroom Classic, *Locate missing photos / folders*：保留离线目录、显示最后位置、重新定位整个文件夹。
- **[R10]** Adobe Lightroom Classic, *Create and manage folders*：Remove 从 catalog 移除但保留硬盘文件，并显示在线状态与可用空间。

## 基线代码证据位置

- `server/internal/storage/repo_provisioning.go`：当前 regular repository 使用 `name` 原样作为目录，并在目标已有标记时隐式登记。
- `server/migrations/000001_sqlite_baseline.up.sql`：当前 `root_id` 可空并使用 `ON DELETE SET NULL`。
- `server/internal/storage/repository_roots.go`：当前添加根会自动关联 strict descendant；根 relocate 接受可续跑部分状态。
- `server/internal/storage/repo_relocate.go`：当前副本登记只改写 marker UUID。
- `server/internal/storage/repo_manager.go`：当前 remove 是直接删除目录行。
- `desktop/internal/runtime/repository.go` 与 `desktop/internal/storage/controller.go`：当前 Desktop 投影不完整且冲突被压平。
- `deploy/compose/*`：当前官方容器基线只暴露 `/data/storage` 与独立 app-state 挂载。
