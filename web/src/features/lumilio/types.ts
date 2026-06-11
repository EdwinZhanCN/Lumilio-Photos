import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type AgentChatRequest = Schemas["handler.AgentChatRequest"];
export type AgentResumeRequest = Schemas["handler.AgentResumeRequest"];
export type ToolInfoResponse = Schemas["handler.ToolInfoResponse"];
export type AgentRefDTO = Schemas["dto.AgentRefDTO"];
export type AgentRefAssetsDTO = Schemas["dto.AgentRefAssetsDTO"];

// --- SSE wire types (not part of the OpenAPI surface) ---

/** Ref handle riding the side channel: never asset data, only the handle,
 * its cardinality and rendering hints. The frontend hydrates assets from
 * GET /api/v1/agent/refs/{id}/assets. */
export interface RefPayload {
  refId: string;
  count: number;
  widget?: "asset_grid";
  params?: { title?: string };
}

export interface SideChannelError {
  code: string;
  message: string;
  hint?: string;
}

export type ToolStatus = "running" | "success" | "error";

/** Control-plane event emitted by tool executions through the side channel. */
export interface SideChannelEvent {
  type: "tool_execution" | "widget_show";
  timestamp: number;
  tool: { name: string; executionId: string };
  execution: {
    status: ToolStatus;
    message?: string;
    error?: SideChannelError;
    parameters?: unknown;
    duration?: number;
  };
  data?: RefPayload;
}

/** Streamed agent message chunk (assistant text and/or reasoning). */
export interface AgentMessageEvent {
  agent_name?: string;
  output?: string;
  reasoning?: string;
  action?: { interrupted?: InterruptInfo; Interrupted?: InterruptInfo };
  error?: string;
}

/** eino interrupt payload requiring user confirmation before resuming. */
export interface InterruptInfo {
  data?: unknown;
  InterruptContexts: InterruptContext[];
}

export interface InterruptContext {
  ID: string;
  Address?: unknown[];
  Info?: { count?: number; confirmationId?: string; message?: string };
  IsRootCause: boolean;
}

// --- Conversation model: messages are lists of typed blocks ---

export type Block =
  | TextBlock
  | ReasoningBlock
  | ToolBlock
  | WidgetBlock
  | ConfirmBlock;

export interface TextBlock {
  kind: "text";
  id: string;
  markdown: string;
}

export interface ReasoningBlock {
  kind: "reasoning";
  id: string;
  text: string;
  startedAt: number;
  /** Seconds spent reasoning; set when the block closes. */
  durationS?: number;
}

/** One tool execution, upserted by executionId as side events stream in. */
export interface ToolBlock {
  kind: "tool";
  id: string;
  executionId: string;
  name: string;
  status: ToolStatus;
  message?: string;
  error?: SideChannelError;
  refId?: string;
  count?: number;
}

/** An explicit show-terminal render request; assets hydrate from the ref API. */
export interface WidgetBlock {
  kind: "widget";
  id: string;
  refId: string;
  count: number;
  widget: "asset_grid";
  title?: string;
}

/** Inline confirmation card for an interrupted (paused) agent run. */
export interface ConfirmBlock {
  kind: "confirm";
  id: string;
  interrupt: InterruptInfo;
  resolved?: "approved" | "rejected";
}

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  blocks: Block[];
}
