# Lumilio Agent 设计与集成指南

`server/internal/agent` 是当前仓库中的 Agent 运行时实现。它基于 Eino ADK，负责把 LLM、工具调用、可恢复会话、侧信道事件和前端消费协议组合起来。

这份文档的目标不是只解释某个页面怎么接，而是把这个模块当成“未来可独立抽出的 Go package”来描述：

- 它解决什么问题
- 它的核心抽象是什么
- 当前仓库中的 HTTP/SSE 适配层如何接入
- 前端如何消费 `side_event`
- 新工具应该怎么写
- 如果后续独立成 package，哪些边界应当稳定下来

## 1. 模块定位

这个 Agent 模块当前承担 5 类职责：

1. 管理 LLM 与工具调用流程。
2. 通过 CheckPointStore 持久化会话，使对话可恢复。
3. 允许工具把结构化数据通过侧信道直接发给前端，而不必绕回 LLM 文本。
4. 在工具之间传递结构化引用，而不是把完整对象暴露给 LLM。
5. 为前端提供一个稳定的流式事件协议。

它不是一个 UI 框架，也不是一个 HTTP 框架。当前仓库里用了 Gin + SSE 作为适配层，但 Agent 核心本身更适合被视为一个“运行时内核”。

## 2. 当前目录结构

```text
server/internal/agent/
├── core/
│   ├── agent_service.go        # AgentService、Runner 组装
│   ├── tool_registry.go        # ToolRegistry、SideChannelEvent、ToolDependencies
│   ├── agent_reference.go      # ReferenceManager、Reference[T]
│   ├── agent_store.go          # Postgres CheckPointStore
│   ├── tool_type_converter.go  # 引用类型转换
│   └── tool_gob_register.go    # 会话持久化所需注册
├── tools/
│   ├── asset_filter.go         # 示例：生成过滤条件并通过 side_event 通知前端
│   └── bulk_like.go            # 示例：消费 ref_id 做批量写入
└── README.zh-CN.md
```

当前还有一些 `eino-*.md` 是内部调研记录，不是对外契约的一部分。

## 3. 核心抽象

### 3.1. AgentService

`core.AgentService` 是这个模块最核心的服务接口。当前实现提供四类能力：

- `AskAgent`: 发起新一轮对话或继续已有线程。
- `ResumeAgent`: 从中断点恢复执行。
- `GetAvailableTools`: 返回当前注册的工具元数据。
- `AskLLM`: 不走工具链路，直接与模型交互。

当前仓库里的服务签名保留了一个可选的 `sideChannels ...chan<- *SideChannelEvent` 参数，这使得调用方可以在不强绑定某个传输协议的前提下接收结构化事件。

```go
type AgentService interface {
    AskAgent(
        ctx context.Context,
        threadID, query string,
        toolNames []string,
        sideChannels ...chan<- *SideChannelEvent,
    ) *adk.AsyncIterator[*adk.AgentEvent]

    ResumeAgent(
        ctx context.Context,
        threadID string,
        params *adk.ResumeParams,
        sideChannels ...chan<- *SideChannelEvent,
    ) (*adk.AsyncIterator[*adk.AgentEvent], error)
}
```

### 3.2. ToolRegistry 与 ToolDependencies

工具不是直接硬编码到 Agent 里的，而是通过 `ToolRegistry` 注册。

注册时有两个输入：

- `schema.ToolInfo`: 给 LLM 和外部调用方看的工具元信息
- `ToolFactory`: 真正构造工具实例的工厂

工具运行时依赖通过 `ToolDependencies` 注入，当前包括：

- `Queries`: 业务数据访问能力
- `SideChannel`: 向前端发送结构化事件
- `ReferenceManager`: 在工具之间存取引用

这意味着：

- Agent 运行时可以保持通用
- 工具依赖可以按请求隔离
- 是否发送 `side_event` 是工具自己的能力，不是前端额外拼接的副作用

### 3.3. SideChannelEvent

`SideChannelEvent` 是当前模块里最重要的前后端契约之一。它表示“工具执行过程中产生的结构化侧效果”。

注意这里故意不用 `ui_event` 命名，因为它的语义不止是“渲染 UI”，还可以是：

