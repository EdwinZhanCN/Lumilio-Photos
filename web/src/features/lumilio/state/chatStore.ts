import { create } from "zustand";
import { streamAgent } from "../api/agentStream";
import type { ChatMessage, TokenUsageInfo } from "../model/chatTypes";
import type { MentionPayload } from "../modules/mentions/mentionSources";
import type { ContextContribution } from "@/lib/assistant";
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
  /** Last model call's token accounting; promptTokens ≈ current context size. */
  usage: TokenUsageInfo | null;

  sendMessage: (
    query: string,
    options?: {
      context?: ContextContribution[];
      mentions?: MentionPayload[];
      mode?: string;
    },
  ) => Promise<void>;
  confirmInterrupt: (interruptId: string, approved: boolean) => Promise<void>;
  newConversation: () => void;
  resetSession: () => void;
}

let activeStreamController: AbortController | null = null;

export const useLumilioChatStore = create<LumilioChatStore>((set, get) => {
  const callbacks = {
    onSessionInfo: (threadId: string) => set({ threadId }),
    onChunk: (chunk: { output?: string; reasoning?: string }) =>
      set((state) => ({ messages: applyChunk(state.messages, chunk) })),
    onSideEvent: (event: Parameters<typeof applySideEvent>[1]) => {
      if (event.type === "token_usage") {
        if (event.usage) set({ usage: event.usage });
        return;
      }
      set((state) => ({ messages: applySideEvent(state.messages, event) }));
    },
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
    usage: null,

    sendMessage: async (query, options) => {
      const trimmed = query.trim();
      if (!trimmed || get().isGenerating) return;

      set((state) => ({
        messages: [...state.messages, userMessage(trimmed), assistantMessage()],
        isGenerating: true,
        connectionError: null,
      }));

      const contextPayload = options?.context?.map((item) => ({
        type: item.type,
        asset_ids: item.assetIds,
        label: item.label,
      }));

      const mentionsPayload = options?.mentions?.map((m) => ({
        type: m.type,
        id: m.id,
        label: m.label,
      }));

      const controller = new AbortController();
      activeStreamController?.abort();
      activeStreamController = controller;
      try {
        await streamAgent(
          "/api/v1/agent/chat",
          {
            query: trimmed,
            thread_id: get().threadId ?? "",
            ...(options?.mode
              ? { mode: options.mode as "review" | "organize" | "analyze" | "curate" }
              : {}),
            ...(contextPayload && contextPayload.length > 0 ? { context: contextPayload } : {}),
            ...(mentionsPayload && mentionsPayload.length > 0 ? { mentions: mentionsPayload } : {}),
          },
          callbacks,
          controller.signal,
        );
      } catch (error) {
        if (!controller.signal.aborted) callbacks.onError((error as Error).message);
      } finally {
        if (activeStreamController === controller) activeStreamController = null;
      }
    },

    confirmInterrupt: async (interruptId: string, approved: boolean) => {
      const threadId = get().threadId;
      if (!threadId) return;

      set((state) => ({
        messages: [
          ...resolveConfirm(state.messages, approved ? "approved" : "rejected"),
          assistantMessage(),
        ],
        awaitingConfirmation: false,
        isGenerating: true,
        connectionError: null,
      }));

      const controller = new AbortController();
      activeStreamController?.abort();
      activeStreamController = controller;
      try {
        await streamAgent(
          "/api/v1/agent/chat/resume",
          { thread_id: threadId, targets: { [interruptId]: { approved } } },
          callbacks,
          controller.signal,
        );
      } catch (error) {
        if (!controller.signal.aborted) callbacks.onError((error as Error).message);
      } finally {
        if (activeStreamController === controller) activeStreamController = null;
      }
    },

    newConversation: () =>
      set({
        threadId: null,
        messages: [],
        isGenerating: false,
        awaitingConfirmation: false,
        connectionError: null,
        usage: null,
      }),
    resetSession: () => {
      activeStreamController?.abort();
      activeStreamController = null;
      set({
        threadId: null,
        messages: [],
        isGenerating: false,
        awaitingConfirmation: false,
        connectionError: null,
        usage: null,
      });
    },
  };
});
