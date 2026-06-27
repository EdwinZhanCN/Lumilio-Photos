# Discovery Agent — 主动发现与编辑式叙述

> 状态: active · 面向: 编码代理与人类贡献者
> 前置: [`agent-ref-system.md`](./agent-ref-system.md) Phase 1–3 已完成（ref 底座、代数工具、pin、conversation store、summarization）。
> 配套: Task Agent 走 [`agent-task.md`](./agent-task.md)，**两份 plan 完全独立、不互相引用对方细节**。两者只共享 ref / facet / hydration / pin 基建。
> 本计划只负责一件事：建一个**独立 agent**，它不回答用户查询，而是主动读取库的形状、自己发明 tight predicate、代数执行验证、按编辑纪律 surface 一段叙述，把最终诠释权留给用户。

## 1. 这个 agent 是什么、不是什么

**是**：agent 发起（非 query-response）→ 读库本体 → 发明 predicate → 代数跑真值 → 自筛 → 编辑式叙述 → 用户做内心诠释 → 反馈回喂。

**不是**：
- 不是 task agent（不管理库、不 pin ref 到 board、不回答「找出 X」）。
- 不复用 Task Agent 的 instruction / 工具集 / 消息模型 / 前端路由。
- 不维持内部 memory（遵守 ref-system §9d「库即记忆」原则；个性化走指令注入，不起独立 preference store）。

**失败时静默**（D-6）：一个周期无 striking 候选则不产出任何 narration。不输出「暂无发现」填充。

## 2. 共享底座的 Discovery 侧扩展

底座本身（ref store / facet engine / hydration / pin 表）见 [`agent-ref-system.md`](./agent-ref-system.md)，本计划只做 Discovery 必需的两处小扩。

### 2.1 FacetSummary 增维度（供本体推断）

Discovery 要从「库的整体形状」判断这是哪种库（回忆录 / 截图-utility / 参考-moodboard / craft-portfolio / family / 混合）。当前 FacetSummary 缺几个关键信号。`queries/agent_facets.sql` 增加：

- `hour_of_day_histogram`（24 桶）：均匀 = 随手记录，峰化 = 事件驱动。
- `source_breakdown`（exif/路径可推断：相机拍摄 vs 截图 vs 扫描 vs 录屏）：**本体推断最强信号**——截图主导的库 ≠ 回忆录。
- `views_per_asset_stats`（如有 view 事件表）：活跃浏览 vs 存了不看。

进 `FacetSummary`，对 Task Agent 无副作用（不读这些字段）。OpenAPI 注解 + `make dto`。

### 2.2 Pin 表增 narration 类型

Discovery 的产物不是 ref（一阶集合），而是**绑定到某 ref 的叙述**。

```sql
ALTER TABLE agent_pins ADD COLUMN kind text NOT NULL DEFAULT 'ref';
-- kind ∈ {'ref', 'narration'}
ALTER TABLE agent_pins ADD COLUMN narration jsonb;  -- 仅 kind='narration' 非空
```

`narration` 存结构化 `NarrationBlock`（§4.4），不是自由文本。

## 3. 契约（实现必须满足）

| # | 契约 | 校验 |
|---|---|---|
| **D-1** | **Tight predicate only**：每个 predicate 组合运算即结果定义，无解释 gap；surface 前自证 | 审计日志附 predicate 表达式 + 自证；人工 review |
| **D-2** | **编辑纪律**：只排列事实，不断因、不命名情绪、不用超数据隐喻、留白结尾；fact/inference 显式分层，inference 默认隐藏 | 确定性校验器（§4.6）拒发违规 |
| **D-3** | **代数 oracle**：每个 claim 绑定可重放的 facet；无绑定不发 | 校验器重放 evidence 必须匹配 |
| **D-4** | **harm 边界**：默认低 harm domain（习惯/器材/地理/时间/截图行为）；人物/家庭/丧失仅 opt-in，且必为标记 inference | instruction 编码 allowlist；高 harm 需前端 opt-in 凭据 |
| **D-5** | **自筛选**：每周期 surface ≤N（默认 1，可配 ≤3）；丢弃平庸 | 周期日志记录候选数/surface 数/丢弃原因；硬上限 |
| **D-6** | **失败静默**：无 striking 候选则零产出 | 周期可空完成，前端不显示空状态 |

