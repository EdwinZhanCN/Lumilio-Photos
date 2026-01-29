import type { components } from "@/lib/http-commons/schema";

type Schemas = components["schemas"];

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

export type AgentChatRequest = Schemas["handler.AgentChatRequest"];
export type AgentResumeRequest = Schemas["handler.AgentResumeRequest"];
export type ToolInfoResponse = Schemas["handler.ToolInfoResponse"];

export type AgentEventType =
  | "session_info"
  | "message"
  | "action"
  | "ui_event"
  | "done"
  | "error"
  | "heartbeat";

export interface AgentStreamEvent {
  type: AgentEventType;
  data: unknown;
}
