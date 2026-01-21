// src/features/lumilio/routes/LumilioChat.tsx
import React, { useState } from "react";
import { LumilioChatProvider } from "../LumilioChatProvider";

import { useLumilioChat } from "../hooks/useLumilioChat";
import { LumilioInput, LumilioMessages } from "../components/LumilioChat";

/** Main chat interface component that displays conversation and handles user input.
 *
 * Renders the message history, input field, and confirmation dialogs for tool
 * executions. Consumes the Lumilio chat context to manage state and dispatch
 * actions for sending messages and resuming conversations after interrupts.
 */
const ChatInterface: React.FC = () => {
  const { state, sendMessage, resumeConversation, dispatch } = useLumilioChat();
  const [activeTool, setActiveTool] = useState<string | null>(null);

  const rootCauseInterrupt = state.interrupt?.InterruptContexts?.find(
    (ctx) => ctx.IsRootCause,
  );

  /** Handles submission of user messages to the agent.
   *
   * Sends the query to the agent with optional tool names if a tool is selected,
   * then resets the active tool state.
   *
   * @param query - The text content of the user's message.
   */
  const handleSubmit = (query: string) => {
    const toolNames = activeTool ? [activeTool] : [];
    sendMessage(query, toolNames);
    setActiveTool(null);
  };

  /** Handles user confirmation or cancellation of tool execution interrupts.
   *
   * Processes the user's decision (approve or cancel) and resumes the conversation
   * with the appropriate target data.
   *
   * @param approved - True if the user confirmed the action, false if cancelled.
   */
  const handleConfirmation = (approved: boolean) => {
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

/** Main route component for the Lumilio Chat page.
 *
 * Wraps the chat interface with the Lumilio chat provider to enable state
 * management and context sharing across all child components in the chat
 * interface tree.
 */
const LumilioChatPage: React.FC = () => {
  return (
    <LumilioChatProvider>
      <ChatInterface />
    </LumilioChatProvider>
  );
};

export default LumilioChatPage;
