# Task Agent — 媒体库可视化与管理（决策框架）

> 状态: **draft / 待决策** · 面向: 产品决策者 + 编码代理
> 前置: [`agent-ref-system.md`](./agent-ref-system.md) Phase 1–3 已完成。Task Agent 的底座（ref / 代数工具 / chat / board / pin）**已存在并可用**，本计划不是从零建，是**收口与方向决策**。
> 配套: Discovery Agent 走 [`agent-discovery.md`](./agent-discovery.md)，两份独立、不互相引用细节。
> **本计划当前是决策框架，不是执行清单。** 下方每个决策点（§3）都需要产品侧拍板后，才会落成可执行的 phased checklist。我标了每个决策的「倾向选项」与理由，但**那是建议不是定论**。

## 1. Task Agent 是什么

保守路线的核心：**用户发起查询 → agent 组合代数 → 以多种方式可视化媒体库 / 管理媒体库。** 这条路线：

- 形态被市场验证（query-response task agent）。
- 即时可用、即时变现。
- **不承担**「主动发现」「叙述」「浮出隐含意义」——那些是 Discovery Agent 的活。
- 与 Discovery Agent 共享底座，但 instruction / 工具面 / 前端入口完全独立。

## 2. 当前现状（已实现，无需重建）

| 构件 | 位置 | 状态 |
|---|---|---|
| eino ADK agent loop | `server/internal/agent/core/` | 就绪 |
| 13 个工具（filter/search_*/combine/rank/top/sample/describe/peek/lookup_*/show/bulk_like/create_album） | `server/internal/agent/tools/` | 就绪 |
| ref store + facet + hydration | `server/internal/agent/ref/`、`facets/` | 就绪 |
| pin（frozen/live）+ REST | `server/internal/agent/pins/` | 就绪 |
| chat dock + SSE | `web/src/features/lumilio/` | 就绪 |
| board（react-grid-layout） | `web/src/features/lumilio/` | 就绪 |
| 4 个 widget（asset_grid/facet_dashboard/timeline/storyline）+ widget library 预览页 | `web/src/features/lumilio/widgets/` | 就绪但**职责混乱**（见 D1） |

## 3. 决策点（需要你拍板）

### D1 — Widget 系统重新定位（Apple/Kepo 轻交互模型，非删除）

**修正**：widget 系统**保留**，但不做「重型可视化仪表盘」——那是 `AssetsGalleryPage` 的活（见 D9）。Widget 重新定义为 **Apple/Kepo 式轻交互卡片**：

- **一眼可读（glanceable）**：~2 秒内不需交互就读到核心信息。
- **单一名词**：一个 widget = 一个保存的查询 / 一个人 / 一个地点 / 一个统计 / 一个 delta。不打包多个 job。
- **轻交互**：tap = deep-link 进 `AssetsGalleryPage`（过滤到该 widget 的 ref）；最多一个快速动作（v1 可能只有 deep-link，无动作；后续可加「like top result」这类单点动作，参考 Apple iOS 17 interactive widget）。
- **持久 + 可生成**：用户放置后留存；agent 可按需生成（「给我一个 2024 最佳的 widget」→ pin 一个 saved-query widget）。**这是 Kepo 的 generative widget 玩法移植**——区别是 Kepo 生成的是外部信息源 widget，Lumilio 生成的是用户自己库的名词 widget。

**与重型可视化的分界（关键，避免重蹈冗余覆辙）**：
- **浏览一个结果集（grid/time/facet 的重型交互）** = `AssetsGalleryPage` 的职责。agent 的 `show` 直接 deep-link 进真实画廊（按 ref 过滤/滚动），**不在 widget 里渲染平行网格**。
- **widget** = 上述轻交互卡片，住 board 或 overlay，代表一个名词，tap 进画廊。

