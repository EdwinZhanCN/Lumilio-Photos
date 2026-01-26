import React, {
  createContext,
  useReducer,
  ReactNode,
  useEffect,
  useCallback,
} from "react";
import { LumilioChatContextValue } from "./lumilio.type.ts";
import { lumilioReducer, initialState } from "./lumilio.reducer";
import { agentService } from "@/services/agentService";
import type {
  AgentChatRequest,
  AgentResumeRequest,
} from "@/services/agentService";

export const LumilioChatContext = createContext<
  LumilioChatContextValue | undefined
>(undefined);

export const LumilioChatProvider = ({ children }: { children: ReactNode }) => {
  const [state, dispatch] = useReducer(lumilioReducer, initialState);

  useEffect(() => {
    const fetchTools = async () => {
      try {
        dispatch({ type: "FETCH_TOOLS_START" });
        const commands = await agentService.getSlashCommands();
        dispatch({ type: "FETCH_TOOLS_SUCCESS", payload: commands });
      } catch (error) {
        console.error("Failed to fetch agent tools:", error);
      }
    };
    fetchTools();
  }, []);

  /** Processes the SSE stream and dispatches appropriate actions.
   *
   * Iterates through the stream of events from the agent and dispatches corresponding
   * actions to update the chat state. Handles session info, messages, UI events,
   * actions, completion, and errors.
   *
   * @param stream - Async generator yielding SSE events from the agent.
   * @param dispatch - React dispatch function to update the chat state.
   */
  const processStream = useCallback(
    async (stream: AsyncGenerator<any>, dispatch: React.Dispatch<any>) => {
      for await (const event of stream) {
        if (!event) continue;

        switch (event.type) {
          case "session_info":
            dispatch({
              type: "CHAT_CONNECT_SUCCESS",
              payload: { threadId: event.data.thread_id },
            });
            break;
          case "message": {
            // A message can contain text chunks and/or an action.
            if (event.data.output || event.data.reasoning) {
              dispatch({ type: "PROCESS_STREAM_CHUNK", payload: event.data });
            }
            // Handle potential case mismatch for the interrupt property from the backend.
            const interrupt =
              event.data.action?.interrupted || event.data.action?.Interrupted;
            if (interrupt) {
              dispatch({
                type: "RECEIVE_INTERRUPT",
                payload: interrupt,
              });
            }
            break;
          }
          case "ui_event":
            dispatch({ type: "RECEIVE_UI_EVENT", payload: event.data });
            break;
          case "action": {
            // An action can also contain text chunks.
            if (event.data.output || event.data.reasoning) {
              dispatch({ type: "PROCESS_STREAM_CHUNK", payload: event.data });
            }
            // Handle potential case mismatch for the interrupt property from the backend.
            const actionInterrupt =
              event.data.action?.interrupted || event.data.action?.Interrupted;
            if (actionInterrupt) {
              dispatch({
                type: "RECEIVE_INTERRUPT",
                payload: actionInterrupt,
              });
            }
            break;
          }
          case "done":
            dispatch({ type: "FINISH_STREAM" });
            break;
          case "error":
            dispatch({
              type: "CHAT_CONNECT_ERROR",
              payload: { error: event.data.error },
            });
            break;
        }
      }
    },
    [],
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
        const stream = agentService.streamAgentChat(request);
        await processStream(stream, dispatch);
      } catch (error) {
        dispatch({
          type: "CHAT_CONNECT_ERROR",
          payload: { error: (error as Error).message },
        });
      }
    },
    [state.threadId, processStream],
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
        const stream = agentService.streamAgentResume(request);
        await processStream(stream, dispatch);
      } catch (error) {
        dispatch({
          type: "CHAT_CONNECT_ERROR",
          payload: { error: (error as Error).message },
        });
      }
    },
    [state.threadId, processStream],
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
