import { fetchEventSource, type EventSourceMessage } from "@microsoft/fetch-event-source";
import { getToken } from "@/lib/http-commons/auth";
import { client } from "@/lib/http-commons/queryClient";
import type {
  AgentCancelRequest,
  AgentCancelResponse,
  AgentChatRequest,
  AgentMessageEvent,
  AgentResumeRequest,
  AgentRunStatus,
  InterruptInfo,
  SideChannelEvent,
} from "../model/chatTypes";

const baseUrl = import.meta.env.VITE_API_URL ?? "";

export interface AgentStreamCallbacks {
  onSessionInfo: (threadId: string, runId: string) => void;
  onRunStatus: (runId: string, status: AgentRunStatus) => void;
  onChunk: (chunk: { output?: string }) => void;
  onSideEvent: (event: SideChannelEvent) => void;
  onInterrupt: (interrupt: InterruptInfo) => void;
  onDone: () => void;
  onError: (message: string) => void;
}

const isInterruptInfo = (value: unknown): value is InterruptInfo => {
  if (!value || typeof value !== "object") return false;
  const interrupt = value as InterruptInfo;
  return Array.isArray(interrupt.InterruptContexts);
};

const getErrorMessage = (data: unknown): string => {
  if (typeof data === "string" && data) return data;
  if (data && typeof data === "object" && "error" in data) {
    const message = (data as { error?: string }).error;
    if (message) return message;
  }
  return "Unknown error";
};

const parsePayload = (message: EventSourceMessage): unknown => {
  if (!message.data) return undefined;
  try {
    return JSON.parse(message.data);
  } catch {
    return message.data;
  }
};

const handleMessageEvent = (
  data: AgentMessageEvent | undefined,
  callbacks: AgentStreamCallbacks,
) => {
  if (!data) return;
  if (data.output) {
    callbacks.onChunk({ output: data.output });
  }
  const interrupt = data.action?.interrupted ?? data.action?.Interrupted;
  if (isInterruptInfo(interrupt)) {
    callbacks.onInterrupt(interrupt);
  }
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

/** Requests server-side cancellation before the caller closes its local SSE
 * connection. The exact run tuple makes stale Stop requests harmless. */
export async function cancelAgentRun(body: AgentCancelRequest): Promise<AgentCancelResponse> {
  const { data, error, response } = await client.POST("/api/v1/agent/chat/cancel", { body });
  if (!response.ok || !data) {
    throw new Error(getErrorMessage(error) || `HTTP ${response.status}`);
  }
  return data;
}

/** Opens an authenticated SSE stream against an agent endpoint and routes
 * each event type to its callback. Resolves when the stream closes. */
export async function streamAgent(
  path: "/api/v1/agent/chat" | "/api/v1/agent/chat/resume",
  body: AgentChatRequest | AgentResumeRequest,
  callbacks: AgentStreamCallbacks,
  signal?: AbortSignal,
): Promise<void> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };
  const token = getToken();
  if (token) {
    headers.Authorization = `Bearer ${token}`;
  }

  await fetchEventSource(`${baseUrl}${path}`, {
    method: "POST",
    headers,
    body: JSON.stringify(body),
    signal,
    openWhenHidden: true,
    async onopen(response) {
      if (!response.ok) {
        throw new Error(`HTTP ${response.status}`);
      }
    },
    onmessage(message) {
      const eventType = message.event || "message";
      const data = parsePayload(message);
      switch (eventType) {
        case "session_info": {
          const session = data as { thread_id?: string; run_id?: string } | undefined;
          if (session?.thread_id && session.run_id) {
            callbacks.onSessionInfo(session.thread_id, session.run_id);
          }
          break;
        }
        case "run_status": {
          const run = data as { run_id?: string; status?: unknown } | undefined;
          if (run?.run_id && isRunStatus(run.status)) {
            callbacks.onRunStatus(run.run_id, run.status);
          }
          break;
        }
        case "message":
        case "action":
          handleMessageEvent(data as AgentMessageEvent | undefined, callbacks);
          break;
        case "side_event":
          if (data && typeof data === "object") {
            callbacks.onSideEvent(data as SideChannelEvent);
          }
          break;
        case "done":
          callbacks.onDone();
          break;
        case "error":
          callbacks.onError(getErrorMessage(data));
          break;
        default:
          break; // heartbeat and unknown events
      }
    },
    onerror(error) {
      throw error;
    },
  });
}
