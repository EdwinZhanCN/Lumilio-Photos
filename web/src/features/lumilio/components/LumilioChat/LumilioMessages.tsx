import { useRef, useEffect } from "react";
import { Markdown } from "../LumilioMarkdown/Markdown";
import type { ChatMessage } from "@/features/lumilio/lumilio.type.ts";
import { useI18n } from "@/lib/i18n.tsx";

/** Processes thinking tags in markdown content for display.
 *
 * Converts `<think>...</think>` tags into collapsible details elements.
 * During streaming, the last unclosed think tag is rendered as open to show active thinking.
 *
 * @param content - The markdown content string containing potential thinking tags.
 * @param thinkingSummary - The summary label for the thinking block.
 * @param isStreaming - Whether the content is currently being streamed from the agent.
 * @returns The processed content string with thinking tags converted to HTML details elements.
 */
function processThinkTags(
  content: string,
  thinkingSummary: string,
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
        ? `<details open><summary> ${thinkingSummary}</summary>`
        : `<details><summary> ${thinkingSummary}</summary>`;
    });
  } else {
    processed = processed.replace(
      /<think>/g,
      `<details><summary> ${thinkingSummary}</summary>`,
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
 * Uses a modern, flat ChatGPT-like design:
 * - User messages are short rounded bubbles on the right, plain text only.
 * - Assistant messages span the full width with no background, like reading an article.
 * - No avatars are shown.
 * - Thinking state is handled inline by the Markdown/ThinkBlock components.
 *
 * @param conversation - Array of chat messages to display.
 * @param isGenerating - Whether the agent is currently generating a response.
 */
export function LumilioMessages({
  conversation,
  isGenerating,
}: LumilioMessagesProps) {
  const { t } = useI18n();
  const messagesEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [conversation]);

  return (
    <div className="max-w-3xl mx-auto px-4 py-6 space-y-8">
      {conversation.map((message, index) => {
        const isLast = index === conversation.length - 1;
        const isStreamingHere =
          message.role === "assistant" && isGenerating && isLast;

        if (message.role === "user") {
          // User message: short rounded bubble, right-aligned, plain text (no markdown)
          return (
            <div key={message.id} className="flex justify-end">
              <div className="bg-primary text-primary-content rounded-2xl px-4 py-2.5 max-w-[80%] text-sm leading-relaxed">
                {message.content}
              </div>
            </div>
          );
        }

        // Assistant message: flat, full-width, no background, like an article
        return (
          <div key={message.id} className="w-full">
            {message.content && (
              <Markdown
                content={processThinkTags(
                  message.content,
                  t("lumilio.messages.thinking"),
                  isStreamingHere,
                )}
                className="text-base leading-relaxed text-base-content"
              />
            )}
          </div>
        );
      })}

      {/* Loading indicator when awaiting first assistant response */}
      {isGenerating &&
        conversation.length > 0 &&
        conversation[conversation.length - 1]?.role === "user" && (
          <div className="w-full">
            <div className="flex items-center gap-2 text-sm text-base-content/50">
              <span className="w-4 h-4 border-2 border-base-content/30 border-t-base-content/60 rounded-full animate-spin" />
              <span>{t("lumilio.messages.thinking")}</span>
            </div>
          </div>
        )}

      <div ref={messagesEndRef} />
    </div>
  );
}
