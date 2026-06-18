# Agent 输入面三件套 v2：Gallery 统一 · 伪全局入口 · 原生 Mention/Slash

> 状态: active · 面向: 编码代理与人类贡献者
> 前置阅读: [agent-ref-system.md](../active/agent-ref-system.md)（INV-1~7、ref/ledger/pin）、[agent-ui-refine.md](agent-ui-refine.md)（ChatDock 现状）。
> 本文取代上一版「四批次」计划。上一版被另一个 agent 实现了一遍，方向有三处偏差（见 §1），本文是纠偏后的重构方案。

## 1. 为什么重构（对上一版实现的纠偏）

上一版思路没错（context = 预物化 ref、mention = 输入时消歧、slash = prompt 宏），但落地有三处偏差：

| 偏差 | 现状 | 纠偏 |
|------|------|------|
| **真全局入口** | `GlobalChatDock` 在除 `/lumilio` 外的每个路由都挂 dock | 删除。agent 不该出现在 home/settings/upload。改为**伪全局**：只在「有 gallery 的地方」出现（GalleryView + FullScreenCarousel 的 FAB 区） |
| **RichInput 套壳** | `AgentChatInput` 用 `[&_.rounded-xl]:rounded-full` 等 className hack 硬套老 `RichInput`（contentEditable + Context/Reducer 状态机），slash 靠字符串 `replace` | 删除老 `RichInput/` 目录与 `AgentChatInput`。mention/slash **原生重建在新 dock 输入上**（textarea + popover，无 contentEditable） |
| **错失复用收敛点** | search FAB / agent FAB / context contributor 都属于「gallery 这一层」，但 Album/Trip/Person Details 绕开 `AssetsGalleryPage` 各自手搭，重复 ~80 行 carousel 逻辑、且**全无搜索** | 把 drift 三页面收敛回 `AssetsGalleryPage`，搜索 + agent 入口 + context 一次性免费且一致 |

**保留**（上一版做对的，直接复用）：`state/contextStore.ts`、`contributors/*`、`mentions/mentionSources.ts`、`slash/slashMacros.ts`、`components/Chat/ContextChips.tsx`、typed blocks、dock 的折叠/token 用量 UI、`QUICK_ASKS`；后端全部（`inject/`、`add_to_album`、`inspect`、`lookup_albums`、`asset_filter` 扩参、`agent_handler` 的 context/mentions 接收、`/assets/filter-options` 端点）。

## 2. 架构主轴：AssetsGalleryPage 是「可滚动资产视图」的唯一组合点

`AssetsGalleryPage`（`features/assets/components/page/`）已组合 header + gallery + SearchFAB + carousel，且自带 `baseFilter`/`viewKey`/`lockedFilterFields`——本就是为作用域化集合视图设计的（`UtilityClassifierAlbum` 已这么用）。重构后它**额外**拥有：

- `hero?: ReactNode` slot：渲染在 `AssetsPageHeader` 与 gallery 之间，承载各页面自定义横幅（相册标题/描述/统计、人物封面、行程地图等）。
- agent 伪全局入口（`<ChatDock variant="fab" />`，§4）。

收敛后，凡是「一屏照片」都走它，drift 自然消失。

### 2.1 drift 三页面收敛清单（2026-06-12 可行性核对后修订）

核对 `dto.AssetFilterDTO`（前端 filter 契约）后发现三页可行性**不均等**——`baseFilter` 是收敛的硬前提，因为「搜索」依赖 `AssetsGalleryPage` 的整套搜索渲染机制，光挂 SearchFAB 而数据源无法表达为 baseFilter 等于空写 query。

