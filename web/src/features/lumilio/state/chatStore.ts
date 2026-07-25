import { create } from "zustand";
import { cancelAgentRun, streamAgent, type AgentStreamCallbacks } from "../api/agentStream";
import type { AgentMode, AgentRunStatus, ChatMessage, TokenUsageInfo } from "../model/chatTypes";
import type { MentionPayload } from "../modules/mentions/mentionSources";
import type { ContextContribution } from "@/lib/assistant";
import {
  applyChunk,
  applyInterrupt,
  applySideEvent,
  assistantMessage,
  cancelActiveBlocks,
  resolveConfirm,
  userMessage,
} from "./blocks";

/** Feature-local interactive chat state (Zustand per project convention);
 * server state (tools list, ref hydration) lives in TanStack Query. */
interface LumilioChatStore {
  threadId: string | null;
  activeRunId: string | null;
  messages: ChatMessage[];
  isGenerating: boolean;
  isStopping: boolean;
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
      mode?: AgentMode;
    },
  ) => Promise<void>;
  confirmInterrupt: (interruptId: string, approved: boolean) => Promise<void>;
  stopGeneration: () => Promise<void>;
  newConversation: () => Promise<void>;
  resetSession: () => void;
}

let activeStreamController: AbortController | null = null;
let clearAfterStop = false;

