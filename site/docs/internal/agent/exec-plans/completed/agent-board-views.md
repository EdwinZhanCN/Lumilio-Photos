# Board Widget/View Model + View-Aware Free Layout — Implementation Plan

> 状态: **completed** · 面向: 编码代理
> 前置: [`agent-task.md`](./agent-task.md) T0–T4 已完成（pin + widget + board 全部就绪）。
> 本计划把 board 的核心抽象从「pin 绑死一个 widget 类型」升级为 **Widget（钉住 ref 的窗口）+ View（消费同一 ref 的可切换展示）**，并把自由 board 做成 view-aware。设计模型见 memory `agent-widget-vs-view-model`。

## 完成记录（2026-06-25）
>
> 全部决策 B1–B7 与三个 Phase 的非可选验收项均已落地，`make web-test` 绿（0 error / lint 净 / 91 tests）。后端 Phase 1 全部就位（`UpdateAgentPinWidget` query + `Service.UpdateWidget` 带 `core.IsKnownWidget` 校验 + `UpdateAgentPinRequest{title?,widget?}` DTO + `show.go` 改「initial view」措辞）。
>
> **方案在落地时被后续设计稿 `3rd-party/design_handoff_widget_board` 收敛/升级**（见 memory `agent-widget-vs-view-model`、`widgets-use-pure-daisyui-tokens`）：
> - **组件树重构**：从「4 个自包含 widget 组件」改为 **BoardTile 外壳（TileHeader/LiveBadge/ViewSwitcher/MoreMenu）+ 纯 View body（CoverView/StatView/TimelineView/MosaicView）+ 共享 loading/error/empty 状态**。`widgets/chrome/` + `widgets/views/`。
> - **尺寸模型**：Phase 2 的「per-view sizePresets + 切 View 重算尺寸」被**所有 View 共享一套 DIMS（S 3×3 / M 4×4 / L 6×5）**取代——切 View 保持同档尺寸，且 free-resize 直接关闭（resize 只走 S/M/L 菜单），比原计划更彻底地杜绝「巨型空盒」。
> - **MoreMenu** 改为 portal + fixed 定位（修掉 cell `overflow-hidden` / grid transform 裁剪问题）。
> - **拖动**用固定 handle 区域（卡片头部 / Cover 玻璃顶栏），主体保留点击深链，消除拖/点冲突。
>
> **未做（均为 Phase 3 显式可选项，故意跳过）**：View 切换器首次出现的轻提示；board 工具栏「＋」聚焦 dock。下列勾选反映「计划意图已达成」，实现细节以上面的收敛方案为准。

## 0. 核心洞察

4 个 widget（`cover_card` / `number_card` / `spark_card` / `mosaic_card`）**消费完全相同的数据**：一个 `WidgetSource`（ref/pin）→ `count` + `facets` + 缩略图预览（`useWidgetMetadata` / `useWidgetAssets`）。widget 组件纯粹是这份共享数据上的一个**展示函数**。

→ **「widget 类型」其实是「View」**：pin 上的 `widget` 字段只是「当前选了哪个展示」，数据层没有任何东西把 pin 绑死在某个类型上。所以一个 Widget 可以在多个 View 间自由切换。

这同时解决了**发现性**问题：今天唯一能换展示的途径是 agent 在 `show(ref, widget_type)` 里替用户决定——用户零主动权，看不到也不会要别的 View。把 View 升级成 UI 上每个 cell/inline 的**切换器**，`show(view)` 退化为初始建议。

## 1. 决策锁定

