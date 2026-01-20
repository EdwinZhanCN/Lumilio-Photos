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
  type:
    | "session_info"
    | "message"
    | "ui_event"
    | "action"
    | "done"
    | "error"
    | "heartbeat";
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
  status: "pending" | "running" | "success" | "error" | "cancelled";
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
  component: "justified_gallery" | "data_table" | "chart";
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
  InterruptContexts: InterruptContext[];
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
