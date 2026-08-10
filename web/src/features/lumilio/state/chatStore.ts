import { create } from "zustand";
import {
  cancelAgentRun,
  getAgentEffectStatus,
  streamAgent,
  type AgentStreamCallbacks,
} from "../api/agentStream";
import type {
  AgentMode,
  AgentRunStatus,
  AgentTurnSnapshot,
  ChatMessage,
  TokenUsageInfo,
} from "../model/chatTypes";
import type { MentionPayload } from "../modules/mentions/mentionSources";
import type { ContextContribution } from "@/lib/assistant";
import {
  applyChunk,
  applyDroppedMentions,
  applyEffectReceipt,
  applyInterrupt,
  applySideEvent,
  assistantMessage,
  cancelActiveBlocks,
  confirmationEffectID,
  failConfirm,
  removeTrailingEmptyAssistant,
  setConfirmSubmitting,
  userMessage,
} from "./blocks";

interface PendingConfirmation {
  interruptId: string;
  effectId?: string;
  approved: boolean;
}

/** Feature-local interactive chat state (Zustand per project convention);
 * server state (tools list, ref hydration) lives in TanStack Query. */
interface LumilioChatStore {
  threadId: string | null;
  activeRunId: string | null;
  messages: ChatMessage[];
  isGenerating: boolean;
  isStopping: boolean;
  awaitingConfirmation: boolean;
  pendingConfirmation: PendingConfirmation | null;
  connectionError: string | null;
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

const confirmationFor = (messages: ChatMessage[], interruptId: string) => {
  for (const message of messages) {
    for (const block of message.blocks) {
      if (
        block.kind === "confirm" &&
        block.interrupt.InterruptContexts.some((context) => context.ID === interruptId)
      ) {
        return block;
      }
    }
  }
  return undefined;
};

const requestSnapshot = (
  mode: AgentMode,
  context: ContextContribution[],
  mentions: MentionPayload[],
): AgentTurnSnapshot => ({
  mode,
  context: context.map((item) => ({
    id: item.id,
    type: item.type,
    label: item.label,
    count: item.assetIds.length,
  })),
  mentions: mentions.map((mention) => ({ ...mention, status: "accepted" })),
});

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
      pendingConfirmation: null,
      connectionError: null,
      usage: null,
    });
  };

  const reconcilePendingConfirmation = async (fallbackMessage: string): Promise<boolean> => {
    const pending = get().pendingConfirmation;
    const threadId = get().threadId;
    if (!pending || !threadId || !pending.effectId) return false;

    let lastError = fallbackMessage;
    for (const delayMs of [0, 150, 400]) {
      if (delayMs > 0) await new Promise((resolve) => setTimeout(resolve, delayMs));
      try {
        const status = await getAgentEffectStatus(threadId, pending.effectId);
        if (status.receipt) {
          set((state) => ({
            messages: applyEffectReceipt(state.messages, status.receipt!),
            pendingConfirmation: null,
            awaitingConfirmation: false,
            isGenerating: false,
            isStopping: false,
            connectionError: null,
          }));
          return true;
        }
        if (["cancelled", "failed"].includes(status.status)) {
          lastError = `The server reports this action as ${status.status}. You can retry safely.`;
          break;
        }
      } catch (error) {
        lastError = (error as Error).message || fallbackMessage;
      }
    }

    const current = get().pendingConfirmation;
    if (current?.interruptId === pending.interruptId) {
      set((state) => ({
        messages: removeTrailingEmptyAssistant(
          failConfirm(state.messages, pending.interruptId, lastError),
        ),
        pendingConfirmation: null,
        awaitingConfirmation: true,
        isGenerating: false,
        isStopping: false,
      }));
    }
    return false;
  };

  const callbacksFor = (controller: AbortController): AgentStreamCallbacks => {
    const isCurrent = () => activeStreamController === controller;
    return {
      onSessionInfo: (threadId, runId, droppedMentions) => {
        if (!isCurrent()) return;
        set((state) => ({
          threadId,
          activeRunId: runId,
          messages: applyDroppedMentions(state.messages, droppedMentions),
        }));
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
            pendingConfirmation: null,
            isGenerating: false,
            isStopping: false,
          }));
          return;
        }
        // A resume is not successful merely because the run reached a terminal
        // transport state. The effect receipt is the confirmation authority.
        set({
          activeRunId: null,
          isGenerating: false,
          isStopping: false,
          awaitingConfirmation: Boolean(get().pendingConfirmation),
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
        if (event.type === "effect_receipt" && event.receipt) {
          const receipt = event.receipt;
          set((state) => {
            const matchesPending = state.pendingConfirmation?.effectId === receipt.effect_id;
            return {
              messages: applyEffectReceipt(state.messages, receipt),
              pendingConfirmation: matchesPending ? null : state.pendingConfirmation,
              awaitingConfirmation: matchesPending ? false : state.awaitingConfirmation,
            };
          });
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
          isGenerating: Boolean(state.pendingConfirmation),
          isStopping: false,
        }));
      },
      onError: (message) => {
        if (!isCurrent()) return;
        clearAfterStop = false;
        if (get().pendingConfirmation) {
          set({ activeRunId: null, connectionError: message, isGenerating: true, isStopping: false });
          return;
        }
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
    pendingConfirmation: null,
    connectionError: null,
    usage: null,

    sendMessage: async (query, options) => {
      const trimmed = query.trim();
      if (!trimmed || get().isGenerating || get().awaitingConfirmation) return;

      const context = options?.context ?? [];
      const mentions = options?.mentions ?? [];
      const mode = options?.mode ?? "free";
      set((state) => ({
        messages: [
          ...state.messages,
          userMessage(trimmed, requestSnapshot(mode, context, mentions)),
          assistantMessage(),
        ],
        activeRunId: null,
        isGenerating: true,
        isStopping: false,
        connectionError: null,
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
            mode,
            ...(context.length
              ? {
                  context: context.map((item) => ({
                    type: item.type,
                    asset_ids: item.assetIds,
                    label: item.label,
                  })),
                }
              : {}),
            ...(mentions.length
              ? {
                  mentions: mentions.map((mention) => ({
                    type: mention.type,
                    id: mention.id,
                    label: mention.label,
                  })),
                }
              : {}),
          },
          callbacksFor(controller),
          controller.signal,
        );
      } catch (error) {
        if (!controller.signal.aborted) callbacksFor(controller).onError((error as Error).message);
      } finally {
        if (activeStreamController === controller) activeStreamController = null;
      }
    },

    confirmInterrupt: async (interruptId, approved) => {
      const state = get();
      if (!state.threadId || state.isGenerating || state.pendingConfirmation) return;
      const confirm = confirmationFor(state.messages, interruptId);
      if (!confirm || !["pending", "failed"].includes(confirm.state)) return;

      const pending: PendingConfirmation = {
        interruptId,
        effectId: confirmationEffectID(confirm.interrupt),
        approved,
      };
      set((current) => ({
        messages: [
          ...setConfirmSubmitting(current.messages, interruptId, approved),
          assistantMessage(),
        ],
        activeRunId: null,
        awaitingConfirmation: false,
        pendingConfirmation: pending,
        isGenerating: true,
        isStopping: false,
        connectionError: null,
      }));

      const controller = new AbortController();
      activeStreamController?.abort();
      activeStreamController = controller;
      let reconciliationMessage =
        "The confirmation stream ended without a receipt. Lumilio Agent will verify the effect before allowing a retry.";
      try {
        await streamAgent(
          "/api/v1/agent/chat/resume",
          { thread_id: state.threadId, targets: { [interruptId]: { approved } } },
          callbacksFor(controller),
          controller.signal,
        );
      } catch (error) {
        reconciliationMessage = (error as Error).message || reconciliationMessage;
        if (!controller.signal.aborted) callbacksFor(controller).onError(reconciliationMessage);
      } finally {
        if (activeStreamController === controller) activeStreamController = null;
      }
      if (!controller.signal.aborted && get().pendingConfirmation?.interruptId === interruptId) {
        await reconcilePendingConfirmation(reconciliationMessage);
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
        controller?.abort();
        if (activeStreamController === controller) activeStreamController = null;
        set((state) => ({
          messages: stopped ? cancelActiveBlocks(state.messages) : state.messages,
          activeRunId: null,
          awaitingConfirmation: false,
          pendingConfirmation: null,
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