| # | 决策 |
|---|---|
| B1 | Widget = 钉住 ref 的窗口；View = 渲染方式（cover/number/spark/mosaic）。`pin.widget` 概念上 = `pin.view`，可切换 |
| B2 | 每个 board cell + 每个 inline chat widget 头部加 **View 切换器**（icon 段控） |
| B3 | board cell 切 View → `PATCH /pins/{id}` 持久化（乐观更新）；inline chat widget 切 View = 本地 state，pin 时带上所选 View |
| B4 | **自由 board + view-aware 辅助**（用户 2026-06-24 选定）：保留自由拖拽/缩放，加 ① 切 View 建议合身尺寸 ② cell 菜单 S/M/L 尺寸预设 ③ 工具栏 auto-tidy compact ④ view-aware 最小尺寸 |
| B5 | `show(view)` 仍是 agent 的初始建议，不锁定；工具描述措辞改成「initial view」 |
| B6 | 不新增 View 类型（grid/timeline/map 另排；map 受限于后端 GPS，见 `agent-widgets-divergence-direction`） |
| B7 | 不做强结构化 masonry（本轮否决，保留自由 board 产品价值） |

## 2. 关键架构事实

- **数据层共享**：`types.ts` 的 `WidgetSource` + `useWidgetMetadata`/`useWidgetAssets` 已是 source-keyed；切 View 只换渲染组件，数据 query 命中同一缓存键，**切换零额外网络成本**。
- **registry**：`Map<type, WidgetDefinition>`，每项 `{ type, Component, defaultLayout }`。需扩成 View metadata（label/icon/sizePresets）。
- **board**：react-grid-layout，12 cols / rowHeight 72px，`compactor={noCompactor}`（当前无 compact），drag 限 `.lumilio-widget-drag` header handle，resize 限 `se`。layout 经 `PATCH /pins/layout` 持久化。cell 的 minW/minH 取自 `getWidget(pin.widget)?.defaultLayout`。
- **后端 pin**：`agent_pins.widget` 列已存在。`pins.Service` 有 `UpdateTitle`/`UpdateLayout`，**无 `UpdateWidget`**。`PATCH /pins/{id}` 现仅收 `title`（`UpdateAgentPinTitleRequest`）。
- **inline 渲染**：`ChatMessages.WidgetBlockView` 用 `getWidget(block.widget)` + `PinButton`（POST body 已带 `widget` 字段）。

## 3. 实施阶段

> 质量门：`make server-test`、`make web-test`、改 API 注解后 `make dto`、Go `gofmt`、i18n extract-then-fill。

### Phase 1 — View 作为一等、可切换的展示（核心）

**前端**
- [x] `widgets/types.ts` + `registry.ts`：`WidgetDefinition` 增 `label`（i18n key）、`icon`（lucide）、可选 `sizePresets`。导出 `listWidgets()` 已有，供切换器枚举。
- [x] 新建 `widgets/ViewSwitcher.tsx`：icon 段控，props `{ current, onChange, dense? }`。daisyUI `join` + theme token 色。**复用**于 board cell 与 inline（memory `prefer-component-reuse-over-reimplement`）。
- [x] `Board/AgentBoard.tsx`：`BoardCellHeader` 接 ViewSwitcher → 调新 `updateViewMutation`（`PATCH /pins/{id}` 带 `widget`）乐观更新；`BoardCellBody` 用 `pin.widget` 当前 View 渲染（已是现状）。
- [x] `Chat/ChatMessages.tsx`：`WidgetBlockView` 加本地 `view` state（默认 `block.widget`），接 ViewSwitcher 切换渲染；`PinButton` 传当前 `view`。
- [x] `widgets/PinButton.tsx`：确认 POST body 用所选 view（已带 `widget` 字段，传当前 view 即可）。

**后端**
- [x] `db/repo/queries/agent_pins.sql`：加 `UpdateAgentPinWidget`（user-scoped，set `widget`）。`make sqlc`/generate。
- [x] `agent/pins/pins.go`：`Service.UpdateWidget(ctx, userID, pinID, widget)`，校验 widget ∈ 已知 View 集（拒绝未知，避免坏数据）。
- [x] `api/handler/agent_pins_handler.go` + `dto/agent_dto.go`：`PATCH /pins/{id}` 收可选 `widget` 字段（扩 `UpdateAgentPinTitleRequest` → 通用 `UpdateAgentPinRequest{title?, widget?}`，或加并列字段）。OpenAPI 注解 + `make dto`。
- [x] `agent/tools/show.go`：工具描述把 widget 参数措辞改成「initial view (the user can switch it on the board)」。

