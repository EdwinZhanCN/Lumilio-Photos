# Lumilio Agent UI 重构：统一仪表盘 + 内嵌对话 Dock

## Context

参照用户提供的 mockup，把 `/lumilio` 从「对话 / 看板」双页签改为**统一单页**：widget 仪表盘铺满页面，Agent 对话以可收起的悬浮 dock 形式叠在网格上（"对话驱动看板"）。明确的范围约束（用户已确认）：

- **不复刻 mockup 里的系统 widgets**（Library Stats / Cameras / Timeline / Quick Asks 只是示意）——widget 系统保持现状（pins 支撑的 ref widgets），重点是把 **可拖拽 + 可缩放的 customize 布局** 做好；
- **不需要后端改动**（pins/layout API 已有）；
- **暂时移除 RichInput**（mention/@、slash 功能下线，写入文档以后恢复），对话输入改为普通 daisyUI 输入；
- 图标用 lucide-react（现状已是），其余 UI 保持 daisyUI 颜色与元素，网格用 react-grid-layout（已用 v2.2.3）。

纯前端改动，全部位于 `web/src/features/lumilio` + 路由表。

## 现有可复用资产（全部已存在）

- [AgentBoard.tsx](web/src/features/lumilio/components/Board/AgentBoard.tsx)：RGL v2 `GridLayout` + `useContainerWidth`，拖拽手柄 `.lumilio-widget-drag`，布局变更 PATCH `/agent/pins/layout`，删除按钮，widget registry 渲染。**RGL v2 缩放默认开启**（`ResizeConfig.enabled` 默认 true、默认 'se' 手柄，`react-grid-layout/css/styles.css` 已导入含手柄样式）——缩放其实已可用，本次补验证与手柄可见性。
- [ChatMessages.tsx](web/src/features/lumilio/components/Chat/ChatMessages.tsx)：typed blocks 消息渲染（text/reasoning/tool/widget/confirm），可直接放入 dock。
- [chatStore.ts](web/src/features/lumilio/state/chatStore.ts)：Zustand 会话状态（sendMessage/confirmInterrupt/newConversation/isGenerating/connectionError）。
- [LumilioAvatar](web/src/features/lumilio/components/LumilioAvatar/LumilioAvatar.tsx)：dock 头部品牌图标（NavBar 同款）。
- widget registry（[registry.ts](web/src/features/lumilio/widgets/registry.ts)）的 `defaultLayout.minW/minH` 已接进 layout 约束。

## 实施步骤

### 1. 新组件 `components/Chat/ChatDock.tsx`

悬浮对话 dock，绝对定位于页面容器底部居中（`absolute bottom-4 left-1/2 -translate-x-1/2`，宽 `w-[min(720px,calc(100%-2rem))]`，`z-20`），daisyUI 卡片样式（`bg-base-100 border border-base-300 rounded-box shadow-xl`）。三段结构：

- **Header**：LumilioAvatar 小尺寸 + "Lumilio Agent" 标题 + 状态点（`isGenerating` → busy / 否则 ready，daisyUI `status` 或小圆点）+ 右侧两个 ghost 圆钮：新对话（lucide `RotateCcw`，调 `newConversation`，有消息时才显示）与折叠切换（lucide `ChevronDown/ChevronUp`）。Header 整体可点击切换折叠。
- **Body**（展开时）：`ChatMessages` 放入 `max-h-[45vh] overflow-y-auto`；空会话时显示现有 `lumilio.chat.empty` 引导文案；`connectionError` 横条与 capabilities 禁用横幅（沿用现 [LumilioChat.tsx](web/src/features/lumilio/routes/LumilioChat.tsx) 的 `agentDisabledReason` 逻辑 + 设置页链接）移入 body 顶部。
- **Footer**：新的普通输入条（见步骤 2）。

折叠态：只剩 header + footer（输入条常驻，像 mockup 底部那条）；`isGenerating` 期间收到新消息不强制展开，但 header 状态点变化提示。折叠状态存组件 state 即可（不持久化）。

### 2. 新组件 `components/Chat/ChatInput.tsx`（替代 LumilioInput/RichInput）

普通 daisyUI 输入条：`textarea`（单行起步自动增高可后置，v1 用 `input` 即可）+ 发送按钮（lucide `Send`，`btn btn-primary btn-circle btn-sm`）。Enter 发送；`isGenerating || disabled` 时禁用；placeholder 复用 `lumilio.input.prompt` / `lumilio.input.unavailable`。不引入 RichInput 的任何依赖。

### 3. 路由与页面重构

- [LumilioChat.tsx](web/src/features/lumilio/routes/LumilioChat.tsx) 重写：删除 `LumilioTabs` 与 `view` prop，页面 = `relative` 容器内的 `<AgentBoard />`（铺满）+ `<ChatDock />`（叠加）。
- [routes.tsx](web/src/routes/routes.tsx)：删除 `/lumilio/board` 路由（统一单页后无意义）。
- AgentBoard 调整：内容区底部加 `pb-40`（给 dock 留出滚动余量）；空看板的引导文案微调（仍指向"在对话里固定结果"，对话现在就在同屏 dock 里）；显式传 `resizeConfig={{ enabled: true, handles: ["se"] }}` 并确认手柄在卡片圆角内可见（必要时加少量 CSS 覆盖手柄定位/颜色为 `text-base-content/30`）。

### 4. 清理与文档

- 删除使用点：`components/LumilioChat/LumilioInput.tsx` 与 barrel `components/LumilioChat/index.ts`（`LumilioStatus` 若再无引用一并删除）。
- **RichInput 目录保留不删**（`components/RichInput/`，自包含、无外部引用），作为未来恢复 mention/@people、slash 的基础；在 [agent-ref-system.md](site/docs/internal/agent/exec-plans/active/agent-ref-system.md) 增补一行记录"RichInput 暂时下线，dock 用纯输入；恢复 mention 注入是后续项"，同时在 [tech-debt-tracker.md](site/docs/internal/agent/exec-plans/tech-debt-tracker.md) 登记。
- i18n：新增 dock 文案 key（如 `lumilio.dock.ready`、`lumilio.dock.collapse`、`lumilio.chat.newConversation` 复用），跑 `vp exec i18next-cli extract` 后补 zh/en 值（沿用本仓库流程，不手编结构）。

## 验证

1. `cd web && vp check --no-fmt --no-lint && vp lint && vp test`（quality gate，纯前端无后端变更）。
2. 手动（`make dev`）：
   - `/lumilio` 单页：网格铺满，dock 悬浮底部；`/lumilio/board` 不再存在；NavBar 头像入口仍达 `/lumilio`；
   - 对话流：发消息 → typed blocks 在 dock 内滚动渲染 → `show` 的 widget block 内嵌预览 + 「固定到看板」→ 看板出现新卡片；
   - 布局 customize：拖拽（标题栏手柄）+ 右下角缩放手柄改变大小，刷新后布局保持（PATCH /agent/pins/layout 持久化）；
   - 折叠/展开 dock、新对话按钮、agent 禁用时 dock 内显示设置引导横幅；
   - 移除 RichInput 后输入条仍可正常发送（Enter + 按钮）。
