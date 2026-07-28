# Handoff: Logical Media Browse & Filtering（从 Step 11 起接手）

> 交接对象：接手本破坏性 PR 剩余工作（Step 11–16）的 agent。
> 权威计划：同目录 `logical-media-browse-filter-plan.md`（下称 plan）。**本文件只描述现状与剩余工作，契约冲突时以 plan 为准。**
> 分支：`experimental/sqlite`（HEAD `d56abb06`，所有改动均未提交，见 `git status`）。

---

## 1. 一句话背景

browse/filter/search/agent 全部以 **media_item** 为最小用户单位；彻底删除 `filter.raw` / `IsRaw` / `BrowseItem.type="asset"` / `total_assets` / search 的 `stack_mode`。**无兼容层、无迁移**：旧 pins/refs/URL 参数不迁移，SQLite catalog 整体重建（user_version=3）。

## 2. 已完成（Step 1–10，勿重做）

| Step | 内容 | 状态 |
|---|---|---|
| 1 | SQLite baseline 重写、`user_version=3`、`media_item_browse_facts` view + indexes | ✅ |
| 2 | ingest component relation（`InitialMediaRelation` + `AttachAssetToMediaItem`，新文件 `server/internal/db/repo/media_relation.go`） | ✅ |
| 3 | media/stack invariant normalization | ✅ |
| 4 | detection 调度移到 metadata completion | ✅ |
| 5 | SQL query/count 重写（`assets_01~05.sql`、browse facts） | ✅ |
| 6 | service/DTO 重写（`QueryMediaItems`/`BrowseItemDTO`/counts=`total_visible`+`total_media_items`+`total_files`） | ✅ |
| 7 | search hydration 改 media-item 粒度、`SearchAssetsRequestDTO` 删 `stack_mode`（swaggerignore） | ✅ |
| 8 | Agent filter：`asset_filter.go`/`pins.go`/`authorized_library.go` 改 composition/stack 输入、snapshot=primary asset ID、summary `→ N media items` | ✅ |
| 9 | sqlc regen + `make dto` 成功，`web/src/lib/http-commons/schema.d.ts` 已更新且验证符合 plan | ✅ |
| 10 | 前端 browse item identity：`types.ts`/`browseItems.ts` 重写为 `media:`/`stack:` ID | ✅ |

### 验证命令（接手后先跑一遍确认基线）

```bash
# server 全量编译（当前全绿）
cd server && CGO_LDFLAGS_ALLOW=-Xpreprocessor CGO_CFLAGS_ALLOW=-Xpreprocessor \
  go build -tags=sqlite_fts5 ./... 2>&1 | grep -v "deprecated\|sqlite3.h\|cgo-gcc-prolog\|^#"

# web type-check（当前 app 代码全绿，仅剩 3 个测试文件报错，见 §5）
cd web && vp check --no-fmt --no-lint
# 快速列出错误文件: vp check --no-fmt --no-lint 2>&1 | grep -E "^\s+,-\[" | sort -u
```

质量门：`make server-test` / `make web-test`（勿绕过 Makefile 环境）。契约再生成：`make dto`。

## 3. 已定契约（后续步骤必须遵守）

### 3.1 后端 DTO（schema.d.ts 已生成，勿改前端 schema，改 backend 注解 + `make dto`）

- `AssetFilterDTO`：`media_item?: { composition?: "contains_raw"|"jpeg_raw"|"raw_unpaired"|"no_raw" }`、`stack?: { membership?: "stacked"|"unstacked", kinds?: ("burst"|"manual")[] }`；**无 `raw`**。
- `BrowseItemDTO`：`type: "media_item"|"stack"`，`id` 形如 `media:<uuid>` / `stack:<uuid>`；counts：`total_visible`/`total_media_items`/`total_files`；**无 `total_assets`**。
- `AssetQueryRequestDTO` 仍保留可选 `stack_mode`（collapsed/expanded，browse presentation 合法）；`SearchAssetsRequestDTO` **无** `stack_mode`。
- 后端校验：`membership=unstacked` 与非空 `kinds` 互斥（返回 InvalidArgument）。
- stack 行的 `cover.stack` 带 `StackCover:true` + `StackSize=len(Members)`；media_item 行的 stack preview 只有 `StackID/StackKind` 无 size（expanded 行无 overlay badge——Step 12 重做 badges）。

### 3.2 前端类型（`web/src/features/assets/types.ts`，Step 10 已定型）

```ts
type BrowseItemId = `media:${string}` | `stack:${string}`;
type BrowseStackKind = "burst" | "manual";
interface MediaCompositionFacts { componentCount; hasRaw; hasJpeg; hasEdited; hasLiveMotion; }
interface BrowseMediaItem { type:"media_item"; id; mediaItemId; asset: Asset; composition?; bestTsMs?; }
interface BrowseStackMemberRef { mediaItemId; primaryAssetId; }
interface BrowseStackItem { type:"stack"; id; stackId; stackKind; representative: Asset;
  assets: Asset[]; members: BrowseStackMemberRef[]; matchedMembers: BrowseStackMemberRef[]; bestTsMs?; }
```