- 工具执行进度
- 结构化结果
- 前端动作提示
- 非文本副作用的数据载体

当前结构如下：

```go
type SideChannelEvent struct {
    Type      string                 `json:"type"`
    Timestamp int64                  `json:"timestamp"`
    Tool      ToolIdentity           `json:"tool"`
    Execution ExecutionInfo          `json:"execution"`
    Data      *DataPayload           `json:"data,omitempty"`
    Extra     *ExtraInfo             `json:"extra,omitempty"`
    Metadata  map[string]interface{} `json:"metadata,omitempty"`
}
```

其中几个关键字段的语义是：

- `tool.executionId`: 同一次工具执行的稳定标识，前端应该用它做关联和去重。
- `execution.status`: `pending | running | success | error | cancelled`
- `data.payload`: 结构化业务数据
- `data.rendering`: 可选的渲染提示，而不是强制 UI 协议

### 3.4. ReferenceManager 与 `Reference[T]`

这是当前模块里另一个关键设计：LLM 不擅长可靠地传递大对象，但很适合传递字符串 ID。

因此当前实现采用了“引用”模型：

1. 工具 A 把某个结构化结果放入会话，获得一个类似 `ref.filter_assets.asset_filter.<suffix>` 的引用
2. LLM 在后续工具调用里只传这个引用字符串
3. 工具 B 用 `Reference[T]` + `ReferenceManager` 自动解引用成目标类型

这让工具之间可以共享复杂结构，而不要求 LLM 重新生成整个对象。

当前仓库里：

- `filter_assets` 会存储 `AssetFilterDTO` 引用
- `bulk_like_assets` 会接收 `Reference[dto.AssetFilterDTO]`

这类引用数据被存储在 ADK Session 中，因此只要启用了 CheckPointStore，就可以随着会话一起持久化和恢复。

### 3.5. CheckPointStore 与可恢复会话

Agent 的可恢复能力不是前端自己维护出来的，而是依赖 Eino ADK 的 checkpoint 机制。

当前实现：

- `agent_service.go` 用 `adk.NewRunner(..., CheckPointStore: s.store)`
- `agent_store.go` 提供基于 Postgres 的 `Get/Set`
- `ResumeAgent` 用同样的配置重建 Agent，再调用 `ResumeWithParams`

这意味着以下状态都能在恢复时保留：

- 对话线程
- 中断点
- 会话变量
- `ReferenceManager` 写入的引用

## 4. 当前运行时流程

下面是当前仓库里一轮对话的真实执行路径：

1. HTTP 适配层接收 `POST /api/v1/agent/chat`
2. 如果没有 `thread_id`，服务端生成新的线程 ID
3. HTTP 层创建 `sideChannel := make(chan *core.SideChannelEvent, 100)`
4. 调用 `agentService.AskAgent(..., sideChannel)`
5. Agent 执行过程中：
   - LLM 文本通过 ADK 事件流返回
   - 工具结构化事件通过 `sideChannel` 返回
6. HTTP 适配层把两类输出都转成 SSE：
   - `message`
   - `side_event`
7. 前端消费 SSE，并把 `side_event` 挂到当前 assistant message 上
8. 如果未来某个工具触发中断，前端可调用 `POST /api/v1/agent/chat/resume`

这个设计的关键点是：文本流和结构化副作用是两条并行通路，但它们共享同一个 `thread_id` 和同一轮 Agent 执行上下文。

## 5. 当前仓库中的 HTTP / SSE 契约

Agent 核心本身不要求一定使用 SSE，但当前仓库的 HTTP 适配层确实就是 SSE，并且前端已经基于它工作。

### 5.1. 端点

当前适配层位于：

- `server/internal/api/handler/agent_handler.go`

暴露的主要端点是：

- `POST /api/v1/agent/chat`
- `POST /api/v1/agent/chat/resume`
- `GET /api/v1/agent/tools`

其中前两个返回 `text/event-stream`。

### 5.2. SSE 事件类型

当前实际使用的 SSE 事件类型如下：