## 4. Agent 实现规格

### 4.1 工具面（与 Task Agent 隔离）

Discovery Agent **只注册以下工具**，独立 `ToolRegistry` 实例（不复用 Task Agent 的 registry），注入同一 `RefStore`（共享数据）：

- **Read**：`describe`、`peek`、`lookup_people`、`filter_assets`、`search_*`、`combine`、`rank`、`top`、`sample`。
- **NO Write**：不注册 `bulk_like`、`create_album`、`show`。
- **唯一 Terminal**：新增 `surface_narration`——产出 `NarrationBlock`，side channel 发 `narration_surface`，落 `kind=narration` pin。

### 4.2 NarrationBlock schema

```
NarrationBlock {
  id            string
  ref_id        string
  predicate     { plan, tightness_proof string }
  lines         [{
    text        string
    evidence    { ref_id, facet_path, value }   // D-3 绑定
    kind        "fact" | "inference"             // D-2 分层；inference 默认不显示
  }]
  domain_harm   "low" | "medium" | "high"
  created_at    timestamp
}
```

### 4.3 Loop topology（触发式，无状态，无多轮）

不复用 conversation store。每周期是单次 eino agent run：

1. `describe(全库)` → 整体 FacetSummary（含 §2.1 新维度）。
2. LLM 推断本体、发明 ≤8 个 tight predicate（tool calls）。
3. 代数执行 → N 份 receipt。
4. LLM 自评 strikingness → 选 ≤N 个。
5. 生成 NarrationBlock（fact/inference 分层）。
6. 过确定性校验器（§4.4）→ 通过则 `surface_narration`。
7. 周期结束：side channel 发 0 或多条事件，落 pin。

**触发**：前端 `POST /api/v1/agent/discover`（用户主动点）。**不做 background scheduling**（v1）。

### 4.4 确定性校验器

新增 `server/internal/agent/discovery/validator.go`（纯函数 + 单测）。输入 NarrationBlock，输出 ok / 拒绝原因：

- 每个 `line.evidence` 必须能从 `ref_id` 当前 facet 重放出 `value`。
- `kind=fact` line 不得含因果连词、情绪命名词、未在 evidence 出现的实体。
- `kind=inference` line 必须有标记（前端默认隐藏）。
- `domain_harm=high` 必须附带 opt-in 凭据（请求里的 `opt_in_high_harm=true`）。

**这一层是纯代码不是 LLM**——把纪律变成可执行检查的地方。

### 4.5 Instruction 骨架

```
You are a Discovery Agent on a user's photo library.

PHASE 1 — ONTOLOGY INFERENCE
Read the overall FacetSummary. Hypothesize what KIND of library this is
(record-keeping / utility-screenshot / reference-moodboard / craft-portfolio /
family-chronicle / mixed). Your hypothesis governs which predicates are
meaningful. Do NOT assume a life-record ontology.

PHASE 2 — PREDICATE INVENTION
Invent ≤8 candidate predicates using the algebra tools. Each MUST be tight:
the operation strictly defines the result set. Write one line of self-proof
per candidate. Reject any that requires interpreting data rather than computing it.

PHASE 3 — EXECUTE & SELF-FILTER
Run them. Read receipts. Judge strikingness — a contrast, discontinuity, or
surprising count. Better to surface nothing than something dull.

PHASE 4 — NARRATE (EDITOR DISCIPLINE)
Arrange facts so meaning arises in the reader.
- Every line checkable against a facet (bind evidence).
- NEVER name emotion, NEVER assert causation, NEVER use metaphor beyond data.
- Separate fact from inference. Inference is opt-in, marked, never default.
- End with space. Reader draws the conclusion.

DOMAIN RESTRICTION
Default low-harm (habits/craft/geography/time/screenshots).
High-harm (people/family/loss) requires explicit opt-in, always marked inference.

FAILURE IS OK
If nothing is striking, surface nothing.
```

## 5. 前端（独立路由）

### 5.1 路由与组件

新路由 `/lumilio/discover`，与 chat 完全分离：

