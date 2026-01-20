// src/features/lumilio/routes/LumilioChat.tsx
import React, { useState } from "react";
import { LumilioChatProvider } from "../LumilioChatProvider";

import { useLumilioChat } from "../hooks/useLumilioChat";
import { LumilioInput, LumilioMessages } from "../components/LumilioChat";

/**
 * The main chat UI, which consumes the context provided by LumilioChatProvider.
 */
const ChatInterface: React.FC = () => {
  const { state, sendMessage, resumeConversation, dispatch } = useLumilioChat();
  const [activeTool, setActiveTool] = useState<string | null>(null);

  // Find the root cause of the interrupt from the context array.
  const rootCauseInterrupt = state.interrupt?.InterruptContexts?.find(
    (ctx) => ctx.IsRootCause,
  );

  const handleSubmit = (query: string) => {
    const toolNames = activeTool ? [activeTool] : [];
    sendMessage(query, toolNames);
    setActiveTool(null);
  };

  const handleConfirmation = (approved: boolean) => {
    // Use the pre-calculated rootCauseInterrupt.
    if (!rootCauseInterrupt) {
      console.error("No root cause found in interrupt");
      dispatch({ type: "CLEAR_INTERRUPT" });
      return;
    }
    const targets = { [rootCauseInterrupt.ID]: { approved } };
    resumeConversation(targets);
  };

  return (
    <div className="flex flex-col h-full bg-base-100 rounded-lg shadow-lg">
      {/* Scrollable Message Area */}
      <div className="flex-1 overflow-y-auto">
        {state.conversation.length === 0 && !state.isGenerating ? (
          <div className="flex h-full items-center justify-center">
            <div className="text-center text-base-content/60">
              <p>Start a conversation with Lumilio!</p>
            </div>
          </div>
        ) : (
          <LumilioMessages
            conversation={state.conversation}
            isGenerating={state.isGenerating}
          />
        )}
      </div>

      {/* Fixed Input Area at the bottom */}
      <div className="shrink-0 bg-base-100/80 backdrop-blur-sm border-t border-base-300">
        {/* Use the derived rootCauseInterrupt to safely access data */}
        {rootCauseInterrupt?.Info && (
          <div className="p-4 border-b border-base-300 bg-warning/10">
            <h4 className="font-bold text-warning">Confirmation Required</h4>
            <p className="text-sm my-2">{rootCauseInterrupt.Info.message}</p>
            <div className="flex gap-2">
              <button
                className="btn btn-sm btn-success"
                onClick={() => handleConfirmation(true)}
              >
                Confirm
              </button>
              <button
                className="btn btn-sm btn-error"
                onClick={() => handleConfirmation(false)}
              >
                Cancel
              </button>
            </div>
          </div>
        )}

        <LumilioInput
          isGenerating={state.isGenerating}
          isInitializing={state.connection.status === "connecting"}
          commands={state.tools.available}
          onSubmit={handleSubmit}
        />
      </div>
    </div>
  );
};

/**
 * The main route component for the Lumilio Chat page.
 * It wraps the UI with the state management provider.
 */
const LumilioChatPage: React.FC = () => {
  return (
    <LumilioChatProvider>
      <ChatInterface />
    </LumilioChatProvider>
  );
};

export default LumilioChatPage;
