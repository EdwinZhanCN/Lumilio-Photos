/**
 * Agent Feature Root Reducer
 * Combines all sub-reducers
 */

import type { AgentState, AgentAction } from "./types";
import { chatReducer } from "./reducers/chat.reducer";
import { toolsReducer } from "./reducers/tools.reducer";
import { uiReducer } from "./reducers/ui.reducer";

export const initialState: AgentState = {
  chat: {
    messages: [],
    isStreaming: false,
    currentMessageId: null,
    error: null,
  },
  tools: {
    tools: [],
    loading: false,
    error: null,
    lastFetch: null,
  },
  ui: {
    isInputDisabled: false,
    selectedTools: [],
    showWelcome: true,
  },
};

export const agentReducer = (
  state: AgentState = initialState,
  action: AgentAction,
): AgentState => {
  return {
    chat: chatReducer(state.chat, action as any), // Type assertion to handle action union
    tools: toolsReducer(state.tools, action as any),
    ui: uiReducer(state.ui, action as any),
  };
};
