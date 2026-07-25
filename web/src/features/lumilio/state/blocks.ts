import type { Block, ChatMessage, InterruptInfo, SideChannelEvent } from "../model/chatTypes";

/** Pure reduction rules from SSE events onto the typed-block conversation.
 * The store applies these against the last assistant message; rendering never
 * parses strings for structure (no pseudo-tags). */

const newId = () => crypto.randomUUID();

export const userMessage = (content: string): ChatMessage => ({
  id: newId(),
  role: "user",
  blocks: [{ kind: "text", id: newId(), markdown: content }],
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

/** tool_execution events upsert a tool block by executionId; widget_show
 * appends a widget block that hydrates from the ref API. */
export const applySideEvent = (messages: ChatMessage[], event: SideChannelEvent): ChatMessage[] => {
  if (!isAssistantLast(messages)) return messages;
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
  return replaceLastMessage(messages, [...blocks, { kind: "confirm", id: newId(), interrupt }]);
};

/** Marks the pending confirm block as resolved (the resume stream follows
 * in a fresh assistant message). */
export const resolveConfirm = (
  messages: ChatMessage[],
  resolved: "approved" | "rejected" | "cancelled",
): ChatMessage[] =>
  messages.map((message) => ({
    ...message,
    blocks: message.blocks.map((block) =>
      block.kind === "confirm" && !block.resolved ? { ...block, resolved } : block,
    ),
  }));

/** Preserves partial assistant output while making the stopped state
 * explicit and preventing unfinished tools/confirmations from looking live. */
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
      if (block.kind === "confirm" && !block.resolved) {
        return { ...block, resolved: "cancelled" };
      }
      return block;
    }),
  };
  return next;
};
