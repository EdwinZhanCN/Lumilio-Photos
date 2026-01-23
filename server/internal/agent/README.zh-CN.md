# Lumilio Agent 前端集成指南 (React + TS)

本文档旨在指导前端开发者如何使用 React 和 TypeScript 与 Lumilio 后端的 Agent 系统进行集成。

## 1. 核心概念

在开始编写代码之前，理解后端的几个核心设计至关重要。

### 1.1. 通信协议：服务器发送事件 (SSE)

后端通过 SSE (Server-Sent Events) 提供流式、实时的单向通信。前端通过 `EventSource` API 建立一个持久连接，后端会通过这个连接持续推送事件。

### 1.2. 主要 Endpoints

-   `POST /api/agent/chat`: 开启一个新对话或在现有对话（通过 `thread_id`）中发送新消息。
-   `POST /api/agent/chat/resume`: 恢复一个被中断的对话。

### 1.3. 事件类型 (Event Types)

后端会发送多种类型的事件，前端需要监听它们：

-   `session_info`: 连接建立后收到的第一个事件，包含 `thread_id`。
-   `message`: 来自 LLM 的主要文本流或工具的最终文本结果。
-   `ui_event`: **（核心功能）** 这是工具通过“侧信道”直接发送给前端的结构化数据。它通常包含了用于渲染特定UI组件的数据和指令。
-   `action`: 当 Agent 决定执行一个工具时发送的事件，其中包含了中断（Interrupt）信息。
-   `done`: 对话回合结束。
-   `error`: 发生错误。
-   `heartbeat`: 用于保持连接活跃的心跳事件。

### 1.4. UI 侧信道 (`ui_event`)

这是本系统最高级的特性之一。当一个工具（例如 `filter_assets`）执行时，它不仅会返回文本结果给 LLM，还会通过 `ui_event` 直接向前端推送一个包含丰富数据的 JSON 对象。

这个对象通常包含：
-   `data.payload`: 结构化的数据，例如一个资产（Asset）数组。
-   `data.rendering`: 一个渲染建议，告诉前端应该使用哪个组件来展示这些数据（例如 `justified_gallery`）。

这使得前端可以展示丰富的、非文本的动态内容（如图片库、表格），而这些内容是由 Agent 在后台驱动的。

### 1.5. 中断与恢复 (Interrupt & Resume)

某些工具（如 `filter_assets`）在执行前需要用户确认。在这种情况下：
1.  工具执行会暂停，并通过 `action` 事件向前端发送一个 `interrupted` 信号。
2.  这个信号中包含了 `interruptContexts`，前端需要从中提取 `ID`。
3.  前端应向用户显示一个确认对话框。
4.  如果用户同意，前端需要调用 `/api/agent/chat/resume` 接口，并在 `targets` 中传入从 `interruptContexts` 中获取的 `ID` 和确认数据。
5.  后端 Agent 将从中断点继续执行。

## 2. TypeScript 类型定义

为了确保类型安全，首先定义从后端接收到的数据结构。

`src/features/agent/assets.auth.collections.settings.upload.types.ts`:
```typescript
// --- 基本事件结构 ---

// UI 侧信道事件的核心结构
export interface SideChannelEvent {
  type: string;
  timestamp: number;
  tool: ToolIdentity;
  execution: ExecutionInfo;
  data?: DataPayload;
  extra?: ExtraInfo;
}

// Agent 主事件流中的消息事件
export interface AgentMessageEvent {
  agent_name: string;
  run_path?: string[];
  output?: string;
  reasoning?: string;
  action?: AgentAction;
  error?: string;
}

// --- SSE 事件包装 ---

export interface SseEvent<T> {
  type: 'session_info' | 'message' | 'ui_event' | 'action' | 'done' | 'error' | 'heartbeat';
  data: T;
}

export interface SessionInfoEvent {
  thread_id: string;
}

// --- 核心数据结构 ---

export interface ToolIdentity {
  name: string;
  executionId: string;
}

export interface ExecutionInfo {
  status: 'pending' | 'running' | 'success' | 'error' | 'cancelled';
  message?: string;
  error?: ErrorInfo;
  parameters?: any;
  duration?: number;
}

export interface ErrorInfo {
  code: string;
  message: string;
  details?: any;
}

export interface DataPayload {
  refId: string;
  payload_type: string;
  payload: any; // 具体类型取决于 payload_type, 例如 AssetDTO[]
  rendering?: RenderingConfig;
}

export interface RenderingConfig {
  component: 'justified_gallery' | 'data_table' | 'chart';
  config?: any; // 例如 JustifiedGalleryConfig
}

export interface ExtraInfo {
  extra_type: string;
  data: any;
}

// --- Agent Action 和中断 ---

export interface AgentAction {
  interrupted?: InterruptInfo;
  // 其他 action 类型...
}

export interface InterruptInfo {
  data: FilterConfirmationInfo;
  interruptContexts: InterruptContext[];
}

export interface InterruptContext {
  ID: string;
  Address: any[];
  Info: FilterConfirmationInfo;
  IsRootCause: boolean;
}

// 中断时，工具发给前端的信息
export interface FilterConfirmationInfo {
  count: number;
  confirmationId: string;
  message: string;
}

// --- DTOs (Data Transfer Objects) ---
// 对应后端 `server/internal/api/dto`

export interface AssetDTO {
  id: string;
  userId: string;
  assetType: 'PHOTO' | 'VIDEO' | 'AUDIO' | 'OTHER';
  filePath: string;
  thumbnailPath: string;
  encodedBlurhash?: string;
  fileSizeBytes: number;
  width: number;
  height: number;
  duration?: number;
  fps?: number;
  bitrate?: number;
  createdAt: string;
  updatedAt: string;
  takenAt?: string;
  liked: boolean;
  rating: number;
  isArchived: boolean;
  isDeleted: boolean;
  isRaw: boolean;
}
```