**现有四个 widget 的处置**：
| 现状 | 处置 | 理由 |
|---|---|---|
| `AssetGridWidget`（平行网格） | **删** | 重型浏览是画廊的活；平行网格是冗余渲染 |
| `FacetDashboardWidget`（album 封面 + facets） | **改造为轻 summary widget**：封面缩略 + count + 1-2 统计，tap 进画廊 | 保留「这是关于什么的」一眼信息，砍掉重型 facet 表 |
| `TimelineWidget`（可拖拽柱状图） | **改造为轻 rhythm widget**：sparkline + 峰值标注，tap 进画廊该时段 | 时间形状是好的 glance 信息；重型 scrub 交互归画廊的日期分组 |
| `StorylineWidget`（自动播放） | **删** | 不 glanceable、非单一名词、自动播放与「calm/dense」冲突 |

**待你定**：
- widget 住哪：board（现有 react-grid-layout）还是 overlay/侧栏（Kepo 式，与画廊并置，一键唤出），还是两者？
- 轻 widget 的标准尺寸/形态（参考 Apple small/medium/large？还是单一尺寸起步）。
- agent 生成 widget 的入口：chat 里一句话（「给我一个 X 的 widget」）还是有专门 UI？

### D2 — Storyline 命运

**现状**：Instagram 式自动播放 story player（`StorylineWidget`），带 rAF 循环、segmented progress bar、tap zone。

**选项**：
- (a) **完全砍掉**：自动播放与 DESIGN.md「calm/dense/operational」冲突；其叙事职责由 Discovery Agent 用真代数 + 编辑纪律承担，做得更好。
- (b) 保留为 board-only 可选 feature。
- (c) 改造为「手动策展的 slideshow 工具」（用户主动选照片播）。

**我的倾向**：(a)。但这是产品调性决定，你拍。

### D3 — Board 与 Chat 的主次 ✅ LOCKED

**决策：保持现状。** `/lumilio` 为唯一入口，它本身已含 board + chat，是目前最好的界面，不改路由结构。

**隐含推论**（与 D1 联动）：既然 board 是 `/lumilio` 内的既有组件，**widget 就住在 board 上**（作为轻交互卡片，替换当前的重型 widget cell）。Kepo 式独立 overlay 暂不做——board 已是 widget 的家。

`/lumilio/widgets` 预览页：降级为 dev-only（撤掉 `LumilioChat.tsx` 右上角的浮动入口按钮）。

### D4 — `show` 与 `pin` 的职责分离

**现状**：`show(ref_id, widget ∈ {asset_grid}, params)`，既「展示」又暗含「pin widget」，两个职责混在一个 terminal 里。

**修正（与 D1/D9 联动）**：拆成两个目的清晰的 terminal：
- **`show(ref_id)` → deep-link 进 `AssetsGalleryPage`**：agent 产出 ref 后，「展示」= 驱动真实画廊（导航到 `/assets` 并按 ref 过滤/滚动）。重型可视化由画廊承担。**去掉 `widget` 参数。**
- **`pin(ref_id, as="widget")` → 创建一个轻 widget 卡片**（Apple/Kepo 式），住 board/overlay。这是「留下一个名词」的动作，不是「展示」。

两个动作、两个目的：**show = 此刻看（进画廊）；pin = 留下来瞥（生成 widget）。**

**待你定**：show 的 deep-link 机制见 D9；pin 的 `as` 参数 v1 是否只有 widget 一种形态。

### D5 — Pin 的 live/frozen 默认与可见性

**现状**：已实现 frozen/live 双语义（live 仅 producer-only plan；transformer/combine 产物 frozen）。前端未可见化。

**选项**：
- (a) 前端标记 live/frozen + live pin 显示「自上次 +N」delta。
- (b) 只标记 live/frozen，不算 delta。
- (c) 不做可见化（用户无感）。

**我的倾向**：(a)。这是 Task Agent 唯一接近「让 pin 有时间维度」的地方，被动展示，不主动 surface（主动是 Discovery 的活）。

### D6 — 工具面范围（管理 vs 仅可视化）✅ LOCKED

