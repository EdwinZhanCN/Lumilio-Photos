import { describe, expect, it } from "vite-plus/test";
import {
  applyChunk,
  applyDroppedMentions,
  applyEffectReceipt,
  applyInterrupt,
  applySideEvent,
  assistantMessage,
  cancelActiveBlocks,
  failConfirm,
  setConfirmSubmitting,
  userMessage,
} from "./blocks";
import type { AgentTurnSnapshot, ChatMessage, SideChannelEvent } from "../model/chatTypes";

const conversation = (): ChatMessage[] => [userMessage("hello"), assistantMessage()];

const toolEvent = (overrides: Partial<SideChannelEvent> = {}): SideChannelEvent => ({
  type: "tool_execution",
  timestamp: Date.now(),
  tool: { name: "filter_assets", executionId: "exec-1" },
  execution: { status: "running", message: "Filtering..." },
  ...overrides,
});

describe("applyChunk", () => {
  it("appends streamed output into a single text block", () => {
    let messages = conversation();
    messages = applyChunk(messages, { output: "Hello" });
    messages = applyChunk(messages, { output: " world" });

    const blocks = messages[1].blocks;
    expect(blocks).toHaveLength(1);
    expect(blocks[0]).toMatchObject({ kind: "text", markdown: "Hello world" });
  });

  it("ignores chunks when the last message is not an assistant", () => {
    const messages = [userMessage("hi")];
    expect(applyChunk(messages, { output: "x" })).toBe(messages);
  });
});

describe("applySideEvent", () => {
  it("upserts tool blocks by executionId", () => {
    let messages = conversation();
    messages = applySideEvent(messages, toolEvent());
    messages = applySideEvent(
      messages,
      toolEvent({
        execution: { status: "success", message: "done" },
        data: { refId: "r1_kyoto", count: 97 },
      }),
    );

    expect(messages[1].blocks).toHaveLength(1);
    expect(messages[1].blocks[0]).toMatchObject({
      kind: "tool",
      executionId: "exec-1",
      status: "success",
      refId: "r1_kyoto",
      count: 97,
    });
  });

  it("interleaves tool blocks between text blocks", () => {
    let messages = conversation();
    messages = applyChunk(messages, { output: "Let me search." });
    messages = applySideEvent(messages, toolEvent());
    messages = applyChunk(messages, { output: "Found them." });
    expect(messages[1].blocks.map((block) => block.kind)).toEqual(["text", "tool", "text"]);
  });

  it("appends widget blocks from widget_show events", () => {
    let messages = conversation();
    messages = applySideEvent(
      messages,
      toolEvent({
        type: "widget_show",
        tool: { name: "show", executionId: "exec-2" },
        execution: { status: "success" },
        data: {
          refId: "r5_top24",
          count: 24,
          widget: "asset_grid",
          params: { title: "Kyoto 2025" },
        },
      }),
    );
    expect(messages[1].blocks[0]).toMatchObject({
      kind: "widget",
      refId: "r5_top24",
      count: 24,
      title: "Kyoto 2025",
    });
  });
});

describe("confirmation receipts", () => {
  const interrupt = {
    InterruptContexts: [
      { ID: "int-1", IsRootCause: true, Info: { effect_id: "effect-1", action: "tag_assets" } },
    ],
  };

  it("does not claim success before an authoritative effect receipt", () => {
    let messages = applyInterrupt(conversation(), interrupt);
    messages = setConfirmSubmitting(messages, "int-1", true);
    expect(messages[1].blocks[0]).toMatchObject({
      kind: "confirm",
      state: "submitting_approval",
    });

    messages = applyEffectReceipt(messages, {
      effect_id: "effect-1",
      tool_name: "tag_assets",
      status: "committed",
      count: 2,
      message: "Applied tag change to 2 assets",
    });
    expect(messages[1].blocks[0]).toMatchObject({
      kind: "confirm",
      state: "committed",
      receipt: { effect_id: "effect-1" },
    });
  });

  it("keeps a failed confirmation retryable", () => {
    let messages = applyInterrupt(conversation(), interrupt);
    messages = setConfirmSubmitting(messages, "int-1", true);
    messages = failConfirm(messages, "int-1");
    expect(messages[1].blocks[0]).toMatchObject({
      kind: "confirm",
      state: "failed",
      error: undefined,
    });
  });
});

describe("turn snapshots", () => {
  it("marks only the exact server-rejected mention", () => {
    const snapshot: AgentTurnSnapshot = {
      mode: "review",
      context: [{ id: "selected", type: "selection", label: "2 selected", count: 2 }],
      mentions: [
        { type: "person", id: "1", label: "Ada", status: "accepted" },
        { type: "person", id: "2", label: "Grace", status: "accepted" },
      ],
    };
    const messages = applyDroppedMentions(
      [userMessage("compare", snapshot), assistantMessage()],
      [{ type: "person", id: "2", label: "Grace", reason: "not_found" }],
    );
    expect(messages[0].request?.mentions).toEqual([
      { type: "person", id: "1", label: "Ada", status: "accepted" },
      { type: "person", id: "2", label: "Grace", status: "dropped", reason: "not_found" },
    ]);
  });
});

describe("cancelActiveBlocks", () => {
  it("preserves partial output and marks unfinished work stopped", () => {
    let messages = conversation();
    messages = applyChunk(messages, { output: "Partial answer" });
    messages = applySideEvent(messages, toolEvent());
    messages = cancelActiveBlocks(messages);

    expect(messages[1].status).toBe("stopped");
    expect(messages[1].blocks[0]).toMatchObject({ kind: "text", markdown: "Partial answer" });
    expect(messages[1].blocks[1]).toMatchObject({ kind: "tool", status: "cancelled" });
  });
});
