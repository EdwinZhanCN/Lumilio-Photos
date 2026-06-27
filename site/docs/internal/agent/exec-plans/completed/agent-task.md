# Task Agent — Implementation Plan

> 状态: **completed** · 面向: 编码代理
> 前置: [`agent-ref-system.md`](./agent-ref-system.md) Phase 1–3 已完成。配套: Discovery Agent 走 [`agent-discovery.md`](./agent-discovery.md)，两份独立。
> 本计划交付**保守路线**：用户发起查询 → agent 组合代数 → 以多种方式可视化/管理媒体库。Widget 系统保留，重新定位为 Apple/Kepo 式轻交互卡片；重型可视化深度集成进 `AssetsGalleryPage`。

## 1. 决策锁定（context，不再讨论）

| # | 决策 |
|---|---|
| D1 | Widget 在 board 上；自由调整大小（无固定尺寸，沿用现有 minW/minH 无 maxW/maxH）；四类型 CoverCard/NumberCard/SparkCard/MosaicCard，MVP = CoverCard+NumberCard |
| D2 | StorylineWidget 砍掉 |
| D3 | `/lumilio` 唯一入口（board+chat 现状）；`/lumilio/widgets` dev-only，撤浮动入口按钮 |
| D4 | `show` 不变（仍发 side-channel 事件 + PinButton）；widget 卡片增加 tap → gallery deep-link |
| D5 | pin 的 live/frozen 可见 + delta 提示 |
| D6 | 加 `tag_assets` 工具（后端已就绪，纯工具层） |
| D7 | 不做 RichInput mention |
| D8 | v1 与 Discovery 完全隔离 |
| D9 | deep-link = `/assets?pin={pinId}`，画廊消费 pin |

## 2. 关键架构事实（来自深度调查）

- **Pin 系统已完整**：`agent_pins` 表（migration 006）含 `pin_id/user_id/title/widget/mode/plan/summary/asset_ids/truncated/layout_*/timestamps`。`pins.Service` 有 `CreateFromRef/List/Delete/UpdateLayout/AssetIDs`（live 重放）。REST: POST/GET/GET:id/GET:id/assets/PATCH:layout/DELETE，全 user-scoped。
- **Pin 是用户动作，非 agent 工具**：`show` 发 `widget_show` side-channel → 前端 `ChatMessages` 渲染 inline widget + `PinButton` → 用户点 → `POST /pins` → board 渲染。**这条链保留不动。**
- **Tags 基建已完整存在**：`tags` + `asset_tags` 表（migration 003）、CRUD sqlc 查询、`AssetService.AddManualTagToAsset/RemoveTagFromAsset/GetOrCreateTagByName`、filter 维度（`search.Filter.TagName/TagNames` → unified queries 全支持）。`tag_assets` 是**纯工具层新增**，零 DB 工作。
- **Board**：react-grid-layout v2，12 cols / rowHeight 72px，`gridConfig/dragConfig/resizeConfig`，自由 resize（只 minW/minH），drag 限 header handle。layout PATCH 持久化（乐观更新）。
- **Widget registry**：`Map<type, WidgetDefinition>`，`defaultLayout={w,h,minW,minH}`。Board 用 `getWidget(pin.widget)` 取组件 + minW/minH。
- **`AssetsGalleryPage`**：可复用组件（props: `baseFilter/lockedFilterFields/hero/viewKey`），数据源是 `useCurrentAssetsView({ baseFilter, sortBy })`（filter-driven）。已有 `useGalleryContextContributor`（gallery→chat 桥）。**反方向（agent→gallery）不存在，需新建。**
- **`/lumilio` 无 SideBar 入口**：仅靠 ChatDock maximize 按钮或 URL 到达。

## 3. Widget Catalog（保留，供实现参照）

四种轻交互卡片类型。每种 glanceable、单一名词、tap 进画廊。MVP 建 CoverCard + NumberCard。