**决策：现有 13 工具 + `tag`。** tag 是「管理」里最高频且非破坏性的动作，起步足够。`delete` / `share` / `stack` 延后（破坏性或大 scope，需 interrupt/resume 确认流，后续单独决策）。

**新增工具**：`tag_assets(ref_id, tags []string, mode ∈ {add, remove})`——对 ref 快照批量打标/移标，返回 ToolReceipt（count + 摘要，不含 ID 列表，遵守 INV-1）。需后端 asset tags 基建（若不存在则前置一个 tag schema 迁移）。

**`bulk_like` / `create_album`**：保留。`create_album` 已有 interrupt/resume 确认流。

### D7 — RichInput / @mention 人物

**现状**：ref-system P2 决定**不做** mention 注入（agent 已有 `lookup_people` 两步解析，mention 是冗余路径）。

**选项**：
- (a) 维持不做。
- (b) 重启 mention 注入（用户体验上比两步走顺，但增加 UI 复杂度）。

**我的倾向**：(a) 维持现状，除非 D6 扩了 tag 后 mention 模式有复用价值。

### D8 — 与 Discovery Agent 的边界渗透

**现状（Discovery 计划 §8）**：v1 不做交叉入口。

**选项**：
- (a) v1 完全隔离（Discovery 卡片不能跳到 Task Agent 操作）。
- (b) Discovery narration 卡片有「在 Task Agent 查这个 ref」跳转。

**我的倾向**：(a) v1。先各自稳了再说；耦合点越晚加越好。

### D9 — 与 `AssetsGalleryPage` 的深度集成（新增，关键）✅ DEEP-LINK LOCKED

**现状**：`AssetsGalleryPage`（`web/src/features/assets/components/page/AssetsGalleryPage.tsx`）已是可复用组件，props 含 `baseFilter` / `lockedFilterFields` / `hero` / `viewKey`，被 album/person 等 scoped 视图复用。已有 `useGalleryContextContributor` 把画廊选择桥接到 agent context（**gallery → chat 方向已通**）。

**决策：deep-link 走 URL，消费 pin。** `?pin={pinId}`。

- agent `show(ref)` 或 widget tap → 导航到 `/assets?pin={pinId}`。
- `AssetsGalleryPage` 读 URL 的 `pin` param → 拉 pin 元数据（含 ref 的 plan / filter）→ 注入画廊渲染。
- **filter-source ref**（plan 是自包含 filter producer）：plan 转 `AssetFilter` 注入 `baseFilter`，画廊按 filter 查询。最高频路径，最简单。
- **非 filter-source ref**（`search_*` / `combine` 产物）：pin 的 `?pin=` 模式触发画廊的 **ref-driven 模式**——画廊按 ref id 走 hydration API 分页取资产（而非按 filter 查询）。这是 `AssetsGalleryPage` 需要新增的一种数据源（filter-driven 之外）。

**impl 落点**：
- `/assets` 路由读 `?pin=` query param。
- `AssetsGalleryPage` 增加 `pinId` prop（或 URL-derived）；当 `pinId` 存在时，数据源切到 pin hydration（复用 `/api/v1/agent/pins/{id}/assets`），`baseFilter` 从 pin plan 转换（filter-source 时）。
- pin 过期/不存在 → 404 兜底文案「该结果已过期，请让 Lumilio 重新查询」（复用 ref-system 已有 404 处理）。

**未决（仍 open）**：ref-driven 画廊模式的工作量。若太重，v1 只支持 filter-source ref 的 `?pin=` deep-link，非 filter-source ref 临时回退到 board 内的 hydration grid（即 `AssetGridWidget` 短期保留作 fallback，直到画廊 ref-driven 模式就绪）。

## Widget Catalog

四种 widget **类型**（渲染形态 / 要建的组件）。每种都 glanceable、单一名词、tap 进画廊（`/assets?pin={pinId}`）。agent 可按需生成实例（「给我一个 X 的 widget」），用户也可手动 pin 任意 ref 为 widget。

