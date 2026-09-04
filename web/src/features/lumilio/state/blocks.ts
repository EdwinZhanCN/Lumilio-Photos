import type {
  AgentTurnSnapshot,
  Block,
  ChatMessage,
  DroppedMention,
  EffectReceipt,
  InterruptInfo,
  SideChannelEvent,
} from "../model/chatTypes";
import { createUUID } from "@/lib/uuid";

/** Pure reduction rules from SSE events onto the typed-block conversation.
 * Rendering never parses strings for structure (no pseudo-tags). */

const newId = () => createUUID();

export const userMessage = (content: string, request?: AgentTurnSnapshot): ChatMessage => ({
  id: newId(),
  role: "user",
  blocks: [{ kind: "text", id: newId(), markdown: content }],
  request: request
    ? {
        mode: request.mode,
        context: request.context.map((item) => ({ ...item })),
        mentions: request.mentions.map((item) => ({ ...item })),
      }
    : undefined,
});

export const assistantMessage = (): ChatMessage => ({
  id: newId(),
  role: "assistant",
  blocks: [],
});

const isAssistantLast = (messages: ChatMessage[]): boolean =>
  messages.length > 0 && messages[messages.length - 1].role === "assistant";

const replaceLastMessage = (messages: ChatMessage[], blocks: Block[]): ChatMessage[] => {
  const next = messages.slice();
  next[next.length - 1] = { ...next[next.length - 1], blocks };
  return next;
};

/** Streamed assistant output appends to a matching trailing text block. */
export const applyChunk = (messages: ChatMessage[], chunk: { output?: string }): ChatMessage[] => {
  if (!isAssistantLast(messages)) return messages;
  let blocks = messages[messages.length - 1].blocks;

  if (chunk.output) {
    const last = blocks[blocks.length - 1];
    if (last?.kind === "text") {
      blocks = [...blocks.slice(0, -1), { ...last, markdown: last.markdown + chunk.output }];
    } else {
      blocks = [...blocks, { kind: "text", id: newId(), markdown: chunk.output }];
    }
  }

  return replaceLastMessage(messages, blocks);
};

const confirmationRoot = (interrupt: InterruptInfo) =>
  interrupt.InterruptContexts.find((context) => context.IsRootCause);

export const confirmationEffectID = (interrupt: InterruptInfo): string | undefined => {
  const info = confirmationRoot(interrupt)?.Info;
  return info?.effect_id ?? info?.EffectID;
};

/** tool_execution events upsert a tool block by executionId; widget_show
 * appends a widget block that hydrates from the ref API. Effect receipts are
 * handled separately because they update a previously rendered confirmation. */
export const applySideEvent = (messages: ChatMessage[], event: SideChannelEvent): ChatMessage[] => {
  if (!isAssistantLast(messages) || event.type === "effect_receipt") return messages;
  if (!event.tool?.executionId) return messages;
  let blocks = messages[messages.length - 1].blocks;

  if (event.type === "widget_show") {
    if (event.data?.refId) {
      blocks = [
        ...blocks,
        {
          kind: "widget",
          id: newId(),
          refId: event.data.refId,
          count: event.data.count ?? 0,
          widget: event.data.widget ?? "asset_grid",
          title: event.data.params?.title,
          params: event.data.params,
        },
      ];
    }
    return replaceLastMessage(messages, blocks);
  }

  const index = blocks.findIndex(
    (block) => block.kind === "tool" && block.executionId === event.tool.executionId,
  );
  const existingId = index >= 0 ? blocks[index].id : newId();
  const toolBlock: Block = {
    kind: "tool",
    id: existingId,
    executionId: event.tool.executionId,
    name: event.tool.name,
    status: event.execution?.status ?? "running",
    message: event.execution?.message,
    error: event.execution?.error,
    refId: event.data?.refId,
    count: event.data?.count,
  };
  blocks =
    index >= 0
      ? [...blocks.slice(0, index), toolBlock, ...blocks.slice(index + 1)]
      : [...blocks, toolBlock];

  return replaceLastMessage(messages, blocks);
};