## 3. React 集成

### 3.1. `useAgentChat` Hook

创建一个自定义 Hook 来封装所有与 Agent 交互的逻辑，包括 `EventSource` 管理、状态管理和事件处理。

`src/features/agent/useAgentChat.ts`:
```typescript
import { useState, useEffect, useRef, useCallback } from 'react';
import { AgentMessageEvent, SideChannelEvent, SessionInfoEvent, InterruptInfo } from './types';

export type ConnectionStatus = 'disconnected' | 'connecting' | 'connected' | 'error';

export const useAgentChat = () => {
  const [messages, setMessages] = useState<Array<AgentMessageEvent | SideChannelEvent>>([]);
  const [status, setStatus] = useState<ConnectionStatus>('disconnected');
  const [threadId, setThreadId] = useState<string | null>(null);
  const [interrupt, setInterrupt] = useState<InterruptInfo | null>(null);
  
  const eventSourceRef = useRef<EventSource | null>(null);

  const closeConnection = useCallback(() => {
    if (eventSourceRef.current) {
      eventSourceRef.current.close();
      eventSourceRef.current = null;
      setStatus('disconnected');
    }
  }, []);

  const connect = useCallback((query: string, toolNames: string[] = []) => {
    closeConnection();
    setMessages([]);
    setInterrupt(null);
    setStatus('connecting');

    const body = JSON.stringify({
      thread_id: threadId || '',
      query,
      tool_names: toolNames,
    });

    const es = new EventSource(`/api/agent/chat?request=${encodeURIComponent(body)}`);
    eventSourceRef.current = es;

    es.onopen = () => setStatus('connected');
    es.onerror = () => {
      setStatus('error');
      es.close();
    };

    const handleEvent = <T>(type: string) => (event: MessageEvent) => {
        try {
            const parsedData = JSON.parse(event.data) as T;
            setMessages(prev => [...prev, { type, ...parsedData }]);
            
            if (type === 'session_info') {
                 setThreadId((parsedData as SessionInfoEvent).thread_id);
            }
            if (type === 'action' && (parsedData as AgentMessageEvent).action?.interrupted) {
                 setInterrupt((parsedData as AgentMessageEvent).action.interrupted);
            }
        } catch (e) {
            console.error(`Failed to parse ${type} event:`, e);
        }
    };
    
    es.addEventListener('session_info', handleEvent<SessionInfoEvent>('session_info'));
    es.addEventListener('message', handleEvent<AgentMessageEvent>('message'));
    es.addEventListener('ui_event', handleEvent<SideChannelEvent>('ui_event'));
    es.addEventListener('action', handleEvent<AgentMessageEvent>('action'));

    es.addEventListener('done', () => {
      // You can handle the end of a turn here
    });

    return () => {
      closeConnection();
    };
  }, [threadId, closeConnection]);

  const resume = useCallback(async (targets: Record<string, any>) => {
    if (!threadId) {
      console.error("Cannot resume without a threadId");
      return;
    }

    setInterrupt(null);
    
    // Resume is also a streaming endpoint
    // To keep it simple, we use fetch here, but you could adapt `connect` to handle it
    await fetch('/api/agent/chat/resume', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
            thread_id: threadId,
            targets: targets,
        }),
    });
    // Note: The resume endpoint will push events to the *existing* EventSource connection
    // if it's still open. For a more robust implementation, you might want to handle
    // connection state more carefully. In this example, we assume the initial `connect`
    // call established a connection that `resume` will use.

  }, [threadId]);

  return { messages, status, connect, resume, interrupt, threadId };
};
```
*Note: The example above passes the request body via query parameter for simplicity with `EventSource` which only supports GET. In a real-world scenario, you would likely initiate the SSE connection with a POST request, which is slightly more complex and might require a library or a different server setup.*

