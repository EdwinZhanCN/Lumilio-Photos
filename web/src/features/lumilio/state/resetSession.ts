import { useContextStore } from "@/lib/assistant";
import { useLumilioChatStore } from "./chatStore";

/** Clear all user-scoped Lumilio state at the application session boundary. */
export function resetLumilioSession(): void {
  useLumilioChatStore.getState().resetSession();
  useContextStore.getState().resetSession();
}
