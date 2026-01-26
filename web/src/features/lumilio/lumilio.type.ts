import type {
  SideChannelEvent,
  InterruptInfo,
} from "@/features/lumilio/schema";
import type { MentionEntity } from "./components/RichInput";

/**
 * A single message in the conversation history.
 */
export type ChatMessage = {
  id: string;
  role: "user" | "assistant";
  content: string;
  // UI events are attached to the assistant message they belong to.
  uiEvents: SideChannelEvent[];
};

/**
 * The status of the SSE connection.
 */
export type ConnectionStatus =
  | "disconnected"
  | "connecting"
  | "connected"
  | "error";

/**
 * Main state for the Lumilio chat feature.
 */
export interface LumilioChatState {
  connection: {
    status: ConnectionStatus;
    error?: string;
  };
  threadId: string | null;
  conversation: ChatMessage[];
  isGenerating: boolean; // True if waiting for any part of an agent response.
  streamingBlock: "reasoning" | "output" | null; // Track current streaming block type

  tools: {
    available: MentionEntity[]; // For the '/' command menu
    isLoading: boolean;
  };

  interrupt: InterruptInfo | null;
}

/**
 * All possible actions that can be dispatched to update the state.
 */
export type LumilioChatAction =
  // Connection and session management
  | { type: "CHAT_START" }
  | { type: "RESUME_START" }
  | { type: "CHAT_CONNECT_SUCCESS"; payload: { threadId: string } }
  | { type: "CHAT_CONNECT_ERROR"; payload: { error: string } }
  | { type: "CHAT_DISCONNECT" }

  // Message stream handling
  | { type: "ADD_USER_MESSAGE"; payload: { content: string } }
  | {
      type: "PROCESS_STREAM_CHUNK";
      payload: { reasoning?: string; output?: string };
    }
  | { type: "FINISH_STREAM" }

  // Side-channel and interrupt handling
  | { type: "RECEIVE_UI_EVENT"; payload: SideChannelEvent }
  | { type: "RECEIVE_INTERRUPT"; payload: InterruptInfo }
  | { type: "CLEAR_INTERRUPT" }

  // Tool and command management
  | { type: "FETCH_TOOLS_START" }
  | { type: "FETCH_TOOLS_SUCCESS"; payload: MentionEntity[] };

/**
 * The value provided by the LumilioChatContext.
 */
export interface LumilioChatContextValue {
  state: LumilioChatState;
  dispatch: React.Dispatch<LumilioChatAction>;
  sendMessage: (query: string, toolNames?: string[]) => void;
  resumeConversation: (targets: Record<string, any>) => void;
}