### 类型 1 — CoverCard（主力）

- **形**：一张封面缩略图 + 标题 + count + 1 行 meta（日期 / 人物 / 地点）。person 用 avatar 替封面。
- **瞥**：~1 秒读到「这是什么、多少张」。
- **Tap**：→ `/assets?pin={id}` 进画廊。
- **承载名词**：任何 saved query、album、trip、place、person。
- **agent 生成语**：「给我一个 [2024 最佳 / 京都之行 / 小明] 的 widget」
- **大小**：small（封面 + count）/ medium（封面 + count + meta）

### 类型 2 — NumberCard（纯数字瞥）

- **形**：一个大数字 + 标签 + 可选 delta 箭头（↑/↓ vs 上期）。无图，或极小图。
- **瞥**：~0.5 秒读到一个数。
- **Tap**：→ `/assets?pin={id}` 进该数字背后的集合。
- **承载名词**：count、delta、「本月」「未看」「5 星候选」。
- **agent 生成语**：「给我一个 [本月拍了多少 / 未看的有多少] 的 widget」
- **大小**：small

### 类型 3 — SparkCard（轻形状）

- **形**：mini sparkline / 小柱图 + 峰值标注 + count。
- **瞥**：~2 秒看到一个分布的形状。
- **Tap**：→ `/assets?pin={id}` 进画廊（或定位到峰值时段）。
- **承载名词**：时间节奏（hour-of-day / month）、评分分布、相机分布。
- **agent 生成语**：「给我一个 [拍摄节奏 / 评分分布] 的 widget」
- **大小**：medium
- **血统**：当前 `TimelineWidget` 的**轻量后代**——只留 sparkline 形状，砍重型 scrub 交互（scrub 归画廊日期分组）。

### 类型 4 — MosaicCard（视觉集合）

- **形**：2–4 张缩略图（mosaic）+ count + 标签。
- **瞥**：~1.5 秒看到「这组照片长什么样」。
- **Tap**：→ `/assets?pin={id}` 进画廊。
- **承载名词**：top rated、recently liked、collection summary（任何 ref 的封面拼贴）。
- **agent 生成语**：「给我一个 [最高分 / 最近喜欢] 的 widget」
- **大小**：medium / large
- **血统**：当前 `FacetDashboardWidget` 的**轻量后代**——保留视觉封面，砍重型 facet 表。

### 可生成的具体 widgets（示例性，非穷举）

| Widget | 类型 | 背后 predicate（示例） | live/frozen |
|---|---|---|---|
| Best of 2024 | CoverCard / MosaicCard | `filter(date=2024) → rank(quality) → top(N)` | live |
| 京都之行 | CoverCard | `filter(place=Kyoto, date=2025-09)` | live |
| 小明 | CoverCard (avatar) | `search_people(xiaoming_id)` | live |
| 本月 | NumberCard | `filter(date=current_month)`，count + delta vs上月 | live |
| 未看过 | NumberCard | `filter(¬viewed_after_30d)` | live |
| 5 星候选 | NumberCard | `rank(quality) ∩ filter(rating=0)` 的 count | live |
| 截图从未打开 | NumberCard / CoverCard | `filter(type=screenshot) ∩ ¬viewed` | live |
| 拍摄节奏 | SparkCard | hour_of_day histogram | live |
| 评分分布 | SparkCard | rating_dist | live |
| 最高分拼贴 | MosaicCard | `rank(quality) → top(4)` | live |
| 最近喜欢 | MosaicCard | `filter(liked, recent)` | live |
| combine 产物（如「妈 ∧ 京都」） | CoverCard | `intersect(search_people(mom), filter(kyoto))` | **frozen**（plan 引用会话 ref，不能 live 重放） |

**live/frozen 规则**（复用 ref-system §4.3）：plan 是自包含 producer（filter/search_*）→ live，pin 重放；plan 含 transformer/combine 且引用会话 ref → frozen，只存快照。

### MVP vs 后续

