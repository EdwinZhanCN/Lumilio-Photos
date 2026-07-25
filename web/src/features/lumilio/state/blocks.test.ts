import { describe, expect, it } from "vite-plus/test";
import {
  applyChunk,
  applyInterrupt,
  applySideEvent,
  assistantMessage,
  cancelActiveBlocks,
  resolveConfirm,
  userMessage,
} from "./blocks";
import type { ChatMessage, SideChannelEvent } from "../model/chatTypes";

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

    const blocks = messages[1].blocks;
    expect(blocks).toHaveLength(1);
    expect(blocks[0]).toMatchObject({
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

    expect(messages[1].blocks.map((b) => b.kind)).toEqual(["text", "tool", "text"]);
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

describe("interrupts", () => {
  const interrupt = {
    InterruptContexts: [{ ID: "int-1", IsRootCause: true }],
  };

  it("appends a confirm block and resolves it", () => {
    let messages = conversation();
    messages = applyInterrupt(messages, interrupt);
    expect(messages[1].blocks[0]).toMatchObject({ kind: "confirm" });

    messages = resolveConfirm(messages, "approved");
    expect(messages[1].blocks[0]).toMatchObject({
      kind: "confirm",
      resolved: "approved",
    });
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