### 类型 1 — CoverCard（主力，MVP）
- 形：封面缩略 + 标题 + count + 1 行 meta（日期/人物/地点）。person 用 avatar。
- tap：→ `/assets?pin={pinId}`
- 承载：saved query / album / trip / place / person

### 类型 2 — NumberCard（纯瞥，MVP）
- 形：大数字 + 标签 + 可选 delta 箭头。
- tap：→ `/assets?pin={pinId}`
- 承载：count / delta / "本月" / "未看" / "5 星候选"

### 类型 3 — SparkCard（后续）
- 形：mini sparkline + 峰值标注 + count。`TimelineWidget` 的轻量后代。

### 类型 4 — MosaicCard（后续）
- 形：2–4 缩略 mosaic + count + 标签。`FacetDashboardWidget` 的轻量后代。

### 现有 widget 处置
| 现有 | 处置 |
|---|---|
| `AssetGridWidget` | 删（重型浏览归画廊） |
| `StorylineWidget` | 删（D2） |
| `FacetDashboardWidget` | 删旧版；后续重建为 MosaicCard |
| `TimelineWidget` | 删旧版；后续重建为 SparkCard |

## 4. 实施阶段

> 质量门：`make server-test`、`make web-test`、改 API 注解后 `make dto`、Go `gofmt`。

### Phase T0 — `tag_assets` 工具（后端，纯工具层）

**零 DB 工作**——tags 基建全存在。

- [x] 新建 `server/internal/agent/tools/tag_assets.go`：
  - Input: `{ ref_id string, tags []string, mode ∈ {"add","remove"} }`
  - `add` 模式：对 ref 快照每个 asset 调 `AssetService.AddManualTagToAsset(ctx, assetID, tagName)`（内部 get-or-create + `source="user"` confidence 1.0）。
  - `remove` 模式：对每个 asset 调 `RemoveTagFromAsset`（需 tag_id，先 `GetTagByName` 解析）。
  - 循环模式照搬 `add_to_album.go:136-147`。
  - 返回 `ToolReceipt{ref_id, count, summary}`（count = 受影响 asset 数，summary 一行摘要）。遵守 INV-1（不回 ID 列表）。
  - 阈值：>1000 assets 时要求先 top/sample 缩小（沿用 `bulk_like` 的 `LimitExceeded` 模式）。
- [x] `server/internal/agent/tools/common.go:RegisterAll()` 加 `RegisterTagAssets()`。
- [x] （可选增强）`asset_filter.go` 的 `AssetFilterInput` 加 `tag_names []string` 字段，`buildFilterParams` 映射到 `search.Filter.TagNames`，让 agent 能按 tag 过滤。这是小改动，解锁 tag 维度的检索。
- [x] OpenAPI 注解 + `make dto`（若 schema 变）。

**验收**：agent 在对话里 `tag_assets(ref, ["旅行"], "add")` → 库里 asset_tags 写入 source=user 行；`filter_assets(tag_names=["旅行"])` 能取回（若做了可选增强）。

### Phase T1 — Gallery `?pin=` deep-link（前端，核心新能力）

让 `AssetsGalleryPage` 能消费 pin——这是 D1「重型 viz 归画廊」的实现落点。

- [x] `/assets` 路由（`web/src/features/assets/routes/Assets.tsx`）读 URL `?pin={pinId}` param，传入 `AssetsGalleryPage`。
- [x] `AssetsGalleryPage` 加 `pinId?: string` prop。当存在时切换数据源：
  - **filter-source live pin**（plan 是 `filter_assets`）：把 pin plan 的 params 转成 `AssetFilter`（复用 `pins.go:replayFilter` 同款映射逻辑，前端侧），作 `baseFilter` 注入现有 `useCurrentAssetsView`。画廊走原生 filter 路径（快、享受 sort/search-within）。
  - **非 filter-source pin**（search/combine 产物 / frozen）：新增 **ref-driven 数据源**——用 `$api.useInfiniteQuery("get","/api/v1/agent/pins/{id}/assets")` 按偏移分页取资产，喂给画廊的 `browseAssets`（需把 `AgentRefAssetsDTO` 适配成画廊期望的 `browseItems` 形状）。