export const applyInterrupt = (
  messages: ChatMessage[],
  interrupt: InterruptInfo,
): ChatMessage[] => {
  if (!isAssistantLast(messages)) return messages;
  const blocks = messages[messages.length - 1].blocks;
  return replaceLastMessage(messages, [
    ...blocks,
    { kind: "confirm", id: newId(), interrupt, state: "pending" },
  ]);
};

const mapsConfirmation = (
  block: Block,
  interruptId: string,
): block is Extract<Block, { kind: "confirm" }> =>
  block.kind === "confirm" &&
  block.interrupt.InterruptContexts.some((context) => context.ID === interruptId);

export const setConfirmSubmitting = (
  messages: ChatMessage[],
  interruptId: string,
  approved: boolean,
): ChatMessage[] =>
  messages.map((message) => ({
    ...message,
    blocks: message.blocks.map((block) =>
      mapsConfirmation(block, interruptId)
        ? {
            ...block,
            state: approved ? "submitting_approval" : "submitting_rejection",
            error: undefined,
          }
        : block,
    ),
  }));

export const failConfirm = (messages: ChatMessage[], interruptId: string): ChatMessage[] =>
  messages.map((message) => ({
    ...message,
    blocks: message.blocks.map((block) =>
      mapsConfirmation(block, interruptId)
        ? { ...block, state: "failed", error: undefined, receipt: undefined }
        : block,
    ),
  }));

export const applyEffectReceipt = (
  messages: ChatMessage[],
  receipt: EffectReceipt,
): ChatMessage[] =>
  messages.map((message) => ({
    ...message,
    blocks: message.blocks.map((block) => {
      if (block.kind !== "confirm") return block;
      if (confirmationEffectID(block.interrupt) !== receipt.effect_id) return block;
      return {
        ...block,
        state: receipt.status === "rejected" ? "rejected" : "committed",
        receipt,
        error: undefined,
      };
    }),
  }));

/** Marks server-rejected mention bindings on the latest user request snapshot. */
export const applyDroppedMentions = (
  messages: ChatMessage[],
  dropped: DroppedMention[],
): ChatMessage[] => {
  if (dropped.length === 0) return messages;
  let index = -1;
  for (let candidate = messages.length - 1; candidate >= 0; candidate -= 1) {
    if (messages[candidate].role === "user" && messages[candidate].request) {
      index = candidate;
      break;
    }
  }
  if (index < 0) return messages;
  const next = messages.slice();
  const message = next[index];
  if (!message.request) return messages;
  next[index] = {
    ...message,
    request: {
      ...message.request,
      mentions: message.request.mentions.map((mention) => {
        const rejected = dropped.find(
          (candidate) => candidate.type === mention.type && candidate.id === mention.id,
        );
        return rejected ? { ...mention, status: "dropped", reason: rejected.reason } : mention;
      }),
    },
  };
  return next;
};

/** Removes a trailing empty assistant placeholder after a resume transport
 * failure while preserving any output already received. */
export const removeTrailingEmptyAssistant = (messages: ChatMessage[]): ChatMessage[] => {
  const last = messages[messages.length - 1];
  return last?.role === "assistant" && last.blocks.length === 0 ? messages.slice(0, -1) : messages;
};

/** Preserves partial assistant output while making the stopped state explicit
 * and preventing unfinished tools/confirmations from looking live. */
export const cancelActiveBlocks = (messages: ChatMessage[]): ChatMessage[] => {
  if (!isAssistantLast(messages)) return messages;
  const next = messages.slice();
  const last = next[next.length - 1];
  next[next.length - 1] = {
    ...last,
    status: "stopped",
    blocks: last.blocks.map((block) => {
      if (block.kind === "tool" && block.status === "running") {
        return { ...block, status: "cancelled" };
      }
      if (
        block.kind === "confirm" &&
        ["pending", "submitting_approval", "submitting_rejection", "failed"].includes(block.state)
      ) {
        return { ...block, state: "cancelled" };
      }
      return block;
    }),
  };
  return next;
};
