/**
 * Chat Reducer
 * Manages chat messages and streaming state
 */

import type { ChatState, ChatAction } from "../types";

export const initialChatState: ChatState = {
  messages: [],
  isStreaming: false,
  currentMessageId: null,
  error: null,
};

export const chatReducer = (
  state: ChatState = initialChatState,
  action: ChatAction,
): ChatState => {
  switch (action.type) {
    case "ADD_MESSAGE": {
      return {
        ...state,
        messages: [...state.messages, action.payload],
        error: null,
      };
    }

    case "UPDATE_MESSAGE": {
      const { messageId, updates } = action.payload;
      return {
        ...state,
        messages: state.messages.map((msg) =>
          msg.id === messageId ? { ...msg, ...updates } : msg,
        ),
      };
    }

    case "SET_STREAMING": {
      return {
        ...state,
        isStreaming: action.payload,
      };
    }

    case "SET_CURRENT_MESSAGE": {
      return {
        ...state,
        currentMessageId: action.payload,
      };
    }

    case "SET_ERROR": {
      return {
        ...state,
        error: action.payload,
        isStreaming: false,
      };
    }

    case "CLEAR_MESSAGES": {
      return {
        ...initialChatState,
      };
    }

    case "REMOVE_MESSAGE": {
      return {
        ...state,
        messages: state.messages.filter((msg) => msg.id !== action.payload),
      };
    }

    default:
      return state;
  }
};
