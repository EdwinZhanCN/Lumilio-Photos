// src/features/lumilio/lumilio.reducer.ts

import {
  LumilioChatState,
  LumilioChatAction,
  ChatMessage,
} from "./lumilio.types";

export const initialState: LumilioChatState = {
  connection: {
    status: "disconnected",
  },
  threadId: null,
  conversation: [],
  isGenerating: false,
  streamingBlock: null,
  tools: {
    available: [],
    isLoading: false,
  },
  interrupt: null,
};

/** Reducer for managing the Lumilio chat state.

  Handles all state transitions for the chat feature, including connection status,
  message management, streaming content processing, UI events, interrupts,
  and tool management.

  Args:
    state: The current Lumilio chat state.
    action: The action to dispatch for state update.

  Returns:
    The updated Lumilio chat state based on the dispatched action.
  */
export const lumilioReducer = (
  state: LumilioChatState,
  action: LumilioChatAction,
): LumilioChatState => {
  switch (action.type) {
    case "CHAT_START":
      return {
        ...state,
        connection: { status: "connecting" },
        isGenerating: true,
        streamingBlock: null,
      };

    case "RESUME_START":
      return {
        ...state,
        interrupt: null,
        isGenerating: true,
      };

    case "CHAT_CONNECT_SUCCESS":
      return {
        ...state,
        connection: { status: "connected" },
        threadId: action.payload.threadId,
        isGenerating: false,
      };
    case "CHAT_CONNECT_ERROR":
      return {
        ...state,
        connection: { status: "error", error: action.payload.error },
        isGenerating: false,
      };
    case "CHAT_DISCONNECT":
      return {
        ...state,
        connection: { status: "disconnected" },
        isGenerating: false,
      };

    case "ADD_USER_MESSAGE": {
      const userMessage: ChatMessage = {
        id: crypto.randomUUID(),
        role: "user",
        content: action.payload.content,
        uiEvents: [],
      };
      const assistantPlaceholder: ChatMessage = {
        id: crypto.randomUUID(),
        role: "assistant",
        content: "",
        uiEvents: [],
      };
      return {
        ...state,
        conversation: [
          ...state.conversation,
          userMessage,
          assistantPlaceholder,
        ],
      };
    }

    case "PROCESS_STREAM_CHUNK": {
      const { reasoning, output } = action.payload;
      const lastMsgIndex = state.conversation.length - 1;
      const lastMsg = state.conversation[lastMsgIndex];

      if (!lastMsg || lastMsg.role !== "assistant") {
        return state;
      }

      let newContent = "";
      let newStreamingBlock = state.streamingBlock;

      if (reasoning) {
        if (state.streamingBlock !== "reasoning") {
          newContent += "<think>" + reasoning;
        } else {
          newContent += reasoning;
        }
        newStreamingBlock = "reasoning";
      }

      if (output) {
        if (state.streamingBlock === "reasoning") {
          newContent += "</think>" + output;
        } else {
          newContent += output;
        }
        newStreamingBlock = "output";
      }

      const newConversation = [...state.conversation];
      newConversation[lastMsgIndex] = {
        ...lastMsg,
        content: lastMsg.content + newContent,
      };

      return {
        ...state,
        conversation: newConversation,
        streamingBlock: newStreamingBlock,
      };
    }

    case "FINISH_STREAM": {
      const lastMsgIndex = state.conversation.length - 1;
      const lastMsg = state.conversation[lastMsgIndex];
      let finalConversation = state.conversation;

      if (
        state.streamingBlock === "reasoning" &&
        lastMsg?.role === "assistant"
      ) {
        const newConversation = [...state.conversation];
        newConversation[lastMsgIndex] = {
          ...lastMsg,
          content: lastMsg.content + "</think>",
        };
        finalConversation = newConversation;
      }
      return {
        ...state,
        isGenerating: false,
        streamingBlock: null,
        conversation: finalConversation,
      };
    }

    case "RECEIVE_UI_EVENT": {
      if (import.meta.env.DEV) {
        console.log(
          "[DEBUG] Reducer RECEIVE_UI_EVENT. Current conversation length:",
          state.conversation.length,
        );
        console.log(
          "[DEBUG] Last assistant message's uiEvents before update:",
          JSON.parse(
            JSON.stringify(
              state.conversation[state.conversation.length - 1]?.uiEvents,
            ),
          ),
        );
        console.log("[DEBUG] New uiEvent payload:", action.payload);
      }

      const lastAsstMsgIndex = state.conversation.length - 1;
      const lastAsstMsg = state.conversation[lastAsstMsgIndex];

      if (lastAsstMsg?.role === "assistant") {
        const newUiEvent = action.payload;
        const existingEventIndex = lastAsstMsg.uiEvents.findIndex(
          (event) => event.tool.executionId === newUiEvent.tool.executionId,
        );

        let newUiEvents;
        let newContent = lastAsstMsg.content;

        if (existingEventIndex !== -1) {
          newUiEvents = [...lastAsstMsg.uiEvents];
          newUiEvents[existingEventIndex] = newUiEvent;
        } else {
          newUiEvents = [...lastAsstMsg.uiEvents, newUiEvent];

          const toolTag = `\n\n<lumilio-tool id="${newUiEvent.tool.executionId}"></lumilio-tool>\n\n`;
          newContent += toolTag;
        }

        const newConversation = [...state.conversation];
        newConversation[lastAsstMsgIndex] = {
          ...lastAsstMsg,
          uiEvents: newUiEvents,
          content: newContent,
        };
        return { ...state, conversation: newConversation };
      }
      return state;
    }

    case "RECEIVE_INTERRUPT":
      return { ...state, interrupt: action.payload, isGenerating: false };
    case "CLEAR_INTERRUPT":
      return { ...state, interrupt: null };

    case "FETCH_TOOLS_START":
      return { ...state, tools: { ...state.tools, isLoading: true } };
    case "FETCH_TOOLS_SUCCESS":
      return {
        ...state,
        tools: { available: action.payload, isLoading: false },
      };
    default:
      return state;
  }
};