- **MVP**：CoverCard + NumberCard 两种类型 + 上表前 6 个具体 widget。覆盖最高频的「留下一个名词瞥一眼」。
- **后续**：SparkCard（节奏 / 分布）+ MosaicCard（视觉集合）。

### 现有四个 widget 的对应处置（与 D1 一致）

| 现有 | 处置 | 去向 |
|---|---|---|
| `AssetGridWidget` | 删 | 重型浏览归画廊（D9 `?pin=`） |
| `StorylineWidget` | 删 | 不 glanceable，自动播放违和 |
| `FacetDashboardWidget` | 改造 → MosaicCard 的 collection-summary 用法 | 后续（非 MVP） |
| `TimelineWidget` | 改造 → SparkCard 的 rhythm 用法 | 后续（非 MVP） |

## 倾向汇总（待你确认或推翻）

若全采纳我的倾向，Task Agent 的形状会是：

- **widget 系统保留**，重新定位为 Apple/Kepo 式轻交互卡片（glanceable / 单一名词 / tap 进画廊 / 可生成）。删 `AssetGridWidget` + `StorylineWidget`，改造 `FacetDashboardWidget` → 轻 summary widget，`TimelineWidget` → 轻 rhythm widget。
- **重型可视化归 `AssetsGalleryPage`**（D9）：agent `show` deep-link 进真实画廊，不在 widget 里渲染平行网格。
- **`show` 与 `pin` 分离**（D4）：show = 此刻进画廊看；pin = 留一个轻 widget 卡片。
- chat 主、board 副；`/lumilio/widgets` 降级为 dev-only（撤主界面浮动入口）。
- pin 的 live/frozen 可见 + delta 提示。
- 工具面：现有 13 个 + 加 `tag`（最保守的「管理」扩展）；delete/share/stack 延后。
- RichInput mention 维持不做。
- 与 Discovery 完全隔离 v1。

**但这只是建议骨架。** 你需要在 §3 的每个决策点上拍板（或提出我没想到的决策点），我才会把这份框架落成带 phased checklist 的执行计划。**特别注意 D1 + D9 是联动的**——widget 重新定位的前提是画廊能承接重型 viz，所以 D9 的 deep-link 机制可行性会反过来约束 D1 的激进程度（见 D9 待你定里的 fallback 说明）。

**进度**：D3 / D6 / D9 + 三个 OQ 已锁。剩余未锁：D2（storyline 倾向砍，待确认）、D5（pin live/frozen 可见性）、D7（mention）、D8（Discovery 边界）。**D9 的 ref-driven 画廊工作量（OQ5）是写出 phased checklist 的最后前置**——它决定 v1 能否完全砍 `AssetGridWidget`，还是需短期 fallback。评估完即可落成执行计划。

## Open Questions

1. ~~Task Agent 与现有 `/assets` 主图库视图的关系~~ → **D9 已答**：深度集成，`?pin={id}` deep-link 驱动画廊。
2. ~~桌面 vs Web 优先级~~ → **已答**：桌面是 Jupyter Lab 形式的服务，只要前端写好即可用，无独立桌面工作。
3. ~~agent 产出文案 i18n~~ → **已答**：agent 文字是模型生成，**不做 i18n**（结构性不可能）。widget chrome 文案（标签、按钮）仍走 i18n。
4. ~~generative widget 边界~~ → **Widget Catalog 已答**：四种类型（CoverCard/NumberCard/SparkCard/MosaicCard）覆盖 saved-query/person/place/stat/rhythm/collection 各类名词；`pin` 的 `as` 参数 v1 可先固定为 CoverCard/NumberCard 两种，agent 选型。
5. **`tag_assets` 后端依赖**：asset tags 基建是否已存在？若无，需一个 tag schema 迁移 + 对应 sqlc 查询作为 D6 的前置。
6. **ref-driven 画廊模式工作量**（D9 未决）：决定非 filter-source ref 的 deep-link 是 v1 就做还是 `AssetGridWidget` 短期 fallback。
