import { useRef, useEffect } from "react";
import { Markdown } from "../LumenMarkdown/Markdown";
import { LumenAvatar } from "../LumenAvatar/LumenAvatar";

// Function to process think tags based on whether thinking is in progress
function processThinkTags(
  content: string,
  isStreaming: boolean = false,
): string {
  // Count open and closed think tags
  const openTags = (content.match(/<think>/g) || []).length;
  const closeTags = (content.match(/<\/think>/g) || []).length;

  // Check if we're currently in a thinking state (more open than close tags)
  const isCurrentlyThinking = openTags > closeTags;

  let processed = content;

  // Replace think tags with details tags
  if (isCurrentlyThinking && isStreaming) {
    // If currently thinking during streaming, make the last unclosed think tag open
    let openTagsReplaced = 0;
    processed = processed.replace(/<think>/g, () => {
      openTagsReplaced++;
      // Make the last think tag (the currently active one) open
      return openTagsReplaced === openTags
        ? "<details open><summary> Thinking...</summary>"
        : "<details><summary> Thinking...</summary>";
    });
  } else {
    // All thinking is finished, close all details
    processed = processed.replace(
      /<think>/g,
      "<details><summary> Thinking...</summary>",
    );
  }

  processed = processed.replace(/<\/think>/g, "</details>");

  return processed;
}

interface LLMMessage {
  role: "user" | "assistant" | "system";
  content: string;
  timestamp: number;
}

interface LumenMessagesProps {
  conversation: LLMMessage[];
  isGenerating: boolean;
  isInitializing: boolean;
}

export function LumenMessages({
  conversation,
  isGenerating,
  isInitializing,
}: LumenMessagesProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  // Auto-scroll to bottom when new messages arrive
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [conversation]);

  if (conversation.length === 0 && !isGenerating && !isInitializing) {
    // Beautiful welcome interface
    return (
      <div className="flex flex-col p-8 overflow-y-auto">
        {/* Animated Lumen Avatar */}
        <div className="flex justify-center h-auto mb-8">
          <div className="relative">
            <div className="absolute inset-0 rounded-full bg-gradient-to-r from-blue-400 to-purple-600 opacity-20 blur-3xl scale-150 animate-pulse"></div>
            <div className="relative">
              <LumenAvatar start={true} size={0.5} />
            </div>
          </div>
        </div>

        {/* Side-by-side content */}
        <div className="flex flex-col md:flex-row gap-8 max-w-5xl mx-auto w-full animate-fade-in">
          {/* Welcome Text */}
          <div className="flex-1 space-y-4">
            <h1 className="text-3xl font-bold text-base-content">
              Welcome to{" "}
              <span className="text-transparent bg-clip-text bg-gradient-to-r from-blue-400 to-purple-600">
                Lumen
              </span>
            </h1>
            <p className="text-base-content/70 text-lg">
              Your intelligent photo assistant. Ask me anything about your
              photos, from finding specific moments to organizing your
              collection.
            </p>
          </div>

          {/* Conversation Starters */}
          <div className="flex-1">
            <h3 className="text-sm font-semibold text-base-content/60 mb-3">
              Try asking:
            </h3>
            <div className="space-y-2">
              {[
                "Find photos from my last vacation",
                "Show me pictures with my family",
                "What are my best photos from this year?",
                "Organize photos by location",
                "Find pictures of sunsets",
              ].map((prompt, index) => (
                <button
                  key={index}
                  className="block w-full text-left p-3 rounded-lg bg-base-200 hover:bg-base-300 transition-colors duration-200"
                  onClick={() => {
                    // This will need to be connected to the input component
                    const inputElement = document.querySelector(
                      "textarea",
                    ) as HTMLTextAreaElement;
                    if (inputElement) {
                      inputElement.value = prompt;
                      // Trigger onChange event to update React state
                      const event = new Event("input", { bubbles: true });
                      inputElement.dispatchEvent(event);
                    }
                  }}
                >
                  <span className="text-sm text-base-content">{prompt}</span>
                </button>
              ))}
            </div>
          </div>
        </div>
        <div ref={messagesEndRef} />
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto p-4 space-y-6 bg-gradient-to-b from-base-100 to-base-200/30">
      {conversation
        .filter((message) => message.role !== "system")
        .map((message, index) => {
          const isLast = index === conversation.length - 1;
          const isStreamingHere =
            message.role === "assistant" && isGenerating && isLast;

          return (
            <div
              key={index}
              className={`flex ${message.role === "user" ? "justify-end" : "justify-start"} animate-fade-in`}
            >
              <div
                className={`flex gap-3 max-w-[80%] ${message.role === "user" ? "flex-row-reverse" : ""}`}
              >
                {/* Avatar */}
                <div className="flex-shrink-0">
                  {message.role === "user" ? (
                    <div></div>
                  ) : (
                    <LumenAvatar start={isStreamingHere} size={0.2} />
                  )}
                </div>

                {/* Message Content */}
                <div
                  className={`flex flex-col ${message.role === "user" ? "items-end" : "items-start"}`}
                >
                  {/* Header with name and timestamp */}
                  <div className="flex items-center gap-2 mb-1">
                    <span className="text-sm font-medium text-base-content/80">
                      {message.role === "user" ? "You" : "Lumen"}
                    </span>
                    <time className="text-xs opacity-60">
                      {new Date(message.timestamp).toLocaleTimeString([], {
                        hour: "2-digit",
                        minute: "2-digit",
                      })}
                    </time>
                  </div>

                  {/* Message Bubble */}
                  <div
                    className={`relative px-4 py-3 rounded-2xl shadow-sm ${
                      message.role === "user"
                        ? "bg-base-200"
                        : "bg-base-100 rounded-bl-none border border-base-300"
                    }`}
                  >
                    {message.role === "assistant" ? (
                      <Markdown
                        content={processThinkTags(
                          message.content,
                          isStreamingHere,
                        )}
                      />
                    ) : (
                      <div className="whitespace-pre-wrap">
                        {message.content}
                      </div>
                    )}
                  </div>
                </div>
              </div>
            </div>
          );
        })}

      <div ref={messagesEndRef} />
    </div>
  );
}

export default LumenMessages;
