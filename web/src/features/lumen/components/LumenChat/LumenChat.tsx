import React, { useState, useRef, useEffect } from "react";
import { LumenStatus } from "./LumenStatus";
import { LumenMessages } from "./LumenMessages";
import { LumenInput } from "./LumenInput";
import { useRuntime } from "../../runtime/AgentRuntimeProvider";

/**
 * LumenChat component that integrates with AgentRuntimeProvider
 * Uses custom runtime to handle agent interactions
 */
export function LumenChat() {
  // Get the runtime from AgentRuntimeProvider
  const { messages, isRunning, error, append } = useRuntime();

  // Local state for input handling
  const [inputMessage, setInputMessage] = useState("");
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const [hasError, setHasError] = useState(false);

  // Handle error display
  useEffect(() => {
    setHasError(!!error);
    if (error) {
      // Auto-hide error after 5 seconds
      const timer = setTimeout(() => setHasError(false), 5000);
      return () => clearTimeout(timer);
    }
  }, [error]);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const handleSendMessage = async () => {
    if (!inputMessage.trim() || isRunning) return;

    setInputMessage("");
    setHasError(false);

    await append({
      role: "user",
      content: [{ type: "text", text: inputMessage }],
    });
  };

  const handleKeyPress = (e: React.KeyboardEvent) => {
    if (e.key === "Enter" && !e.shiftKey) {
      e.preventDefault();
      handleSendMessage();
    }
  };

  // Convert our message format to the format expected by LumenMessages
  const conversation = messages.map((msg) => ({
    role: msg.role,
    content: msg.content
      .filter((part) => part.type === "text")
      .map((part) => part.text)
      .join("\n"),
    timestamp: msg.createdAt.getTime(),
  }));

  return (
    <div className="flex flex-col h-full bg-gradient-to-br from-base-100 via-base-100 to-base-200/30 rounded-xl shadow-lg overflow-hidden border border-base-200">
      {/* Error Banner */}
      {hasError && error && (
        <div className="relative mx-4 mt-4 p-3 rounded-lg bg-red-50 border border-red-200 text-red-700 shadow-sm animate-fade-in">
          <div className="flex items-start">
            <div className="flex-shrink-0">
              <svg
                className="w-5 h-5 text-red-600"
                fill="currentColor"
                viewBox="0 0 20 20"
              >
                <path
                  fillRule="evenodd"
                  d="M10 18a8 8 0 100-16 8 8 0 000 16zM8.707 7.293a1 1 0 00-1.414 1.414L8.586 10l-1.293 1.293a1 1 0 101.414 1.414L10 11.414l1.293 1.293a1 1 0 001.414-1.414L11.414 10l1.293-1.293a1 1 0 00-1.414-1.414L10 8.586 8.707 7.293z"
                  clipRule="evenodd"
                />
              </svg>
            </div>
            <div className="ml-3 flex-1">
              <h3 className="text-sm font-medium">Error</h3>
              <div className="mt-1 text-sm text-red-600">{error}</div>
            </div>
            <button
              className="ml-auto flex-shrink-0 p-1 rounded-md text-red-600 hover:bg-red-100 focus:outline-none"
              onClick={() => setHasError(false)}
            >
              <span className="sr-only">Dismiss</span>
              <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                <path
                  fillRule="evenodd"
                  d="M4.293 4.293a1 1 0 011.414 0L10 8.586l4.293-4.293a1 1 0 111.414 1.414L11.414 10l4.293 4.293a1 1 0 01-1.414 1.414L10 11.414l-4.293 4.293a1 1 0 01-1.414-1.414L8.586 10 4.293 5.707a1 1 0 010-1.414z"
                  clipRule="evenodd"
                />
              </svg>
            </button>
          </div>
        </div>
      )}

      <LumenStatus isInitializing={false} progress={null} />

      {/* Chat Messages */}
      <div className="flex-1 overflow-hidden">
        <LumenMessages
          conversation={conversation}
          isGenerating={isRunning}
          isInitializing={false}
        />
      </div>

      {/* Input Area */}
      <div className="relative z-10">
        <LumenInput
          inputMessage={inputMessage}
          isGenerating={isRunning}
          isInitializing={false}
          onInputChange={setInputMessage}
          onSendMessage={handleSendMessage}
          onKeyPress={handleKeyPress}
        />
      </div>

      <div ref={messagesEndRef} />
    </div>
  );
}

export default LumenChat;