- DTO→BrowseItem 转换在 `web/src/features/assets/model/browseItems.ts`（`createBrowseItemsFromBrowseItemDTOs`）。**graft 技巧**：把 `cover.stack`/`media.stack` 预览嫁接到 primary asset 上（`{...primaryAsset, stack: cover?.stack ?? primaryAsset.stack}`），gallery 继续从 `asset.stack` 读 overlay。
- gallery/selection 消费者已适配（JustifiedGallery/SquareGallery/AssetBrowser/useBrowseSelectionContext/useAssetActions/usePinAssetsView/useRepositoryAssetCount）。

### 3.3 Agent（Step 8 已定型）

- `AssetFilterInput`：`Composition string` / `StackMembership string` / `StackKinds []string`；snapshot 每 media item 一个 primary asset ID；summary 格式 `filter(...) → N media items`。
- `pins.go` 的 `filterReplayPayload` 字段：`composition`/`stack_membership`/`stack_kinds`（旧 payload 无这些字段 → 过滤不生效，可接受，不迁移）。

## 4. 剩余工作

Step 11–16 已全部完成（2026-07-27），细节见下。剩余的唯一未执行项是 **E2E 与性能基准**，见 §4.1。

| Step | 内容 | 状态 |
|---|---|---|
| 11 | 前端 filter state / URL / UI 重写：`AssetUserFilter` 换 `media_item`+`stack`、`FilterDraft` 扁平化（删 `filterEnabled` 与全部 `*Enabled`）、`SectionShell` 改「标题 + active 时清除」、新增 `CompositionSection`/`StackSection`、URL 参数 `composition`/`stack_membership`/`stack_kind`、header `ActiveFilterChips` | ✅ |
| 12 | badges：`MediaCompositionBadges`（RAW / JPEG+RAW / LIVE / EDITED，纯文字 badge，绝不借用 stack icon）；`StackedThumbnail` 按 `stackKind` 换图标（burst=`GalleryHorizontalEnd`，manual=`Layers`）并在部分命中时显示 `2 / 3` | ✅ |
| 13 | event/date grouping 改为直接对 BrowseItem 分组（`groupBrowseItemsBySort` 不再经 `asset_id` 往返，避免同 representative 的两行互相顶掉），分组键仍来自 representative 的日期 | ✅ |
| 14 | 真实 SQLite browse 矩阵 `server/internal/db/browse_matrix_test.go`（composition × membership × kind × collapsed/expanded × list/count parity × 分页 × 部分命中 × cover fallback）＋ 7 条 invariant；handler contract 测试全量重写 | ✅ |
| 15 | 前端测试按新契约重写（`filter`/`browseRouteState`/`filterState`/`legacyBrowseStateMigration`/`browseItems`/`StackedThumbnail`/`AssetsPageHeader`），新增 `MediaCompositionBadges.test.tsx`；Agent 侧新增 `asset_filter_test.go` 与 pins replay payload 测试 | ✅ |
| 16 | `scripts/check-architecture.sh` 新增三条 browse 守卫（见 §4.2）；`make server-test` / `make web-test` / `make dto` 全绿且生成物无漂移 | ✅ |

### 4.1 E2E：已跑通（12/12），并抓到一个真实产品 bug

`docker-compose.e2e.yml` 全套跑过，四个 suite 全绿：`@smoke` 4、`@auth-hardening` 5、`@video-regression` 2、`@backup-recovery` 1。

**抓到的真实 bug（本 PR 引入，单测与 tsc 都没拦住）**：`useAssetsSearch.ts` 仍在 search 请求体里发 `stack_mode: "collapsed"`，而 Step 7 之后后端对带该字段的 search 请求一律返回 **400** —— 也就是**应用内搜索全挂**。已删除该字段。

为什么类型系统没拦住（**重要，别再踩**）：`dto.SearchAssetsRequestDTO` 里确实没有这个字段，但请求体是在 `useMemo<T>(() => ({…}))` 里构造的，**TypeScript 在这个位置不做 excess property check**。我实测往里塞一个 `totally_bogus_field: 1` 也照样编译通过。所以「OpenAPI 生成类型 = 契约被强制」在这类调用点**不成立**，必须靠 grep 守卫兜底（已加，见 §4.2 第 4 条）。

