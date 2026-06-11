import { create } from "zustand";
import { streamAgent } from "../api/agentStream";
import type { ChatMessage } from "../types";
import {
  applyChunk,
  applyInterrupt,
  applySideEvent,
  assistantMessage,
  finishStream,
  resolveConfirm,
  userMessage,
} from "./blocks";

/** Feature-local interactive chat state (Zustand per project convention);
 * server state (tools list, ref hydration) lives in TanStack Query. */
interface LumilioChatStore {
  threadId: string | null;
  messages: ChatMessage[];
  isGenerating: boolean;
  /** Set while an interrupt awaits the user's confirm/cancel. */
  awaitingConfirmation: boolean;
  connectionError: string | null;

  sendMessage: (query: string) => Promise<void>;
  confirmInterrupt: (interruptId: string, approved: boolean) => Promise<void>;
  newConversation: () => void;
}

export const useLumilioChatStore = create<LumilioChatStore>((set, get) => {
  const callbacks = {
    onSessionInfo: (threadId: string) => set({ threadId }),
    onChunk: (chunk: { output?: string; reasoning?: string }) =>
      set((state) => ({ messages: applyChunk(state.messages, chunk) })),
    onSideEvent: (event: Parameters<typeof applySideEvent>[1]) =>
      set((state) => ({ messages: applySideEvent(state.messages, event) })),
    onInterrupt: (interrupt: Parameters<typeof applyInterrupt>[1]) =>
      set((state) => ({
        messages: applyInterrupt(state.messages, interrupt),
        awaitingConfirmation: true,
        isGenerating: false,
      })),
    onDone: () =>
      set((state) => ({
        messages: finishStream(state.messages),
        isGenerating: false,
      })),
    onError: (message: string) =>
      set((state) => ({
        messages: finishStream(state.messages),
        connectionError: message,
        isGenerating: false,
      })),
  };

  return {
    threadId: null,
    messages: [],
    isGenerating: false,
    awaitingConfirmation: false,
    connectionError: null,

    sendMessage: async (query: string) => {
      const trimmed = query.trim();
      if (!trimmed || get().isGenerating) return;

      set((state) => ({
        messages: [...state.messages, userMessage(trimmed), assistantMessage()],
        isGenerating: true,
        connectionError: null,
      }));

      try {
        await streamAgent(
          "/api/v1/agent/chat",
          { query: trimmed, thread_id: get().threadId ?? "" },
          callbacks,
        );
      } catch (error) {
        callbacks.onError((error as Error).message);
      }
    },

    confirmInterrupt: async (interruptId: string, approved: boolean) => {
      const threadId = get().threadId;
      if (!threadId) return;

      set((state) => ({
        messages: [
          ...resolveConfirm(
            state.messages,
            approved ? "approved" : "rejected",
          ),
          assistantMessage(),
        ],
        awaitingConfirmation: false,
        isGenerating: true,
        connectionError: null,
      }));

      try {
        await streamAgent(
          "/api/v1/agent/chat/resume",
          { thread_id: threadId, targets: { [interruptId]: { approved } } },
          callbacks,
        );
      } catch (error) {
        callbacks.onError((error as Error).message);
      }
    },

    newConversation: () =>
      set({
        threadId: null,
        messages: [],
        isGenerating: false,
        awaitingConfirmation: false,
        connectionError: null,
      }),
  };
});
