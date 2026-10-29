import {
  AGENT_SESSION_STORAGE_KEY,
  AGENT_SESSION_STORAGE_VERSION,
} from "@/lib/settings/registry.ts";
import { isRecord, parseJSON } from "@/lib/settings/storage.ts";
import type { ChatMessage } from "../model/chatTypes";

/**
 * Per-tab Agent transcript persistence.
 *
 * The server keeps a thread's model memory for a bounded idle window
 * (`DefaultConversationTTL`, two hours) and confirmation effects durably. A
 * browser reload used to discard the local transcript and any in-flight
 * confirmation identity, so a committed effect whose receipt was lost could
 * stay ambiguous. Keeping the transcript in `sessionStorage` lets a reload
 * resume the same thread and reconcile a pending confirmation through the
 * scoped effect-status endpoint without replaying the mutation.
 *
 * `sessionStorage` is deliberate: the transcript is per tab, never shared
 * across tabs or browser restarts, and cleared by the session boundary.
 */

/** Mirrors the server conversation TTL; an older transcript has no memory left. */
export const AGENT_SESSION_MAX_AGE_MS = 2 * 60 * 60 * 1000;

export interface PersistedPendingConfirmation {
  interruptId: string;
  effectId?: string;
  approved: boolean;
}

export interface PersistedAgentSession {
  threadId: string;
  messages: ChatMessage[];
  pendingConfirmation: PersistedPendingConfirmation | null;
  savedAt: number;
}

function storage(): Storage | undefined {
  try {
    return typeof window === "undefined" ? undefined : window.sessionStorage;
  } catch {
    return undefined;
  }
}

export function loadAgentSession(now = Date.now()): PersistedAgentSession | null {
  const store = storage();
  if (!store) return null;
  let raw: unknown;
  try {
    raw = parseJSON(store.getItem(AGENT_SESSION_STORAGE_KEY));
  } catch {
    return null;
  }
  if (!isRecord(raw) || raw.version !== AGENT_SESSION_STORAGE_VERSION || !isRecord(raw.data)) {
    return null;
  }
  const data = raw.data;
  if (
    typeof data.threadId !== "string" ||
    !data.threadId ||
    !Array.isArray(data.messages) ||
    typeof data.savedAt !== "number" ||
    now - data.savedAt > AGENT_SESSION_MAX_AGE_MS
  ) {
    clearAgentSession();
    return null;
  }
  const pending = isRecord(data.pendingConfirmation) ? data.pendingConfirmation : null;
  return {
    threadId: data.threadId,
    messages: data.messages as ChatMessage[],
    pendingConfirmation:
      pending && typeof pending.interruptId === "string"
        ? {
            interruptId: pending.interruptId,
            effectId: typeof pending.effectId === "string" ? pending.effectId : undefined,
            approved: pending.approved === true,
          }
        : null,
    savedAt: data.savedAt,
  };
}

export function saveAgentSession(session: Omit<PersistedAgentSession, "savedAt">): void {
  const store = storage();
  if (!store) return;
  try {
    store.setItem(
      AGENT_SESSION_STORAGE_KEY,
      JSON.stringify({
        version: AGENT_SESSION_STORAGE_VERSION,
        data: { ...session, savedAt: Date.now() },
      }),
    );
  } catch {
    // Quota or privacy mode: the in-memory transcript remains authoritative.
  }
}

export function clearAgentSession(): void {
  try {
    storage()?.removeItem(AGENT_SESSION_STORAGE_KEY);
  } catch {
    // Storage may be unavailable; nothing was persisted then.
  }
}
