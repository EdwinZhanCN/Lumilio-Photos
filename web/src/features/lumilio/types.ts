/**
 * Lumen/AI Agent Feature Types
 */

import type { ToolInfoResponse } from "@/services/agentService";

// ============================================================================
// Message Types
// ============================================================================

/**
 * Message role
 */
export type MessageRole = "user" | "assistant" | "system";

/**
 * Chat message
 */
export interface ChatMessage {
  id: string;
  role: MessageRole;
  content: string;
  timestamp: number;
  status?: "pending" | "streaming" | "completed" | "error";
  error?: string;
  metadata?: {
    agentName?: string;
    runPath?: string[];
    toolCalls?: Array<{
      name: string;
      input: unknown;
    }>;
  };
}

// ============================================================================
// Chat State
// ============================================================================

/**
 * Chat session state
 */
export interface ChatState {
  messages: ChatMessage[];
  isStreaming: boolean;
  currentMessageId: string | null;
  error: string | null;
}

// ============================================================================
// Tools State
// ============================================================================

/**
 * Available tools state
 */
export interface ToolsState {
  tools: ToolInfoResponse[];
  loading: boolean;
  error: string | null;
  lastFetch: number | null;
}

// ============================================================================
// UI State
// ============================================================================

/**
 * UI state for chat interface
 */
export interface AgentUIState {
  isInputDisabled: boolean;
  selectedTools: string[];
  showWelcome: boolean;
}

// ============================================================================
// Root State
// ============================================================================

/**
 * Root state for lumen/agent feature
 */
export interface AgentState {
  chat: ChatState;
  tools: ToolsState;
  ui: AgentUIState;
}

// ============================================================================
// Actions
// ============================================================================

/**
 * Chat actions
 */
export type ChatAction =
  | { type: "ADD_MESSAGE"; payload: ChatMessage }
  | { type: "UPDATE_MESSAGE"; payload: { messageId: string; updates: Partial<ChatMessage> } }
  | { type: "SET_STREAMING"; payload: boolean }
  | { type: "SET_CURRENT_MESSAGE"; payload: string | null }
  | { type: "SET_ERROR"; payload: string | null }
  | { type: "CLEAR_MESSAGES" }
  | { type: "REMOVE_MESSAGE"; payload: string };

/**
 * Tools actions
 */
export type ToolsAction =
  | { type: "SET_TOOLS"; payload: ToolInfoResponse[] }
  | { type: "SET_TOOLS_LOADING"; payload: boolean }
  | { type: "SET_TOOLS_ERROR"; payload: string | null }
  | { type: "TOGGLE_TOOL"; payload: string }
  | { type: "CLEAR_SELECTED_TOOLS" };

/**
 * UI actions
 */
export type UIAction =
  | { type: "SET_INPUT_DISABLED"; payload: boolean }
  | { type: "SET_SHOW_WELCOME"; payload: boolean };

/**
 * All agent actions
 */
export type AgentAction = ChatAction | ToolsAction | UIAction;

// ============================================================================
// Context
// ============================================================================

/**
 * Agent context value
 */
export interface AgentContextValue {
  state: AgentState;
  dispatch: React.Dispatch<AgentAction>;
}
