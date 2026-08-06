import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];

export type AgentChatRequest = Schemas["handler.AgentChatRequest"];
export type AgentResumeRequest = Schemas["handler.AgentResumeRequest"];
export type AgentCancelRequest = Schemas["handler.AgentCancelRequest"];
export type AgentCancelResponse = Schemas["handler.AgentCancelResponse"];
export type AgentMode = AgentChatRequest["mode"];
export type ToolInfoResponse = Schemas["handler.ToolInfoResponse"];
export type AgentRefDTO = Schemas["dto.AgentRefDTO"];
export type AgentRefAssetsDTO = Schemas["dto.AgentRefAssetsDTO"];

// SSE wire types are not part of the OpenAPI surface.

/** Ref handle riding the side channel: never asset data, only the handle,
 * its cardinality and rendering hints. The frontend hydrates assets from
 * GET /api/v1/agent/refs/{id}/assets. */
export interface RefPayload {
  refId: string;
  count: number;
  widget?: string;
  params?: { title?: string; [key: string]: unknown };
}

export interface SideChannelError {
  code: string;
  message: string;
  hint?: string;
}

export type ToolStatus = "running" | "success" | "error" | "cancelled";
export type AgentRunStatus =
  | "running"
  | "cancel_requested"
  | "awaiting_confirmation"
  | "cancelled"
  | "completed"
  | "failed";

/** Durable result of a confirmation-gated mutation. This receipt, rather than
 * the resume request or transport status, is the authority for what happened. */
export interface EffectReceipt {
  effect_id: string;
  tool_name: string;
  status: "committed" | "rejected" | "cancelled" | "failed";
  count: number;
  album_id?: number;
  message: string;
  already_committed?: boolean;
}

/** Control-plane event emitted by tool executions through the side channel. */
export interface SideChannelEvent {
  type: "tool_execution" | "widget_show" | "token_usage" | "effect_receipt";
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
  usage?: TokenUsageInfo;
  receipt?: EffectReceipt;
}

/** Token accounting of the last model call; prompt tokens are effectively
 * the conversation's current context size. */
export interface TokenUsageInfo {
  promptTokens: number;
  completionTokens: number;
  totalTokens: number;
}

/** Streamed public Agent message chunk. Provider reasoning is never part of
 * the product SSE contract. */
export interface AgentMessageEvent {
  agent_name?: string;
  output?: string;
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
  Info?: ConfirmationInfo;
  IsRootCause: boolean;
}

export interface ConfirmationInfo {
  effect_id?: string;
  EffectID?: string;
  action?: string;
  Action?: string;
  confirmationId?: string;
  ConfirmationId?: string;
  count?: number;
  Count?: number;
  message?: string;
  Message?: string;
  ref_id?: string;
  RefID?: string;
  title?: string;
  Title?: string;
}

export interface DroppedMention {
  type: string;
  id: string;
  label: string;
  reason: string;
}

export interface TurnContextSnapshot {
  id: string;
  type: "selection" | "viewing";
  label: string;
  count: number;
}

export interface TurnMentionSnapshot {
  type: string;
  id: string;
  label: string;
  status: "accepted" | "dropped";
  reason?: string;
}

/** Immutable copy of exactly what the user sent with one turn. It keeps the
 * visible conversation aligned with the server's scoped request. */
export interface AgentTurnSnapshot {
  mode: AgentMode;
  context: TurnContextSnapshot[];
  mentions: TurnMentionSnapshot[];
}

// --- Conversation model: messages are lists of typed blocks ---

export type Block = TextBlock | ToolBlock | WidgetBlock | ConfirmBlock;

export interface TextBlock {
  kind: "text";
  id: string;
  markdown: string;
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
  widget: string;
  title?: string;
  params?: Record<string, unknown>;
}

export type ConfirmationState =
  | "pending"
  | "submitting_approval"
  | "submitting_rejection"
  | "committed"
  | "rejected"
  | "failed"
  | "cancelled";

/** Inline confirmation card for an interrupted (paused) agent run. */
export interface ConfirmBlock {
  kind: "confirm";
  id: string;
  interrupt: InterruptInfo;
  state: ConfirmationState;
  receipt?: EffectReceipt;
  error?: string;
}

export interface ChatMessage {
  id: string;
  role: "user" | "assistant";
  blocks: Block[];
  /** Present only on user messages and never mutated except to mark a mention
   * dropped after the server returns authoritative binding metadata. */
  request?: AgentTurnSnapshot;
  status?: "stopped";
}