- [x] pin context hero（`hero` prop）：渲染 pin title + count + live/frozen 徽章 + 「返回 Lumilio」链接。
- [x] pin 过期/404：显示「该结果已过期，请让 Lumilio 重新查询」（复用 ref-system 已有 404 文案模式）。
- [x] URL 可分享/可书签（`?pin=` 是纯 URL state）。

**验收**：board 上点 widget 卡片 → 导航到 `/assets?pin={id}` → 画廊渲染该 pin 的资产，分页正确，按快照序；filter-source pin 走 filter 路径（sort/search-within 可用）；非 filter-source pin 走 hydration 路径；过期 pin 有兜底文案。

**实现要点**：ref-driven 模式需把 `AgentRefAssetsDTO` 适配进画廊的 `browseItems`/`browseGroups` 形状。项目结构清晰（画廊已抽象 `useCurrentAssetsView` 数据层），新增 pin-hydration 数据源是直接工作，非阻塞。

### Phase T2 — 轻 widget 卡片（前端，MVP 两种）

- [x] 新建 `web/src/features/lumilio/widgets/CoverCardWidget.tsx`：
  - `variant="inline"`：compact 卡片（封面 + count + title），渲染在 chat 消息内，下方接现有 `PinButton`。
  - `variant="board"`：同形态，整卡可点 → `<Link to={"/assets?pin=" + pinId}>`。
  - hydrate：`useWidgetAssetsPreview(source, 1)` 取封面；`useWidgetMetadata(source)` 取 count/facets。source 已支持 `{kind:"pin"}` / `{kind:"ref"}`，无需改 hook。
- [x] 新建 `web/src/features/lumilio/widgets/NumberCardWidget.tsx`：
  - 显示 count（大字）+ 标签 + 可选 delta（见 T3）。
  - tap 行为同 CoverCard。
- [x] `registry.ts` 注册 `cover_card` / `number_card`，各给 `defaultLayout: { w:3, h:2, minW:2, minH:2 }`（轻卡片比旧 widget 小）。删旧四项注册。
- [x] `show` terminal（`server/internal/agent/tools/show.go`）：`WidgetAssetGrid` 常量改为/增 `WidgetCoverCard = "cover_card"` / `WidgetNumberCard = "number_card"`。`RegisterShow` 的默认 widget 改 `cover_card`。`show_facets/show_timeline/show_storyline` 三个注册删除（D2 + 砍旧 widget）。
- [x] `PinButton.tsx`：兼容新 widget 类型（传 `widget` 字段进 POST body，已是现状）。
- [x] `ChatMessages.tsx`：`WidgetBlockView` 用新组件渲染 inline 变体。

**验收**：agent `show(ref, "cover_card")` → chat 内出 CoverCard inline + PinButton → 点 pin → board 出 CoverCard board 变体 → 点卡片 → 进画廊（T1 的 `?pin=`）。

### Phase T3 — 清理与 board 打磨

- [x] 删 `AssetGridWidget.tsx`、`StorylineWidget.tsx`、旧 `FacetDashboardWidget.tsx`、旧 `TimelineWidget.tsx`。
- [x] 删 `/lumilio/widgets` 路由 + `LumilioChat.tsx` 里浮动 Widget Library 按钮（D3）。`WidgetLibrary.tsx` + `mockWidgetData.ts` 保留为 dev fixture（直接 URL 可达，不在主 UI 暴露）。
- [x] board 卡片 live/frozen 徽章（D5）：`AgentBoard` 的 `BoardCellBody` 或卡片组件内，读 `pin.mode` 显示徽章。
- [x] delta 提示（D5 简版）：live pin 的 board 卡片，hydrate 时 `GetPin` 重放 plan 得当前 count，与 pin 存储的 `count`（快照数）比，当前 > 快照 → 显示「+N new」。无需新 DB 列。（「自上次查看」的精确 delta 需加 `last_viewed_at` 列，延后。）

