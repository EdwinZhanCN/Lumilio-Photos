import { beforeEach, describe, expect, it } from "vite-plus/test";
import { useContextStore } from "@/lib/assistant";
import { resetLumilioSession } from "./resetSession";
import { useLumilioChatStore } from "./chatStore";

describe("resetLumilioSession", () => {
  beforeEach(() => {
    resetLumilioSession();
  });

  it("clears user-scoped chat and contributed asset context", () => {
    useLumilioChatStore.setState({
      threadId: "thread-a",
      messages: [{ id: "message-a", role: "user", blocks: [] }],
    });
    useContextStore.setState({
      contributions: new Map([
        [
          "selection-a",
          { id: "selection-a", type: "selection", assetIds: ["private-a"], label: "A" },
        ],
      ]),
      excluded: new Set(["selection-a"]),
    });

    resetLumilioSession();

    expect(useLumilioChatStore.getState().threadId).toBeNull();
    expect(useLumilioChatStore.getState().messages).toEqual([]);
    expect(useContextStore.getState().contributions.size).toBe(0);
    expect(useContextStore.getState().excluded.size).toBe(0);
  });
});