| 事件名 | 用途 |
| --- | --- |
| `session_info` | 返回当前线程 ID |
| `message` | 用户可见的 LLM 文本流；也可能携带 `action` 字段 |
| `side_event` | 工具通过侧信道直接发送的结构化事件 |
| `done` | 当前回合结束 |
| `error` | 出错 |
| `heartbeat` | 保活 |

需要注意一个当前实现细节：

- 前端类型里保留了 `action`
- 但当前 Gin 适配层并不会单独发 `event: action`
- 当前实现会把 `event.Action` 塞进 `message.data.action`

也就是说，前端要以 `message` 里的 `action` 字段为准，而不是假定一定会收到独立的 `action` SSE 帧。

### 5.3. 当前 payload 形状

`message` 一般长这样：

```json
{
  "agent_name": "Photo Asset Assistant",
  "run_path": ["Photo Asset Assistant"],
  "reasoning": "我先帮你筛选最近喜欢的 RAW 照片。",
  "output": "我已经为你配置好了筛选条件。",
  "action": {
    "interrupted": {
      "data": {
        "count": 120,
        "confirmationId": "confirm_xxx",
        "message": "需要确认后才能继续执行。"
      },
      "InterruptContexts": []
    }
  }
}
```

`side_event` 一般长这样：

```json
{
  "type": "tool_execution",
  "timestamp": 1742947200000,
  "tool": {
    "name": "filter_assets",
    "executionId": "1742947200000000000"
  },
  "execution": {
    "status": "success",
    "message": "Filter applied successfully",
    "duration": 16
  },
  "data": {
    "refId": "ref.filter_assets.asset_filter.550e8400e29b41d4a716446655440000",
    "payload_type": "AssetFilterDTO",
    "payload": {
      "liked": true
    },
    "rendering": {
      "component": "justified_gallery",
      "config": {
        "groupBy": "date"
      }
    }
  }
}
```

### 5.4. 为什么前端这里更适合 `@microsoft/fetch-event-source`

当前后端使用的是：

- `POST`
- `Content-Type: application/json`
- SSE 响应流

浏览器原生 `EventSource` 只支持 `GET`，不适合当前这个接口形状。  
因此前端应该优先使用 `@microsoft/fetch-event-source`，而不是做 query string hack。

## 6. 前端集成示例：使用 `@microsoft/fetch-event-source`

当前前端真实实现位于：

- `web/src/features/lumilio/LumilioChatProvider.tsx`
- `web/src/features/lumilio/schema.ts`
- `web/src/features/lumilio/agent.type.ts`

下面是按当前实现整理后的推荐写法。

### 6.1. 定义事件类型

```ts
export type AgentEventType =
  | "session_info"
  | "message"
  | "action"
  | "side_event"
  | "done"
  | "error"
  | "heartbeat";

export interface AgentStreamEvent {
  type: AgentEventType;
  data: unknown;
}
```

这里保留 `action` 主要是为了兼容未来独立事件类型；当前仓库的 Gin 适配层实际上仍以 `message.data.action` 为主。

### 6.2. 解析 SSE 消息

```ts
import {
  fetchEventSource,
  type EventSourceMessage,
} from "@microsoft/fetch-event-source";

const parseAgentStreamEvent = (
  message: EventSourceMessage,
): AgentStreamEvent | null => {
  const eventType = (message.event || "message") as AgentEventType;

  if (eventType === "heartbeat" || !message.data) {
    return null;
  }

  let data: unknown = message.data;
  try {
    data = JSON.parse(message.data);
  } catch {
    // 如果服务端返回的不是 JSON，这里可以保留原始字符串
  }

  return { type: eventType, data };
};
```

### 6.3. 发起聊天流

```ts
async function streamAgent(
  path: string,
  body: unknown,
  onEvent: (event: AgentStreamEvent) => void,
  signal?: AbortSignal,
) {
  await fetchEventSource(path, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body),
    signal,
    async onopen(response) {
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
    },
    onmessage(message) {
      const event = parseAgentStreamEvent(message);
      if (event) {
        onEvent(event);
      }
    },
    onerror(error) {
      throw error;
    },
  });
}
```

### 6.4. 处理事件