**同时修的 E2E 测试陈旧问题**（不是产品 bug）：`upload` / `auth-session` / `video-semantic` / `video-semantic-regression` 四个 spec 都在读旧的 `item.asset`，而新契约是 `item.media_item.primary_asset`；后两个还在给 search 发那个字段。这些 spec 各自**手写**了 `BrowseResponse`/`QueryResponse` 局部类型，配合 `api<T>()` 里的 `return body as T`，把契约漂移完全遮住了。现已全部改成从 `schema.d.ts` 派生（`components["schemas"]["dto.…"]`），实测把旧写法塞回去会**编译失败**。

**跑法（注意状态污染）**：

```bash
vp run e2e:up            # 每次都重建卷 = 干净 catalog
make web-browser-test    # @smoke
make web-auth-hardening-test
make web-video-semantic-test
make web-backup-recovery-test
vp run e2e:down
```

`@video-regression` 断言的是**精确的索引总数**，所以它**必须跑在干净 catalog 上**；连跑两次而中间没有 `e2e:down && e2e:up`，第二次一定假失败——我第一次就是这么被误导的。

### 4.1.1 查询计划守卫：已加（代替性能基准的第一层）

`server/internal/db/browse_query_plan_test.go` + `internal/db/repo/browse_query_plans.go`。

**思路**：性能退化里真正致命的那种（join 从索引查找退化成全表扫描，O(page) 变 O(catalog)）是**计划变化**，不是耗时变化。计划断言确定性、机器无关、亚秒级，正好绕开「单机跑基准全是噪声」的问题。

- `repo.BrowseQueryPlanTargets` 把 5 条热路径查询的**生成 SQL 原文**导出（两条 list + 三条 count），测试直接对它跑 `EXPLAIN QUERY PLAN`，不复制 SQL，不会漂移。
- fixture 播 2000 个 media item，**按真实分布**（1/6 是 JPEG+RAW、1/20 live photo、约 4% 在 3–8 张的 burst stack），然后跑 `ANALYZE`。分布比数量重要得多：全是「单 component 未分组 JPEG」的话 `sqlite_stat1` 会记录生产中不存在的选择性，planner 选出的计划就没有参考价值。
- 参数全部绑 NULL：SQLite 规划时不做 parameter sniffing，访问路径与真实调用一致。
- 断言用**正向别名清单**（`hotBrowseJoinAliases`，13 个别名 → 5 张基表）而不是「不许出现任何裸 SCAN」，因为 CTE / view / `json_each` 的裸 SCAN 是合法的（`filter_params`、`eligible`、`facts`、`paged`、`stack_matches`、`cover_facts`…）。清单配一条**完整性检查**：清单里的别名如果在所有计划里都不出现，说明别名被改名了、守卫已悄悄失效 → 直接报错。

两种失败都已注入验证：删掉 `idx_media_item_assets_item*` → 5 处报 `SCAN mia` 等；往清单塞一个不存在的别名 → 报 "no longer appears"。

**当前实测结论**：所有基表访问都走索引，裸 SCAN 只出现在 CTE 上。已知特征（**没有**断言，属于设计固有）：
- 视图 `media_item_browse_facts` 会 `USE TEMP B-TREE FOR GROUP BY`；
- 排序会 `USE TEMP B-TREE FOR ORDER BY`（`COALESCE(taken_time, upload_time)` 上没有覆盖索引）——这是深分页真正的成本所在，将来要优化就从这里下手；
- person / album / tag 这些过滤器在计划里表现为 correlated scalar subquery，看着吓人，但 SQL 是 `narg IS NULL OR EXISTS(...)`，参数为 NULL 时运行期短路，不会真的每行执行。

### 4.1.2 未执行：性能基准（耗时数字）

未采集（用户明确说暂不做大规模资产测试）。真要做时的建议：browse 查询的规模变量是**行数**不是字节数，用合成行即可，不需要真实照片；用 `go test -bench -count=10` + `benchstat` 做 before/after 相对比较，别把 ms 阈值写进 CI；重点测 offset 递增（0/1k/10k/50k）的曲线和冷缓存首屏。吞吐类（导入/缩略图/ML）用已有的 `server/tools/uploadbench`。

### 4.2 architecture guard 的边界（重要）

`scripts/check-architecture.sh` 现在多跑四条 grep 守卫，全部已用「注入回归 → 确认报错」验证过：

1. **retired RAW filter**：`IsRaw` / `filter.raw` / `raw?: boolean` / `raw: true|false` / `params.get("raw")`。白名单：`dbtypes.PhotoSpecificMetadata.is_raw`（EXIF 事实）、生成 SQL 的 component fact 列、`legacyBrowseStateMigration*`（读旧持久化，`raw` 是被**丢弃**的输入，不是过滤器）。
2. **search stack_mode**：`asset_search_fused.go` 里出现 `StackMode` 即失败。
3. **retired total_assets**：`server/internal/api/dto` 里出现 `json:"total_assets"` / `json:"results_total_assets"` 即失败。（face/OCR 进度统计里的 `TotalAssets` 是**另一个概念**，不在检查范围内。）
4. **前端 stack_mode 只许出现在 `useAssetsList.ts`**（browse 的 collapse/expand），`web/src` 其它任何位置出现即失败。这条是 E2E 抓到那个 400 bug 之后补的，因为 tsc 在 `useMemo<T>(() => ({…}))` 里不做 excess property check，**类型系统在这里帮不上忙**。写注释时注意别在 `web/src` 里出现这个字面量，会自己触发守卫。

