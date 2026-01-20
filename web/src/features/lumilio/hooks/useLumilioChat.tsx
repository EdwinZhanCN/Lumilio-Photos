// src/features/lumilio/hooks/useLumilioChat.tsx
import { useContext } from "react";
import { LumilioChatContext } from "../LumilioChatProvider";
import { LumilioChatContextValue } from "../lumilio.types";

/**
 * Custom hook to access the Lumilio Chat context.
 *
 * This provides access to the chat state, dispatch function, and async actions
 * like `sendMessage` and `resumeConversation`.
 *
 * It must be used within a `LumilioChatProvider`.
 *
 * @returns The context value containing state, dispatch, and actions.
 */
export const useLumilioChat = (): LumilioChatContextValue => {
  const context = useContext(LumilioChatContext);
  if (context === undefined) {
    throw new Error("useLumilioChat must be used within a LumilioChatProvider");
  }
  return context;
};
