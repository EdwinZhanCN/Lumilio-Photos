import { afterEach, beforeEach, describe, expect, it, vi } from "vite-plus/test";
import { AGENT_SESSION_STORAGE_KEY } from "@/lib/settings/registry.ts";
import {
  AGENT_SESSION_MAX_AGE_MS,
  clearAgentSession,
  loadAgentSession,
  saveAgentSession,
} from "./chatSessionPersistence";
import { assistantMessage, userMessage } from "./blocks";

function memoryStorage(): Storage {
  const values = new Map<string, string>();
  return {
    get length() {
      return values.size;
    },
    clear: () => values.clear(),
    getItem: (key) => values.get(key) ?? null,
    key: (index) => [...values.keys()][index] ?? null,
    removeItem: (key) => void values.delete(key),
    setItem: (key, value) => void values.set(key, String(value)),
  };
}

describe("Agent session persistence", () => {
  let sessionStorage: Storage;

  beforeEach(() => {
    sessionStorage = memoryStorage();
    vi.stubGlobal("window", { sessionStorage });
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("round-trips a thread and its pending confirmation identity", () => {
    const messages = [userMessage("tag these"), assistantMessage()];
    saveAgentSession({
      threadId: "thread-1",
      messages,
      pendingConfirmation: { interruptId: "interrupt-1", effectId: "effect-1", approved: true },
    });

    expect(loadAgentSession()).toMatchObject({
      threadId: "thread-1",
      // JSON drops undefined fields such as a user turn's absent request snapshot.
      messages: JSON.parse(JSON.stringify(messages)),
      pendingConfirmation: { interruptId: "interrupt-1", effectId: "effect-1", approved: true },
    });
  });

  it("expires with the server conversation memory and removes the stale copy", () => {
    saveAgentSession({ threadId: "thread-1", messages: [], pendingConfirmation: null });
    expect(loadAgentSession(Date.now() + AGENT_SESSION_MAX_AGE_MS + 1)).toBeNull();
    expect(sessionStorage.getItem(AGENT_SESSION_STORAGE_KEY)).toBeNull();
  });

  it("ignores foreign or corrupt payloads", () => {
    sessionStorage.setItem(AGENT_SESSION_STORAGE_KEY, "{not json");
    expect(loadAgentSession()).toBeNull();
    sessionStorage.setItem(
      AGENT_SESSION_STORAGE_KEY,
      JSON.stringify({ version: 99, data: { threadId: "x", messages: [], savedAt: Date.now() } }),
    );
    expect(loadAgentSession()).toBeNull();
  });

  it("clears at the session boundary", () => {
    saveAgentSession({ threadId: "thread-1", messages: [], pendingConfirmation: null });
    clearAgentSession();
    expect(loadAgentSession()).toBeNull();
  });
});