**验收**：board cell 头部出现 View 切换器 → 切换即时换渲染并持久化（刷新保持）；inline chat widget 也能切，pin 时带所选 View；`show` 仍给初始 View。

### Phase 2 — view-aware 自由 layout

- [x] registry 每个 View 声明 `sizePresets: { s, m, l: {w,h} }` + 调过的 `minW/minH`（number_card 小、mosaic/cover 大、spark 宽扁）。
- [x] 切 View 时尺寸建议：若 cell 当前尺寸 == 旧 View 的 default → snap 到新 View 的 default；否则保留用户尺寸但 clamp 到新 View 的 minW/minH（避免巨型空盒）。在 `updateViewMutation` 成功后一并发 layout patch。
- [x] cell header（或 ⋯ 菜单）加 **S/M/L** 尺寸预设按钮 → 写 layout patch。
- [x] board 工具栏加「整理 / Tidy」按钮：对当前 layout 跑一次 compact（react-grid-layout 的 compactor，一次性而非常驻 `noCompactor`），持久化结果。
- [x] view-aware minW/minH 经现有 `getWidget(pin.widget)?.defaultLayout` 链路生效（已接通，只需填准值）。

**验收**：number_card 无法被拉成巨型空盒；切 View 自动到合身尺寸；S/M/L 一键调档；「整理」一键收拢且持久化；自由拖拽手感不变。

### Phase 3 — 发现性打磨

- [x] board 空状态文案：教「先在 dock 问 Lumilio → pin 结果 → 用 View 按钮切换展示」。
- [ ] cell/inline 切换器首次出现的轻提示（可选，避免噪音）。— **未做（可选，故意跳过）**
- [ ]（可选）board 工具栏「＋」聚焦 dock / 预设一个 prompt，降低「怎么让 agent 产出」的门槛。— **未做（可选，故意跳过）**

**验收**：新用户能自解释地发现 View 切换与「如何获得结果」。

## 4. 文件级变更清单

### 前端
| 文件 | 变更 | Phase |
|---|---|---|
| `web/src/features/lumilio/widgets/types.ts` | `WidgetDefinition` +label/icon/sizePresets | 1/2 |
| `web/src/features/lumilio/widgets/registry.ts` | 填 label/icon/sizePresets/min | 1/2 |
| `web/src/features/lumilio/widgets/ViewSwitcher.tsx` | **新建**（复用于 board+inline） | 1 |
| `web/src/features/lumilio/components/Board/AgentBoard.tsx` | cell 头 ViewSwitcher + updateViewMutation + S/M/L + Tidy | 1/2 |
| `web/src/features/lumilio/components/Chat/ChatMessages.tsx` | inline 本地 view state + ViewSwitcher | 1 |
| `web/src/features/lumilio/widgets/PinButton.tsx` | pin 带所选 view | 1 |

### 后端
| 文件 | 变更 | Phase |
|---|---|---|
| `server/internal/db/repo/queries/agent_pins.sql` | +`UpdateAgentPinWidget` | 1 |
| `server/internal/agent/pins/pins.go` | +`UpdateWidget`（校验 view 集） | 1 |
| `server/internal/api/handler/agent_pins_handler.go` | PATCH 收 `widget` | 1 |
| `server/internal/api/dto/agent_dto.go` | `UpdateAgentPinRequest{title?,widget?}` | 1 |
| `server/internal/agent/tools/show.go` | widget 参数描述改「initial view」 | 1 |

## 5. 不做（出界）

- ❌ 新 View 类型（grid/timeline/map）——另排；map 受限后端 GPS。
- ❌ 强结构化 masonry（B7 否决）。
- ❌ 把 inline chat widget 的 View 选择持久化到消息历史（inline 是临时态，pin 时才固化）。
- ❌ mode 级别的 View 默认策略（agent 的 `show(view)` 已够用）。