| 页面 | 数据源 → baseFilter | 可行性 | 备注 |
|------|------|------|------|
| `AlbumDetails` | `{album_id}` | ✅ 纯前端可收敛 | `AssetFilterDTO.album_id` 已存在。**注意**：hero 的滚动折叠动画绑在自有 scroll 容器，收敛后需改听 `#app-scroll-container` |
| `PersonDetails` | `{person_id}` | ⚠️ 需后端 | `AssetFilterDTO` **无 person_id**（后端 `GetAssetsUnified` 已有 `PersonID` 参数）。需在 DTO + filter 构建路径暴露 `person_id` 后 `make dto`，才能收敛 |
| `TripDetails` | 待查（location bbox + date？） | ❓ 待查 | 若可表达为 `location`+`date` baseFilter 则同 Album；否则需评估 |

收敛动作（仅对可行页）：删除该页的 carousel 定位逻辑（slideIndex/isLocatingAsset/定位遮罩）、`FullScreenCarousel` 直挂、header 直挂，改为在现有 `AssetsProvider` 内渲染 `<AssetsGalleryPage baseFilter viewKey title icon lockedFilterFields hero={<自定义横幅/>} />`。

> Phase 2 实测结论：「全收敛」需要先补 `person_id` 进 `AssetFilterDTO`（后端，小改动），TripDetails 数据源待查。本次先落地入口机制（§4）+ **AlbumDetails 收敛已完成**（2026-06-12，380→~210 行，删除重复 carousel 逻辑；hero 改流式横幅，去掉绑死自有 scroll 容器的滚动折叠动画——待 make dev 评估是否需补回）。PersonDetails（需 person_id DTO）/TripDetails（数据源待查）分批推进。

## 3. Context 注入（保留上一版，微调）

- `contextStore`（feature-local Zustand）+ contributors 已就绪：`useGalleryContextContributor`（选择模式 → selection）、`useCarouselContextContributor`（当前浏览 → viewing）。
- 收敛后 contributor 跟随 `AssetsGalleryPage`/`FullScreenCarousel` 自动在所有视图生效（人物页选中、相册页浏览……）。
- 清理：`useCarouselContextContributor(true, ...)` 的硬编码 `true` 去掉（carousel 仅在打开时挂载，isOpen 恒真，参数冗余）。
- 后端：`AskAgent` 请求体的 `context[]` 物化为 `rN_selected`/`rN_viewing` 进 ledger（已实现，保留）。chip 在 dock 输入上方显示、可移除（`ContextChips` 已实现）。

## 4. 伪全局 Agent 入口（FAB + portal dock）

删除 `GlobalChatDock.tsx` 与 `App.tsx` 的挂载。dock 由两处显式挂载：

- **`/lumilio`（看板页）**：`<ChatDock variant="embedded" />`——居中底部面板，in-flow，**不 portal**（沿用现状）。
- **GalleryView（AssetsGalleryPage）**：`<ChatDock variant="fab" />`——折叠态是右下角 FAB（叠在 SearchFAB 上方，同右下角 FAB 簇）；展开时面板 **portal 到 `document.body`、`z-[10000]`**，确保盖过 carousel（`z-9999`）。

`FullScreenCarousel` 内部额外渲染一个小 agent FAB（z 在 carousel 内，约 z-20），点击切换同一 `dockStore`。因为面板已 portal 到顶层，从 carousel 触发时面板浮在 carousel 之上。**单一 dock 实例、单一 `dockStore` 开关**，两个 FAB 触发点。

FAB 布局（已确认：右下堆叠 + portal）：

```
右下角 FAB 簇（z-40）            carousel 内（z-9999 容器内）
┌─────────┐                    ┌──────────────┐
│ 🟣 agent │ ← toggle dock      │  × (top-left)│
│ 🔍 search│ ← SearchFAB        │   ...media...│
└─────────┘                    │      🟣 agent│ ← toggle 同 dock
                               └──────────────┘
        dock 展开 → portal(body) z-[10000]，盖过两者
```

ChatDock 的 `variant` 收敛为 `embedded`（in-flow 面板）/`fab`（FAB + portal 面板）两态；删除 `global` 态与「真全局默认折叠」逻辑。

## 5. 原生 Mention/Slash 输入（替换老 RichInput）

