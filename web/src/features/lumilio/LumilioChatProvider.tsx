import {
  createContext,
  useReducer,
  ReactNode,
  useEffect,
  useCallback,
  useRef,
} from "react";
import {
  fetchEventSource,
  type EventSourceMessage,
} from "@microsoft/fetch-event-source";
import { LumilioChatContextValue } from "./lumilio.type.ts";
import { lumilioReducer, initialState } from "./lumilio.reducer";
import { $api } from "@/lib/http-commons/queryClient";
import type {
  AgentEventType,
  AgentStreamEvent,
  ApiResult,
  AgentChatRequest,
  AgentResumeRequest,
  ToolInfoResponse,
} from "./agent.type";
import type {
  AgentMessageEvent,
  InterruptInfo,
  SessionInfoEvent,
  SideChannelEvent,
} from "./schema";
import type { MentionEntity } from "./components/RichInput";

const baseUrl = import.meta.env.VITE_API_URL || "http://localhost:8080";

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
  } catch (error) {
    if (import.meta.env.DEV) {
      console.warn("Failed to parse SSE payload:", error);
    }
  }

  return { type: eventType, data };
};

const buildSlashCommands = (
  tools: ToolInfoResponse[] | undefined,
): MentionEntity[] => {
  if (!tools || !Array.isArray(tools)) {
    return [];
  }

  return tools
    .filter((tool) => tool.name)
    .map((tool) => ({
      id: tool.name!,
      label: tool.name!,
      type: "command",
      meta: tool.desc || "No description available",
      desc: tool.desc,
    }));
};

const getErrorMessage = (data: unknown): string => {
  if (!data) return "Unknown error";
  if (typeof data === "string") return data;
  if (typeof data === "object" && data && "error" in data) {
    const message = (data as { error?: string }).error;
    if (message) return message;
  }
  return "Unknown error";
};

const isInterruptInfo = (value: unknown): value is InterruptInfo => {
  if (!value || typeof value !== "object") return false;
  const interrupt = value as InterruptInfo;
  return (
    "data" in interrupt &&
    "InterruptContexts" in interrupt &&
    Array.isArray(interrupt.InterruptContexts)
  );
};

export const LumilioChatContext = createContext<
  LumilioChatContextValue | undefined
>(undefined);

export const LumilioChatProvider = ({ children }: { children: ReactNode }) => {
  const [state, dispatch] = useReducer(lumilioReducer, initialState);

  const toolsQuery = $api.useQuery("get", "/api/v1/agent/tools", {}, {
    retry: false,
  });
  const toolsLoadingRef = useRef(false);

  useEffect(() => {
    if (toolsQuery.isLoading && !toolsLoadingRef.current) {
      toolsLoadingRef.current = true;
      dispatch({ type: "FETCH_TOOLS_START" });
    }
    if (!toolsQuery.isLoading) {
      toolsLoadingRef.current = false;
    }
  }, [toolsQuery.isLoading, dispatch]);

  useEffect(() => {
    if (toolsQuery.isError) {
      console.error("Failed to fetch agent tools:", toolsQuery.error);
      dispatch({ type: "FETCH_TOOLS_SUCCESS", payload: [] });
      return;
    }

    if (!toolsQuery.data) return;

    const response =
      toolsQuery.data as ApiResult<ToolInfoResponse[]> | undefined;
    const commands = buildSlashCommands(response?.data);
    dispatch({ type: "FETCH_TOOLS_SUCCESS", payload: commands });
  }, [toolsQuery.data, toolsQuery.isError, toolsQuery.error, dispatch]);

  const handleAgentMessageEvent = useCallback(
    (eventData: AgentMessageEvent | undefined) => {
      if (!eventData) return;

      if (eventData.output || eventData.reasoning) {
        dispatch({
          type: "PROCESS_STREAM_CHUNK",
          payload: {
            output: eventData.output,
            reasoning: eventData.reasoning,
          },
        });
      }

      const interrupt =
        eventData.action?.interrupted ??
        (eventData.action as { Interrupted?: unknown } | undefined)?.Interrupted;
      if (isInterruptInfo(interrupt)) {
        dispatch({ type: "RECEIVE_INTERRUPT", payload: interrupt });
      }
    },
    [dispatch],
  );

  const handleAgentStreamEvent = useCallback(
    (event: AgentStreamEvent) => {
      if (!event) return;

      switch (event.type) {
        case "session_info": {
          const data = event.data as SessionInfoEvent | undefined;
          if (data?.thread_id) {
            dispatch({
              type: "CHAT_CONNECT_SUCCESS",
              payload: { threadId: data.thread_id },
            });
          }
          break;
        }
        case "message":
        case "action": {
          handleAgentMessageEvent(event.data as AgentMessageEvent);
          break;
        }
        case "ui_event":
          dispatch({
            type: "RECEIVE_UI_EVENT",
            payload: event.data as SideChannelEvent,
          });
          break;
        case "done":
          dispatch({ type: "FINISH_STREAM" });
          break;
        case "error":
          dispatch({
            type: "CHAT_CONNECT_ERROR",
            payload: { error: getErrorMessage(event.data) },
          });
          break;
        default:
          break;
      }
    },
    [dispatch, handleAgentMessageEvent],
  );

  const streamAgent = useCallback(
    async (
      path: string,
      body: AgentChatRequest | AgentResumeRequest,
      signal?: AbortSignal,
    ) => {
      await fetchEventSource(`${baseUrl}${path}`, {
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
            handleAgentStreamEvent(event);
          }
        },
        onerror(error) {
          throw error;
        },
      });
    },
    [handleAgentStreamEvent],
  );

  /** Sends a message to the agent and initiates a conversation.
   *
   * Dispatches actions to add the user message, start the chat connection,
   * and processes the streaming response from the agent.
   *
   * @param query - The text content of the user's message.
   * @param toolNames - Optional list of tool names to invoke for this message.
   */
  const sendMessage = useCallback(
    async (query: string, toolNames: string[] = []) => {
      dispatch({ type: "ADD_USER_MESSAGE", payload: { content: query } });
      dispatch({ type: "CHAT_START" });

      const request: AgentChatRequest = {
        query,
        thread_id: state.threadId || "",
        tool_names: toolNames,
      };

      try {
        await streamAgent("/api/v1/agent/chat", request);
      } catch (error) {
        dispatch({
          type: "CHAT_CONNECT_ERROR",
          payload: { error: (error as Error).message },
        });
      }
    },
    [state.threadId, streamAgent],
  );

  /** Resumes a conversation that was interrupted.
   *
   * Resumes an existing conversation thread with specified targets,
   * typically used after an interrupt requires user confirmation or action.
   *
   * @param targets - A record containing target data for resuming the conversation.
   */
  const resumeConversation = useCallback(
    async (targets: Record<string, any>) => {
      if (!state.threadId) {
        console.error("Cannot resume without a threadId.");
        return;
      }
      dispatch({ type: "RESUME_START" });

      const request: AgentResumeRequest = {
        thread_id: state.threadId,
        targets,
      };

      try {
        await streamAgent("/api/v1/agent/chat/resume", request);
      } catch (error) {
        dispatch({
          type: "CHAT_CONNECT_ERROR",
          payload: { error: (error as Error).message },
        });
      }
    },
    [state.threadId, streamAgent],
  );

  const value: LumilioChatContextValue = {
    state,
    dispatch,
    sendMessage,
    resumeConversation,
  };

  return (
    <LumilioChatContext.Provider value={value}>
      {children}
    </LumilioChatContext.Provider>
  );
};
