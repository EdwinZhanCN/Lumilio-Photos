import { fetchEventSource, type EventSourceMessage } from "@microsoft/fetch-event-source";
import { getToken } from "@/lib/http-commons/auth";
import { client } from "@/lib/http-commons/queryClient";
import {
  normalizeProblem,
  normalizeProblemReference,
  readProblemResponse,
} from "@/lib/http-commons/problem";
import type { components } from "@/lib/http-commons/schema";
import type {
  AgentCancelRequest,
  AgentCancelResponse,
  AgentChatRequest,
  AgentMessageEvent,
  AgentResumeRequest,
  AgentRunStatus,
  DroppedMention,
  EffectReceipt,
  InterruptInfo,
  SideChannelEvent,
} from "../model/chatTypes";

const baseUrl = import.meta.env.VITE_API_URL ?? "";

type GeneratedAgentEffectStatus = components["schemas"]["handler.AgentEffectStatusResponse"];

export type AgentEffectStatus = Omit<
  GeneratedAgentEffectStatus,
  "effect_id" | "status" | "receipt"
> & {
  effect_id: string;
  status: "pending" | "committed" | "rejected" | "cancelled" | "failed";
  receipt?: EffectReceipt;
};

export interface AgentStreamCallbacks {
  onSessionInfo: (threadId: string, runId: string, droppedMentions: DroppedMention[]) => void;
  onRunStatus: (runId: string, status: AgentRunStatus) => void;
  onChunk: (chunk: { output?: string }) => void;
  onSideEvent: (event: SideChannelEvent) => void;
  onInterrupt: (interrupt: InterruptInfo) => void;
  onDone: () => void;
  onError: (problem: unknown) => void;
}

const isInterruptInfo = (value: unknown): value is InterruptInfo => {
  if (!value || typeof value !== "object") return false;
  const interrupt = value as InterruptInfo;
  return Array.isArray(interrupt.InterruptContexts);
};

const parsePayload = (message: EventSourceMessage): unknown => {
  if (!message.data) return undefined;
  try {
    return JSON.parse(message.data);
  } catch {
    return undefined;
  }
};

const handleMessageEvent = (
  data: AgentMessageEvent | undefined,
  callbacks: AgentStreamCallbacks,
) => {
  if (!data) return;
  if (data.output) callbacks.onChunk({ output: data.output });
  const interrupt = data.action?.interrupted ?? data.action?.Interrupted;
  if (isInterruptInfo(interrupt)) callbacks.onInterrupt(interrupt);
};

const isRunStatus = (value: unknown): value is AgentRunStatus =>
  typeof value === "string" &&
  [
    "running",
    "cancel_requested",
    "awaiting_confirmation",
    "cancelled",
    "completed",
    "failed",
  ].includes(value);

const isDroppedMention = (value: unknown): value is DroppedMention => {
  if (!value || typeof value !== "object") return false;
  const item = value as Partial<DroppedMention>;
  return [item.type, item.id, item.label, item.reason].every((field) => typeof field === "string");
};

/** Requests server-side cancellation before the caller closes its local SSE
 * connection. The exact run tuple makes stale Stop requests harmless. */
export async function cancelAgentRun(body: AgentCancelRequest): Promise<AgentCancelResponse> {
  const { data, error, response } = await client.POST("/api/v1/agent/chat/cancel", { body });
  if (!response.ok || !data) {
    throw error ? normalizeProblem(error) : normalizeProblem(undefined);
  }
  return data;
}

/** Reconciles a confirmation receipt after the streaming connection ends.
 * The endpoint is part of the annotated OpenAPI surface; this helper stays next
 * to the SSE transport because it is used only to recover stream delivery. */
export async function getAgentEffectStatus(
  threadId: string,
  effectId: string,
  signal?: AbortSignal,
): Promise<AgentEffectStatus> {
  const { data, error } = await client.GET("/api/v1/agent/effects/{id}", {
    params: { path: { id: effectId }, query: { thread_id: threadId } },
    signal,
  });
  if (error || !data?.effect_id || !isEffectStatus(data.status)) {
    throw error ? normalizeProblem(error) : normalizeProblem(undefined);
  }
  const receipt = isEffectReceipt(data.receipt) ? data.receipt : undefined;
  return { ...data, effect_id: data.effect_id, status: data.status, receipt };
}

function isEffectStatus(value: unknown): value is AgentEffectStatus["status"] {
  return (
    typeof value === "string" &&
    ["pending", "committed", "rejected", "cancelled", "failed"].includes(value)
  );
}

function isEffectReceipt(value: unknown): value is EffectReceipt {
  if (!value || typeof value !== "object") return false;
  const receipt = value as Partial<EffectReceipt>;
  return (
    typeof receipt.effect_id === "string" &&
    typeof receipt.tool_name === "string" &&
    typeof receipt.status === "string" &&
    ["committed", "rejected", "cancelled", "failed"].includes(receipt.status) &&
    typeof receipt.count === "number" &&
    typeof receipt.message === "string"
  );
}

/** Opens an authenticated SSE stream against an agent endpoint and routes
 * each event type to its callback. Resolves when the stream closes. */
export async function streamAgent(
  path: "/api/v1/agent/chat" | "/api/v1/agent/chat/resume",
  body: AgentChatRequest | AgentResumeRequest,
  callbacks: AgentStreamCallbacks,
  signal?: AbortSignal,
): Promise<void> {
  const headers: Record<string, string> = { "Content-Type": "application/json" };
  const token = getToken();
  if (token) headers.Authorization = `Bearer ${token}`;

  await fetchEventSource(`${baseUrl}${path}`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
    signal,
    openWhenHidden: true,
    async onopen(response) {
      if (response.ok) return;
      throw await readProblemResponse(response);
    },
    onmessage(message) {
      const eventType = message.event || "message";
      const data = parsePayload(message);
      switch (eventType) {
        case "session_info": {
          const session = data as
            | { thread_id?: string; run_id?: string; dropped_mentions?: unknown[] }
            | undefined;
          if (session?.thread_id && session.run_id) {
            callbacks.onSessionInfo(
              session.thread_id,
              session.run_id,
              (session.dropped_mentions ?? []).filter(isDroppedMention),
            );
          }
          break;
        }
        case "run_status": {
          const run = data as { run_id?: string; status?: unknown } | undefined;
          if (run?.run_id && isRunStatus(run.status)) callbacks.onRunStatus(run.run_id, run.status);
          break;
        }
        case "message":
        case "action":
          handleMessageEvent(data as AgentMessageEvent | undefined, callbacks);
          break;
        case "side_event":
          if (data && typeof data === "object") callbacks.onSideEvent(data as SideChannelEvent);
          break;
        case "done":
          callbacks.onDone();
          break;
        case "error":
          callbacks.onError(normalizeProblemReference(data) ?? normalizeProblem(undefined));
          break;
        default:
          break; // heartbeat and forward-compatible events
      }
    },
    onerror(error) {
      throw error;
    },
  });
}