**验收**：无死代码（grep 无旧 widget 引用）；board 卡片有 live/frozen 徽章 + delta；浮动 widget library 按钮消失。

### Phase T4 — SparkCard + MosaicCard（后续，非 MVP）

- [x] `SparkCardWidget.tsx`：从 `useWidgetMetadata` 的 `facets.histogram` 渲染 mini sparkline + 峰值标注。
- [x] `MosaicCardWidget.tsx`：`useWidgetAssetsPreview(source, 4)` 取 2–4 缩略拼贴。
- [x] registry 注册；show terminal 增对应 widget 常量。

## 5. 文件级变更清单

### 后端
| 文件 | 变更 |
|---|---|
| `server/internal/agent/tools/tag_assets.go` | **新建**（T0） |
| `server/internal/agent/tools/common.go` | +1 行注册（T0） |
| `server/internal/agent/tools/asset_filter.go` | （可选）加 `tag_names`（T0） |
| `server/internal/agent/tools/show.go` | widget 常量改名；删 show_facets/timeline/storyline 注册（T2） |
| `server/internal/api/dto/agent_dto.go` | 若 pin DTO 加 `last_viewed_at`（T3 增强，可选） |

### 前端
| 文件 | 变更 |
|---|---|
| `web/src/features/lumilio/widgets/CoverCardWidget.tsx` | **新建**（T2） |
| `web/src/features/lumilio/widgets/NumberCardWidget.tsx` | **新建**（T2） |
| `web/src/features/lumilio/widgets/registry.ts` | 注册新类型、删旧（T2） |
| `web/src/features/lumilio/widgets/AssetGridWidget.tsx` | **删**（T3） |
| `web/src/features/lumilio/widgets/StorylineWidget.tsx` | **删**（T3） |
| `web/src/features/lumilio/widgets/FacetDashboardWidget.tsx` | **删旧**（T3，T4 重建 MosaicCard） |
| `web/src/features/lumilio/widgets/TimelineWidget.tsx` | **删旧**（T3，T4 重建 SparkCard） |
| `web/src/features/assets/routes/Assets.tsx` | 读 `?pin=` param（T1） |
| `web/src/features/assets/components/page/AssetsGalleryPage.tsx` | 加 `pinId` prop + ref-driven 数据源（T1，主要工作量） |
| `web/src/features/lumilio/components/Board/AgentBoard.tsx` | live/frozen 徽章（T3） |
| `web/src/features/lumilio/routes/LumilioChat.tsx` | 删浮动 widget library 按钮（T3） |
| `web/src/routes/routes.tsx` | （可选）删 `/lumilio/widgets` 路由（T3） |

## 6. 不做（出界）

- ❌ Task Agent 不加叙事/发现能力（Discovery Agent 的活）。
- ❌ 不做 Memory System（库即记忆）。
- ❌ v1 不做 vision 工具。
- ❌ delete/share/stack 工具延后（D6 只加 tag）。
- ❌ 「自上次查看」精确 delta（需 last_viewed_at 列，延后；v1 用「since creation」简版）。
- ❌ RichInput mention（D7）。

## 7. 备注

1. **widget 类型无需迁移**：本项目无生产环境。直接改 `server/migrations/000006_cloud_agent.up.sql` 里 `widget` 列的 DEFAULT（`'asset_grid'` → `'cover_card'`），以及 `show.go` 的常量。重置 DB 即生效。
2. **`/lumilio` 入口已就绪**：经 ChatDock maximize 按钮可达，不需 SideBar 入口。
3. **T1 非风险**：项目结构清晰，画廊数据层已抽象（`useCurrentAssetsView`），加 pin-hydration 数据源是直接工作。