```ts
function handleAgentStreamEvent(event: AgentStreamEvent) {
  switch (event.type) {
    case "session_info":
      // 保存 thread_id
      break;

    case "message":
      // 处理 output / reasoning
      // 同时检查 event.data.action?.interrupted
      break;

    case "side_event":
      // 存储到当前 assistant message 的 sideEvents
      break;

    case "done":
      // 当前回合结束
      break;

    case "error":
      // 显示错误
      break;
  }
}
```

### 6.5. 当前前端对 `side_event` 的组织方式

当前 Lumilio 前端并没有把 `side_event` 当成一条普通聊天消息，而是把它挂在对应 assistant message 上：

```ts
type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  sideEvents: SideChannelEvent[];
};
```

这样做的好处是：

- 文本流仍然保持“对话”形态
- 工具结果与对应 assistant message 有稳定关联
- 前端可以通过 `tool.executionId` 渲染内联组件

当前实现里，reducer 还会在 markdown 中插入：

```html
<lumilio-tool id="..."></lumilio-tool>
```

随后再根据 `executionId` 去 `sideEvents` 里查找对应事件并渲染具体组件。

## 7. 工具开发约定

当前仓库中的两个示例工具已经覆盖了两类典型模式：

- `filter_assets`: 生成结构化筛选条件，并通过 `side_event` 告诉前端如何展示结果
- `bulk_like_assets`: 消费上一步的 `ref_id`，执行批量写入，再通过 `side_event` 报告结果

### 7.1. 一个工具至少应该明确两条输出通路

每个工具都应该想清楚：

1. 给 LLM 的返回值是什么
2. 给前端的 `side_event` 是什么

一般建议：

- 面向 LLM：返回简洁文本或摘要
- 面向前端：发送结构化数据、进度、状态和可选渲染提示

不要把所有前端所需信息都塞回 LLM 文本里再让前端解析。

### 7.2. 推荐事件节奏

一个有状态工具通常应按这个顺序发事件：

1. `pending`
2. `running`
3. `success` 或 `error`

这正是 `filter_assets` 和 `bulk_like_assets` 现在的做法。

### 7.3. 最小工具示例

```go
func RegisterExampleTool() {
    info := &schema.ToolInfo{
        Name: "example_tool",
        Desc: "Return a short text result and emit a side event",
    }

    core.GetRegistry().Register(info, func(
        ctx context.Context,
        deps *core.ToolDependencies,
    ) (tool.BaseTool, error) {
        return utils.InferTool(info.Name, info.Desc, func(
            ctx context.Context,
            input *ExampleInput,
        ) (*ExampleOutput, error) {
            execID := fmt.Sprintf("%d", time.Now().UnixNano())

            if deps.SideChannel != nil {
                deps.SideChannel <- &core.SideChannelEvent{
                    Type:      "tool_execution",
                    Timestamp: time.Now().UnixMilli(),
                    Tool: core.ToolIdentity{
                        Name:        "example_tool",
                        ExecutionID: execID,
                    },
                    Execution: core.ExecutionInfo{
                        Status:  core.ExecutionStatusRunning,
                        Message: "Processing example request...",
                    },
                }
            }

            return &ExampleOutput{
                Message: "Example tool finished",
            }, nil
        })
    })
}
```

### 7.4. 关于 `ref_id`

`ref_id` 的目标是让工具之间共享结构化数据，不是让最终用户理解内部实现。

当前推荐格式是：

```text
ref.{source_tool}.{kind}.{opaque_suffix}
```

例如：

```text
ref.filter_assets.asset_filter.550e8400e29b41d4a716446655440000
```

其中：

- `source_tool` 表示是谁产出的
- `kind` 表示这个引用代表什么语义对象
- `opaque_suffix` 保证唯一性，不应该被业务逻辑解析

因此推荐约定是：

- 工具可以返回 `ref_id` 给 LLM
- LLM 可以在后续工具调用里使用 `ref_id`
- 最终用户界面不应依赖用户手工输入 `ref_id`

配套的 `ReferenceMeta` 中建议至少包含：

- `source_tool`
- `kind`
- `type_name`
- `description`

这也是当前 `agent_service.go` 在 system instruction 里明确限制的事情。