**故意没有加**的守卫：`MemberAssetIDs`/`CoverAssetID`/`cover_asset_id` 这类退役字段。它们与 album/folder/tag 的合法封面字段重名，grep 会产生几十条误报；这些字段的消失已由 Go 与 TypeScript 类型系统强制，grep 守卫只会制造噪音。

**注意**：`git grep -E` 在本机（macOS）**不支持 `\b`**，写守卫时不要用词边界，改用 `(^|[^[:alnum:]_])` 之类的显式包围。

### 4.3 Step 11–16 落地时的几个偏离/决定

- **Studio picker：composition 过滤整个删掉**（不是换成 `no_raw`）。旧的 `raw: false` 本身就已经过时：编辑器的工作源走 `getExportUrl(assetId, {format:"jpeg"})`（`StudioEditor.tsx:96`），而 export handler 用 `imagesource.OpenPhoto` 把 RAW 解析成内嵌预览（full render 兜底）——**RAW 早就能编辑了**。现在是 `initialFilters={{ type: "PHOTO" }}` + `lockedFields={["type"]}`：顺带修好了原来 `type` 被 lock 但没有值（= 锁住却不过滤，视频也会出现在「选一张照片来编辑」里）的问题。
  - 唯一仍依赖原文件的是 `getExifSource`（border 工具的 EXIF 文案），它 fetch 原始字节且失败返回 null，本来就是 best-effort，不影响编辑。
- **collapsed 行的 cover 不随过滤变化**：SQL 的规则是「designated cover 只要可见就用它，否则退到最低 position 的可见成员」——可见 = 未删除，**与是否命中过滤无关**。部分命中由 `matched_items` 表达（前端 `matchedMembers`），这正是 `2 / 3` badge 的数据来源。Step 14 一开始按「cover 退到命中成员」写断言，是错的，已改。
- **`hasActiveLockedFields` / `countEnabledFilters` 已删除**：扁平 draft 后没有消费者，`enabledCount` 直接用 `countActiveAssetUserFilters(appliedFilter)`。
- **`SearchAssetsRequestDTO.StackMode` 保留但被拒绝**：字段仍可绑定，handler 见到就返回 400（`rejectSearchStackMode`）。这是**有意**的显式拒绝，不是遗留。

## 5. 陷阱与经验（务必读）

1. **`make dto` 是唯一契约通道**：任何 DTO/注解改动后必须重跑；前端出现 `as` cast 或 `Record<string, never>` 即契约失败，回后端修。
2. **server 编译必须带 CGO env + `-tags=sqlite_fts5`**（见 §2 命令），否则 media 依赖链接失败。
3. **handler 包曾因依赖 broken 的 agent/core 而整包跳过 type-check**——每次后端改动后跑全量 `go build ./...`，不要只 build 单包。
4. `vp fmt` 会写文件；生成物（`src/wasm/**`、schema.d.ts、`doc.md`）不许手改。
5. i18n 严格 extract-then-fill（见 Step 11 第 11 条）。
6. 前端 `raw` 残留：现在只剩无关局部变量（`assetGroups.ts` 的 `const raw = key.slice(...)`、`MediaViewer.tsx` 的 `const raw = searchParams.get("t_ms")`）、`legacyBrowseStateMigration` 的旧持久化输入，以及 `MediaViewer.test.tsx` 里名为 "raw" 的 fixture asset。这些都是合法的，`check-architecture.sh` 已覆盖真正的回归形态。
7. `AssetQueryRequestDTO.stack_mode` 保留是**有意的**（browse collapse/expand presentation），不要顺手删。
8. 旧 agent pins replay payload 宽松处理（无新字段则过滤不生效）是**有意的**，不要加迁移。
9. commit 约定：`feat(assets): …` / `fix: …` 等；本 PR 所有改动尚未 commit，是否 commit 由用户决定，**勿主动 commit**。

## 6. 当前状态与接手建议

**全部质量门已绿**（2026-07-27）：`make test`（server + web）、`make dto`（生成物幂等无漂移）、`scripts/check-architecture.sh`，以及 E2E 四个 suite 12/12。所有改动**仍未 commit**，是否 commit 由用户决定。

剩余可选项：性能基准数字（§4.1.2）。本文件与 plan 已移入 `exec-plans/completed/`。
