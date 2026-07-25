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

describe("Lumilio chat run lifecycle", () => {
  let streamSignal: AbortSignal | undefined;
  let runSequence = 0;

  beforeEach(() => {
    streamAgentMock.mockReset();
    cancelAgentRunMock.mockReset();
    streamSignal = undefined;
    runSequence = 0;
    useLumilioChatStore.setState({
      threadId: null,
      activeRunId: null,
      messages: [],
      isGenerating: false,
      isStopping: false,
      awaitingConfirmation: false,
      connectionError: null,
      usage: null,
    });

    streamAgentMock.mockImplementation(
      async (
        _path: string,
        _body: unknown,
        callbacks: AgentStreamCallbacks,
        signal?: AbortSignal,
      ) => {
        runSequence += 1;
        streamSignal = signal;
        callbacks.onSessionInfo("thread-1", `run-${runSequence}`);
        callbacks.onRunStatus(`run-${runSequence}`, "running");
        await new Promise<void>((resolve) => signal?.addEventListener("abort", () => resolve()));
      },
    );
    cancelAgentRunMock.mockImplementation(async () => {
      expect(streamSignal?.aborted).toBe(false);
      return { thread_id: "thread-1", run_id: `run-${runSequence}`, status: "cancel_requested" };
    });
  });

  it("always sends explicit free mode and cancels server-side before aborting SSE", async () => {
    const sending = useLumilioChatStore.getState().sendMessage("hello");
    await vi.waitFor(() => expect(useLumilioChatStore.getState().activeRunId).toBe("run-1"));

    const body = streamAgentMock.mock.calls[0][1];
    expect(body).toMatchObject({ query: "hello", mode: "free" });

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
});