### 3.2. `AgentChatComponent.tsx`

创建一个组件来使用上面的 Hook，并渲染聊天界面。

```tsx
import React, { useState } from 'react';
import { useAgentChat } from './useAgentChat';
import { AgentMessageEvent, SideChannelEvent, AssetDTO } from './types';

// 这是一个根据 ui_event 动态渲染的组件
const DynamicUIComponent: React.FC<{ event: SideChannelEvent }> = ({ event }) => {
  const { data } = event;

  if (!data?.rendering) {
    return null;
  }
  
  // 仅作为示例
  const JustifiedGallery: React.FC<{ assets: AssetDTO[] }> = ({ assets }) => (
    <div style={{ display: 'flex', flexWrap: 'wrap', gap: '10px', padding: '10px', border: '1px solid #ccc', borderRadius: '8px' }}>
      {assets.map(asset => (
        <img key={asset.id} src={asset.thumbnailPath} alt={`Asset ${asset.id}`} style={{ height: '100px', borderRadius: '4px' }} />
      ))}
    </div>
  );

  switch (data.rendering.component) {
    case 'justified_gallery':
      const assets = data.payload as AssetDTO[];
      return (
        <div>
          <p>Found {assets.length} assets:</p>
          <JustifiedGallery assets={assets} />
        </div>
      );
    default:
      return <pre>{JSON.stringify(data.payload, null, 2)}</pre>;
  }
};


export const AgentChatComponent: React.FC = () => {
  const [input, setInput] = useState<string>('');
  const { messages, status, connect, resume, interrupt } = useAgentChat();

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    if (input.trim()) {
      connect(input);
      setInput('');
    }
  };

  const handleConfirmation = (approved: boolean) => {
    if (!interrupt) return;
    
    const rootCause = interrupt.interruptContexts.find(ctx => ctx.IsRootCause);
    if (!rootCause) {
        console.error("No root cause found in interrupt");
        return;
    }

    // `resume` a tool execution doesn't require extra data. 
    // Just targeting the ID is enough.
    const targets = { [rootCause.ID]: { approved } }; 
    resume(targets);
  };

  return (
    <div>
      <h1>Lumilio Agent</h1>
      <div style={{ border: '1px solid #ddd', padding: '10px', height: '500px', overflowY: 'auto' }}>
        {messages.map((msg, index) => {
          if ('tool' in msg) { // This is a SideChannelEvent
             return <DynamicUIComponent key={index} event={msg as SideChannelEvent} />;
          } else { // This is an AgentMessageEvent
             const agentMsg = msg as AgentMessageEvent;
             return (
                <div key={index}>
                    {agentMsg.reasoning && <p style={{ color: 'grey', fontStyle: 'italic' }}>Thinking: {agentMsg.reasoning}</p>}
                    {agentMsg.output && <p><strong>Agent:</strong> {agentMsg.output}</p>}
                </div>
             );
          }
        })}
      </div>

      {interrupt && (
        <div style={{ border: '1px solid orange', padding: '10px', margin: '10px 0' }}>
          <h4>Confirmation Required</h4>
          <p>{interrupt.data.message}</p>
          <button onClick={() => handleConfirmation(true)}>Confirm</button>
          <button onClick={() => handleConfirmation(false)}>Cancel</button>
        </div>
      )}

      <form onSubmit={handleSubmit}>
        <input
          type="text"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          placeholder="Ask the agent..."
          disabled={status === 'connecting'}
        />
        <button type="submit" disabled={status === 'connecting'}>Send</button>
      </form>
      <p>Status: {status}</p>
    </div>
  );
};
```

## 4. 总结

通过以上步骤，你可以构建一个功能强大且与 Lumilio Agent 系统深度集成的前端应用。关键在于：
1.  使用 `EventSource` 来处理 SSE 流。
2.  监听并正确处理不同类型的事件，尤其是 `message` 和 `ui_event`。
3.  根据 `ui_event` 的 `rendering` 建议来动态渲染丰富的 UI 组件。
4.  实现中断-确认-恢复的完整流程，为用户提供交互式工具执行体验。
