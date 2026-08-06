import { beforeEach, describe, expect, it, vi } from "vite-plus/test";
import type { AgentStreamCallbacks } from "../api/agentStream";

const { streamAgentMock, cancelAgentRunMock } = vi.hoisted(() => ({
  streamAgentMock: vi.fn(),
  cancelAgentRunMock: vi.fn(),
}));

vi.mock("../api/agentStream", () => ({
  streamAgent: streamAgentMock,
  cancelAgentRun: cancelAgentRunMock,
}));

import { useLumilioChatStore } from "./chatStore";

const resetStore = () =>
  useLumilioChatStore.setState({
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

describe("Lumilio chat run lifecycle", () => {
  let streamSignal: AbortSignal | undefined;
  let runSequence = 0;

  beforeEach(() => {
    streamAgentMock.mockReset();
    cancelAgentRunMock.mockReset();
    streamSignal = undefined;
    runSequence = 0;
    resetStore();

    streamAgentMock.mockImplementation(
      async (
        _path: string,
        _body: unknown,
        callbacks: AgentStreamCallbacks,
        signal?: AbortSignal,
      ) => {
        runSequence += 1;
        streamSignal = signal;
        callbacks.onSessionInfo("thread-1", `run-${runSequence}`, []);
        callbacks.onRunStatus(`run-${runSequence}`, "running");
        await new Promise<void>((resolve) => signal?.addEventListener("abort", () => resolve()));
      },
    );
    cancelAgentRunMock.mockImplementation(async () => {
      expect(streamSignal?.aborted).toBe(false);
      return { thread_id: "thread-1", run_id: `run-${runSequence}`, status: "cancel_requested" };
    });
  });

  it("sends an immutable request snapshot and cancels server-side before aborting SSE", async () => {
    const sending = useLumilioChatStore.getState().sendMessage("hello", {
      mode: "review",
      context: [{ id: "selection", type: "selection", label: "2 selected", assetIds: ["a", "b"] }],
      mentions: [{ type: "person", id: "1", label: "Ada" }],
    });
    await vi.waitFor(() => expect(useLumilioChatStore.getState().activeRunId).toBe("run-1"));

    const body = streamAgentMock.mock.calls[0][1];
    expect(body).toMatchObject({ query: "hello", mode: "review" });
    expect(useLumilioChatStore.getState().messages[0].request).toEqual({
      mode: "review",
      context: [{ id: "selection", type: "selection", label: "2 selected", count: 2 }],
      mentions: [{ type: "person", id: "1", label: "Ada", status: "accepted" }],
    });

    await useLumilioChatStore.getState().stopGeneration();
    await sending;

    expect(cancelAgentRunMock).toHaveBeenCalledWith({
      thread_id: "thread-1",
      run_id: "run-1",
    });
    expect(streamSignal?.aborted).toBe(true);
    expect(useLumilioChatStore.getState()).toMatchObject({
      activeRunId: null,
      isGenerating: false,
      isStopping: false,
    });
    expect(useLumilioChatStore.getState().messages.at(-1)?.status).toBe("stopped");
  });

  it("cancels an active run before clearing a conversation", async () => {
    const sending = useLumilioChatStore.getState().sendMessage("hello");
    await vi.waitFor(() => expect(useLumilioChatStore.getState().activeRunId).toBe("run-1"));

    await useLumilioChatStore.getState().newConversation();
    await sending;

    expect(cancelAgentRunMock).toHaveBeenCalledTimes(1);
    expect(useLumilioChatStore.getState().threadId).toBeNull();
    expect(useLumilioChatStore.getState().messages).toEqual([]);
  });

  it("waits for the effect receipt before showing a confirmation as committed", async () => {
    streamAgentMock.mockReset();
    streamAgentMock
      .mockImplementationOnce(async (_path, _body, callbacks: AgentStreamCallbacks) => {
        callbacks.onSessionInfo("thread-1", "run-1", []);
        callbacks.onRunStatus("run-1", "running");
        callbacks.onInterrupt({
          InterruptContexts: [
            {
              ID: "interrupt-1",
              IsRootCause: true,
              Info: { effect_id: "effect-1", action: "tag_assets", count: 2 },
            },
          ],
        });
        callbacks.onRunStatus("run-1", "awaiting_confirmation");
        callbacks.onDone();
      })
      .mockImplementationOnce(async (_path, _body, callbacks: AgentStreamCallbacks) => {
        callbacks.onSessionInfo("thread-1", "run-2", []);
        callbacks.onRunStatus("run-2", "running");
        expect(
          useLumilioChatStore.getState().messages[1].blocks.find((block) => block.kind === "confirm"),
        ).toMatchObject({ state: "submitting_approval" });
        callbacks.onSideEvent({
          type: "effect_receipt",
          timestamp: Date.now(),
          tool: { name: "tag_assets", executionId: "exec-1" },
          execution: { status: "success" },
          receipt: {
            effect_id: "effect-1",
            tool_name: "tag_assets",
            status: "committed",
            count: 2,
            message: "Applied tag change to 2 assets",
          },
        });
        callbacks.onRunStatus("run-2", "completed");
        callbacks.onDone();
      });

    await useLumilioChatStore.getState().sendMessage("tag these");
    await useLumilioChatStore.getState().confirmInterrupt("interrupt-1", true);

    const confirm = useLumilioChatStore
      .getState()
      .messages.flatMap((message) => message.blocks)
      .find((block) => block.kind === "confirm");
    expect(confirm).toMatchObject({ state: "committed", receipt: { effect_id: "effect-1" } });
    expect(useLumilioChatStore.getState().pendingConfirmation).toBeNull();
  });
});