新组件 `components/Chat/MentionInput.tsx`（取代 `AgentChatInput` + 老 `RichInput/`）：

- 基座：`textarea`（自动增高），**不用 contentEditable**。token 以输入框上方/内联的 chip 形式呈现，文本里保留 `@label` 人类可读串。
- 触发：输入 `@` 弹 mention 类型/实体 popover（person/album/pin/camera/lens，源自 `mentionSources`，TanStack Query 拉候选）；输入 `/` 弹 slash 宏 popover（`slashMacros`）。键盘上下选择、Enter 确认、Esc 关闭。
- 提交：emit `{ query, mentions: {type,id,label}[] }`。slash 宏在提交前展开为完整 prompt（用户可见、可编辑——非黑盒）。@pin 走注入语义（§agent-ref-system 4.3），其余 mention 进 `Bound entities:`。
- 消毒：label 是 untrusted，后端进 Instruction 前过 `ref.SanitizeUserText`（已有出口，不新开路径）。

删除：`components/RichInput/` 整个目录、`components/Chat/AgentChatInput.tsx`；回退另一 agent 对 `RichInput/{types,utils,index}.ts` 的改动（随目录删除一并消失）。

## 6. 执行顺序（保持编译可用）

1. **Phase 0 — 入口纠偏 / restore**：删 `GlobalChatDock.tsx`；回退 `App.tsx`（去 import + 挂载）；`LumilioChat` 保持 `variant="embedded"`。
2. **Phase 1 — 原生 MentionInput**：建 `MentionInput.tsx`；ChatDock 改用它（`variant` 收敛 + portal 面板）；**确认编译后**删 `AgentChatInput.tsx` + 老 `RichInput/`。
3. **Phase 2 — Gallery 统一**：`AssetsGalleryPage` 加 `hero` slot；改造 `AlbumDetails`/`PersonDetails`/`TripDetails`（TripDetails 验证数据源，必要时 `useCarouselController` 兜底）；在 `AssetsGalleryPage` 挂 `<ChatDock variant="fab" />`。
4. **Phase 3 — Carousel agent FAB**：`FullScreenCarousel` 内加 agent FAB 触发 `dockStore`；清理 `useCarouselContextContributor` 冗余参数。
5. **Phase 4 — 收尾**：i18n `vp exec i18next-cli extract` 补 dock/mention/context key；删 tech-debt 里 RichInput 下线条目（已恢复）；质量门。

## 7. 质量门与验收

每阶段：`make web-test`（`vp check --no-fmt --no-lint && vp lint && vp test`）、`make server-test`（后端未变则免）、i18n 走 extract 不手编。

验收（手动 `make dev`）：
- home/settings/upload **无** agent dock；`/lumilio` 看板页 dock 居中底部如旧（伪全局生效，真全局已撤）。
- Album/Person/Trip 详情页：出现 SearchFAB（可搜该集合）+ 右下 agent FAB；carousel 内 agent FAB；dock 展开盖过 carousel。
- 选中 N 张 → agent FAB → dock 显示「已附上 N 张」chip → 「做成相册」走 create_album。carousel 浏览单张 → 「这张什么参数」→ inspect 可答。
- mention：`@人物` 直接 search_people 无追问；`@pin` 注入 ledger；`/回顾` 展开为可编辑 prompt。
- 老 `RichInput/` 与 `AgentChatInput` 已删，输入框 @ // 原生可用。

## 8. Open Questions

1. TripDetails 数据源能否表达为 `baseFilter`——Phase 2 实施时验证；不能则该页用 `useCarouselController` 兜底（仍消除 carousel 重复，但不入 AssetsGalleryPage）。
2. portal 面板在移动端的尺寸/键盘遮挡——Phase 1 实施时按 ChatDock 现有 max-h 处理。
3. FAB 簇与移动端底部安全区（`SquareGallery` 紧凑模式）的间距——Phase 3 微调。
