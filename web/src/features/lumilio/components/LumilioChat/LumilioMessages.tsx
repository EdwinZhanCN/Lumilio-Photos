import { useRef, useEffect } from "react";
import { Markdown } from "../LumilioMarkdown/Markdown";
import { LumilioAvatar } from "../LumilioAvatar/LumilioAvatar";
import type { ChatMessage } from "@/features/lumilio/lumilio.type.ts";

/** Processes thinking tags in markdown content for display.
 *
 * Converts special thinking tags (``...``) into collapsible details elements.
 * During streaming, the last unclosed think tag is rendered as open to show active thinking.
 *
 * @param content - The markdown content string containing potential thinking tags.
 * @param isStreaming - Whether the content is currently being streamed from the agent.
 * @returns The processed content string with thinking tags converted to HTML details elements.
 */
function processThinkTags(
  content: string,
  isStreaming: boolean = false,
): string {
  const openTags = (content.match(/<think>/g) || []).length;
  const closeTags = (content.match(/<\/think>/g) || []).length;
  const isCurrentlyThinking = openTags > closeTags;

  let processed = content;
  if (isCurrentlyThinking && isStreaming) {
    let openTagsReplaced = 0;
    processed = processed.replace(/<think>/g, () => {
      openTagsReplaced++;
      return openTagsReplaced === openTags
        ? "<details open><summary> Thinking...</summary>"
        : "<details><summary> Thinking...</summary>";
    });
  } else {
    processed = processed.replace(
      /<think>/g,
      "<details><summary> Thinking...</summary>",
    );
  }
  processed = processed.replace(/<\/think>/g, "</details>");
  return processed;
}

interface LumilioMessagesProps {
  conversation: ChatMessage[];
  isGenerating: boolean;
}

/** Renders the chat message list with automatic scrolling.
 *
 * Displays all messages in the conversation, including user and assistant messages.
 * Handles thinking tag processing for assistant messages and renders tool execution
 * UI components as dynamic content. Automatically scrolls to the newest message.
 *
 * @param conversation - Array of chat messages to display.
 * @param isGenerating - Whether the agent is currently generating a response.
 */
export function LumilioMessages({
  conversation,
  isGenerating,
}: LumilioMessagesProps) {
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [conversation]);

  return (
    <div className="p-4 space-y-4">
      {conversation.map((message, index) => {
        const isLast = index === conversation.length - 1;
        const isStreamingHere =
          message.role === "assistant" && isGenerating && isLast;

        return (
          <div
            key={message.id}
            className={`chat ${message.role === "user" ? "chat-end" : "chat-start"}`}
          >
            <div className="">
              <div className="w-10 rounded-full">
                {message.role === "assistant" && (
                  <div className="w-full h-full flex items-center">
                    <LumilioAvatar start={isStreamingHere} size={0.2} />
                  </div>
                )}
              </div>
            </div>
            <div
              className={` ${
                message.role === "user"
                  ? "chat-bubble chat-bubble-primary"
                  : "rounded-2xl bg-base-200 w-full fadeIn"
              }`}
            >
              {message.content && (
                <Markdown
                  content={processThinkTags(message.content, isStreamingHere)}
                  className={`${message.role === "user" ? "" : "mx-6 my-4"}`}
                />
              )}
            </div>
          </div>
        );
      })}

      {isGenerating &&
        conversation.length > 0 &&
        conversation[conversation.length - 1]?.role === "user" && (
          <div className="chat chat-start">
            <div className="chat-image avatar">
              <div className="w-10 rounded-full flex items-center justify-center">
                <LumilioAvatar start={true} size={0.1} />
              </div>
            </div>
            <div className="chat-bubble bg-base-200">
              <span className="loading loading-dots loading-md"></span>
            </div>
          </div>
        )}

      <div ref={messagesEndRef} />
    </div>
  );
}