export const useLumilioChatStore = create<LumilioChatStore>((set, get) => {
  const clearConversation = () => {
    clearAfterStop = false;
    set({
      threadId: null,
      activeRunId: null,
      messages: [],
      isGenerating: false,
      isStopping: false,
      awaitingConfirmation: false,
      connectionError: null,
      usage: null,
    });
  };

  const callbacksFor = (controller: AbortController): AgentStreamCallbacks => {
    const isCurrent = () => activeStreamController === controller;
    return {
      onSessionInfo: (threadId, runId) => {
        if (!isCurrent()) return;
        set({ threadId, activeRunId: runId });
        // Stop can be pressed before session_info arrives. Keep the stream
        // open until the exact run id exists, then cancel server-side first.
        if (get().isStopping) void get().stopGeneration();
      },
      onRunStatus: (runId, status: AgentRunStatus) => {
        if (!isCurrent()) return;
        const activeRunId = get().activeRunId;
        if (activeRunId && activeRunId !== runId) return;
        if (status === "running") {
          set({ activeRunId: runId, isGenerating: true });
          return;
        }
        if (status === "cancel_requested") {
          set({ isStopping: true });
          return;
        }
        if (status === "awaiting_confirmation") {
          set({
            activeRunId: runId,
            awaitingConfirmation: true,
            isGenerating: false,
            isStopping: false,
          });
          return;
        }
        if (status === "cancelled") {
          set((state) => ({
            messages: cancelActiveBlocks(state.messages),
            activeRunId: null,
            awaitingConfirmation: false,
            isGenerating: false,
            isStopping: false,
          }));
          return;
        }
        set({
          activeRunId: null,
          awaitingConfirmation: false,
          isGenerating: false,
          isStopping: false,
        });
      },
      onChunk: (chunk) => {
        if (!isCurrent()) return;
        set((state) => ({ messages: applyChunk(state.messages, chunk) }));
      },
      onSideEvent: (event) => {
        if (!isCurrent()) return;
        if (event.type === "token_usage") {
          if (event.usage) set({ usage: event.usage });
          return;
        }
        set((state) => ({ messages: applySideEvent(state.messages, event) }));
      },
      onInterrupt: (interrupt) => {
        if (!isCurrent()) return;
        set((state) => ({
          messages: applyInterrupt(state.messages, interrupt),
          awaitingConfirmation: true,
          isGenerating: false,
          isStopping: false,
        }));
      },
      onDone: () => {
        if (!isCurrent()) return;
        set((state) => ({
          activeRunId: state.awaitingConfirmation ? state.activeRunId : null,
          isGenerating: false,
          isStopping: false,
        }));
      },
      onError: (message) => {
        if (!isCurrent()) return;
        clearAfterStop = false;
        set({
          activeRunId: null,
          connectionError: message,
          awaitingConfirmation: false,
          isGenerating: false,
          isStopping: false,
        });
      },
    };
  };

  return {
    threadId: null,
    activeRunId: null,
    messages: [],
    isGenerating: false,
    isStopping: false,
    awaitingConfirmation: false,
    connectionError: null,
    usage: null,

    sendMessage: async (query, options) => {
      const trimmed = query.trim();
      if (!trimmed || get().isGenerating || get().awaitingConfirmation) return;

      set((state) => ({
        messages: [...state.messages, userMessage(trimmed), assistantMessage()],
        activeRunId: null,
        isGenerating: true,
        isStopping: false,
        connectionError: null,
      }));

      const contextPayload = options?.context?.map((item) => ({
        type: item.type,
        asset_ids: item.assetIds,
        label: item.label,
      }));
      const mentionsPayload = options?.mentions?.map((mention) => ({
        type: mention.type,
        id: mention.id,
        label: mention.label,
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
            mode: options?.mode ?? "free",
            ...(contextPayload?.length ? { context: contextPayload } : {}),
            ...(mentionsPayload?.length ? { mentions: mentionsPayload } : {}),
          },
          callbacksFor(controller),
          controller.signal,
        );
      } catch (error) {
        if (!controller.signal.aborted) {
          callbacksFor(controller).onError((error as Error).message);
        }
      } finally {
        if (activeStreamController === controller) activeStreamController = null;
      }
    },

    confirmInterrupt: async (interruptId, approved) => {
      const threadId = get().threadId;
      if (!threadId || get().isGenerating) return;

      set((state) => ({
        messages: [
          ...resolveConfirm(state.messages, approved ? "approved" : "rejected"),
          assistantMessage(),
        ],
        // The previous awaiting run is terminal as soon as Resume begins. A
        // Stop pressed before new session_info must wait for the new run id.
        activeRunId: null,
        awaitingConfirmation: false,
        isGenerating: true,
        isStopping: false,
        connectionError: null,
      }));

      const controller = new AbortController();
      activeStreamController?.abort();
      activeStreamController = controller;
      try {
        await streamAgent(
          "/api/v1/agent/chat/resume",
          { thread_id: threadId, targets: { [interruptId]: { approved } } },
          callbacksFor(controller),
          controller.signal,
        );
      } catch (error) {
        if (!controller.signal.aborted) {
          callbacksFor(controller).onError((error as Error).message);
        }
      } finally {
        if (activeStreamController === controller) activeStreamController = null;
      }
    },

    stopGeneration: async () => {
      const { threadId, activeRunId, isGenerating, awaitingConfirmation } = get();
      if (!threadId || !activeRunId) {
        if (isGenerating || awaitingConfirmation) set({ isStopping: true });
        return;
      }

      set({ isStopping: true, connectionError: null });
      try {
        const result = await cancelAgentRun({ thread_id: threadId, run_id: activeRunId });
        const stopped = result.status === "cancel_requested" || result.status === "cancelled";
        const controller = activeStreamController;
        // The server has accepted (or already resolved) the exact run before
        // the local transport is closed.
        controller?.abort();
        if (activeStreamController === controller) activeStreamController = null;
        set((state) => ({
          messages: stopped ? cancelActiveBlocks(state.messages) : state.messages,
          activeRunId: null,
          awaitingConfirmation: false,
          isGenerating: false,
          isStopping: false,
        }));
        if (clearAfterStop) clearConversation();
      } catch (error) {
        clearAfterStop = false;
        set({ isStopping: false, connectionError: (error as Error).message });
      }
    },

    newConversation: async () => {
      const state = get();
      if (state.isGenerating || state.awaitingConfirmation || state.activeRunId) {
        clearAfterStop = true;
        await state.stopGeneration();
        return;
      }
      clearConversation();
    },

    resetSession: () => {
      clearAfterStop = false;
      const controller = activeStreamController;
      activeStreamController = null;
      const { threadId, activeRunId } = get();
      if (threadId && activeRunId) {
        void cancelAgentRun({ thread_id: threadId, run_id: activeRunId }).finally(() =>
          controller?.abort(),
        );
      } else {
        controller?.abort();
      }
      clearConversation();
    },
  };
});