- 顶部：「开始一次发现」按钮（触发 `POST /agent/discover`）+「允许涉及人物关系」opt-in toggle（默认关，对应 D-4）。
- 主体：narration 流，每张卡片：
  - 渲染 `lines`（fact 默认显示，inference 折叠 + 「显示 agent 的推测」按钮）。
  - 每行可点开 → 展开 `evidence`（facet 路径 + 值 + 跳 hydration 资产页）。
  - 底部三按钮：`停住了` / `没感觉` / `它读错了`（→ §6 反馈）。
  - 末行固定 `[ the rest is yours ]`。
- 空状态：仅按钮 + 一句「Discovery Agent 不会主动打扰；点一下让它读你的库」。

不接入 chat dock、不接入 board、不与 Task Agent 共享消息状态。

### 5.2 状态

服务器态：TanStack Query（discover 为 mutation，narration 列表为 query on `/agent/pins?kind=narration`）。交互态：本地 Zustand（仅 opt-in + 反馈 pending）。**与 Task surface 的 store 完全独立。**

## 6. 反馈回路（第二 oracle）

无状态个性化（agent 自己不记，每次由外界递上下文）：

- 新表 `discovery_feedback`：`(user_id, narration_id, reaction ∈ {'struck','flat','wrong'}, correction_text?, created_at)`。
- 每次 `POST /agent/discover` 时，handler 取该 user 近期 N 条反馈，摘要后注入本轮 instruction（作为「过往对什么有/无反应」hint）。
- 反馈只影响 predicate 发明方向与自筛，**不影响事实层**（事实永远只由代数决定）。

## 7. 实施阶段

> 质量门：`make server-test`、`make web-test`、改 API 后 `make dto`、Go `gofmt`。

### Phase D0 — 底座扩展（前置）
- [ ] FacetSummary 加三维度（§2.1）；`make dto`。
- [ ] `agent_pins` 加 `kind`/`narration`（迁移 032）；pin handler 支持 `kind=narration` CRUD。
- 验收：新 facet 字段在 `describe(全库)` 可见；narration pin 可插取。

### Phase D1 — Discovery 后端骨架
- [ ] `server/internal/agent/discovery/`：独立 ToolRegistry（read-only + `surface_narration`）。
- [ ] Discovery instruction（§4.5）落代码（可配置）。
- [ ] `POST /api/v1/agent/discover` handler：单周期 + side channel + 落 pin。
- [ ] 确定性校验器（§4.4）+ 单测覆盖 D-2/D-3 违规。
- [ ] 审计日志：候选数/surface 数/丢弃原因/predicate 表达式。
- 验收：mock 库（Go 侧 fixture，同构 `mockWidgetData.ts`）跑一轮，D-1~D-6 全过；构造越界 narration 被校验器拒发。

### Phase D2 — Discovery 前端
- [ ] `/lumilio/discover` 路由 + 卡片组件（fact/inference 分层、evidence 展开）。
- [ ] opt-in toggle + 三按钮反馈 + correction 输入。
- 验收：触发一轮见 narration；点 evidence 跳资产；反应落库。

### Phase D3 — 反馈回路闭环
- [ ] discover handler 取近期反馈、摘要、注入 instruction。
- [ ] 回归测试：反馈只影响 predicate 方向，不影响事实层。
- 验收：连续两轮，第二轮候选分布受第一轮反馈影响（抽检 instruction 注入段 + 候选日志）。

## 8. 不做（出界）

- ❌ background scheduling（v1 纯触发）。
- ❌ 写工具（不改库、不 pin ref；用户想行动自行切 Task surface）。
- ❌ Memory System（库即记忆；反馈走指令注入）。
- ❌ v1 vision（后续 producer，进 read 工具面，不影响架构）。
- ❌ 与 Task surface 的交叉入口（v1；v2 视需求）。

## 9. Open Questions

1. 最终命名（用户面向，需与 Task Agent 区分）。
2. instruction 少样本：放不放「facet 形状 → 本体」少样本？可能稳，可能过拟合。先空。
3. tight predicate 自证可靠性：LLM 自证可能误判。D1 验收人工审计一批，看是否补确定性 tightness 静态检查器。
4. N（每轮 surface 上限）调参：默认 1。需真实库试。
5. background scheduling 触发条件（v2）：闲置阈值？入库事件阈值？
6. 反馈摘要注入预算：多少条、压成多少 token。需实验。