## 8. 中断与恢复

当前模块已经具备恢复能力，但需要分两层理解：

### 8.1. 运行时层面

`ResumeAgent` 已经完整支持：

- 用原线程 ID 恢复
- 从 CheckPointStore 加载会话
- 根据 `ResumeParams.Targets` 恢复中断节点

### 8.2. 协议层面

当前 Gin 适配层会把 `event.Action` 序列化到 `message.data.action`。  
所以前端收到 `message` 时，应检查：

```ts
eventData.action?.interrupted
```

然后再决定是否展示确认 UI 并调用：

```http
POST /api/v1/agent/chat/resume
```

请求体：

```json
{
  "thread_id": "xxx",
  "targets": {
    "interrupt_id": {
      "approved": true
    }
  }
}
```

需要说明的是：当前仓库里的示例工具主要展示了 side event 和 reference 流程，并没有把“中断工具”作为主示例，因此未来如果把这个模块抽成独立 package，建议补一个最小 interrupt 示例工具。

## 9. 后端接入示例

当前仓库中的典型接入方式大致如下：

```go
func setupAgent(queries *repo.Queries, provider core.LLMConfigProvider) {
    tools.RegisterFilterAsset()
    tools.RegisterBulkLikeTool()

    agentService := core.NewAgentService(queries, provider)
    _ = agentService
}
```

再由 HTTP 层做适配：

```go
agentHandler := handler.NewAgentHandler(agentService)

r.POST("/api/v1/agent/chat", agentHandler.Chat)
r.POST("/api/v1/agent/chat/resume", agentHandler.ResumeChat)
r.GET("/api/v1/agent/tools", agentHandler.GetTools)
```

如果未来独立成 package，推荐把这两层分开：

- 核心运行时：agent package
- 传输适配层：可选 `adapter/gin`、`adapter/http`

## 10. 作为独立 Go package 时的建议边界

如果这个模块后续要抽成独立包，建议优先稳定下面这些边界。

### 10.1. 建议保留为公共 API 的部分

- `AgentService`
- `ToolRegistry`
- `ToolDependencies`
- `SideChannelEvent`
- `ReferenceManager`
- `Reference[T]`
- 通用的 checkpoint 接口

### 10.2. 建议迁移为可选适配层的部分

- Gin handler
- Postgres checkpoint 实现
- 当前仓库内的 repo / dto / llm 依赖

更具体地说，当前这些点是“仓库耦合”，不适合直接当作独立 package 的核心公共接口：

- `server/config`
- `server/internal/llm`
- `server/internal/db/repo`
- `server/internal/api/dto`

更理想的方向是：

- 用 model factory 替代 `LLMConfigProvider`
- 用业务接口替代 `*repo.Queries`
- 把示例工具放到 example 或单独模块
- 把 HTTP/SSE 放到 adapter 层

### 10.3. 建议稳定下来的协议契约

一旦独立 package 化，下列内容就应视为外部契约，尽量避免频繁破坏性变更：

- `thread_id`
- `side_event` 事件名
- `SideChannelEvent` 的 JSON 字段
- `tool.executionId`
- `execution.status`
- `data.rendering` 的语义是“提示”，不是“强制”

## 11. 实践建议

最后给几个在当前代码基础上比较重要的约定：

1. 把 `side_event` 当成一等协议，不要再退回 `ui_event` 语义。
2. 前端优先使用 `@microsoft/fetch-event-source`，因为当前接口是 POST + SSE。
3. 让工具显式区分“给 LLM 的返回值”和“给前端的 side event”。
4. 让 `ref_id` 采用“语义前缀 + 不透明后缀”的格式，但不要把完整语义都塞进 ID。
5. 让 `ref_id` 留在工具链内部，而不是暴露给用户操作。
6. 在抽包前，优先把 repo、llm、gin、postgres 这几类仓库耦合从核心 API 中剥离。

如果未来这个模块被提取为独立 package，那么这份文档应该继续保留，但路径和示例代码应迁移为：

- 根 README：讲核心抽象和稳定契约
- `examples/`: 讲 Gin / SSE / React 集成
- `adapters/`: 放传输和存储适配层
