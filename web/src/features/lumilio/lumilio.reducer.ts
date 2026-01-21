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
 *
 * Handles all state transitions for the chat feature, including connection status,
 * message management, streaming content processing, UI events, interrupts,
 * and tool management.
 *
 * @param state - The current Lumilio chat state.
 * @param action - The action to dispatch for state update.
 * @returns The updated Lumilio chat state based on the dispatched action.
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
      
      if (import.meta.env.DEV) {
        console.log("[DEBUG] PROCESS_STREAM_CHUNK", { 
          reasoning: Boolean(reasoning), 
          output: Boolean(output), 
          currentBlock: state.streamingBlock 
        });
      }

      const lastMsgIndex = state.conversation.length - 1;
      const lastMsg = state.conversation[lastMsgIndex];

      if (!lastMsg || lastMsg.role !== "assistant") {
        if (import.meta.env.DEV) {
          console.warn("[DEBUG] PROCESS_STREAM_CHUNK ignored: Last message is not assistant", lastMsg);
        }
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
        // Check newStreamingBlock to handle case where we just switched to reasoning in the same chunk
        // or if we were already in reasoning from previous chunks
        if (newStreamingBlock === "reasoning") {
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
        let newStreamingBlock = state.streamingBlock;

        if (existingEventIndex !== -1) {
          newUiEvents = [...lastAsstMsg.uiEvents];
          newUiEvents[existingEventIndex] = newUiEvent;
        } else {
          newUiEvents = [...lastAsstMsg.uiEvents, newUiEvent];

          // If we are currently in a reasoning block, close it before adding the tool
          if (state.streamingBlock === "reasoning") {
            newContent += "</think>";
            newStreamingBlock = null; // Reset block state so next reasoning chunk re-opens it
            if (import.meta.env.DEV) {
              console.log("[DEBUG] Closed think block for tool insertion");
            }
          }

          const toolTag = `\n\n<lumilio-tool id="${newUiEvent.tool.executionId}"></lumilio-tool>\n\n`;
          newContent += toolTag;
        }

        const newConversation = [...state.conversation];
        newConversation[lastAsstMsgIndex] = {
          ...lastAsstMsg,
          uiEvents: newUiEvents,
          content: newContent,
        };
        return {
          ...state,
          conversation: newConversation,
          streamingBlock: newStreamingBlock,
        };
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
