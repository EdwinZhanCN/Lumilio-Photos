import { useRef, useEffect } from "react";
import {Markdown, StaggeredMarkdown} from "../LumenMarkdown/Markdown";
import { LumenAvatar } from "../LumenAvatar/LumenAvatar";
import {useStreamingSplit} from "@/lib/utils/useStreamingSplit.ts";

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
    return (
      <div className="flex flex-1 overflow-y-auto p-4 space-y-4">
        <div className="text-center text-base-content/60 py-8">
          <div>Start a conversation with the Lumen!</div>
        </div>
        <div ref={messagesEndRef} />
      </div>
    );
  }

  return (
    <div className="flex-1 overflow-y-auto p-4 space-y-4">
      {conversation
        .filter((message) => message.role !== "system")
        .map((message, index) => {
          const isLast = index === conversation.length - 1;
          const isStreamingHere =
            message.role === "assistant" && isGenerating && isLast;

          return (
            <div
              key={index}
              className={`chat ${
                message.role === "user" ? "chat-end" : "chat-start"
              }`}
            >
              <div className="chat-image avatar">
                <div className="w-10 rounded-full">
                  {message.role === "user" ? (
                    <div className="w-full h-full flex items-center justify-center text-white font-bold rounded-full bg-primary">
                      U
                    </div>
                  ) : (
                    <div className="w-full h-full flex items-center justify-center">
                      <LumenAvatar start={isStreamingHere} size={0.1} />
                    </div>
                  )}
                </div>
              </div>
              <div className="chat-header">
                {message.role === "user" ? "You" : "Lumen"}
                <time className="text-xs opacity-50 ml-1">
                  {new Date(message.timestamp).toLocaleTimeString()}
                </time>
              </div>
              <div
                className={`chat-bubble ${
                  message.role === "user"
                    ? "chat-bubble-primary"
                    : "bg-base-200"
                }`}
              >
                {message.role === "assistant" ? (
                    (() => {
                      const { prefix, suffix } = useStreamingSplit(message.content, isStreamingHere);

                      return (
                          <>
                            {/* 旧前缀：正常渲染，完全不加动画，保留已渲染的结构 */}
                            {prefix && <Markdown content={processThinkTags(prefix, false)} />}

                            {/* 新增尾巴：逐词淡入（不走 Markdown 解析，避免半截 token 把树搞坏） */}
                            {suffix && (
                                <div className="text-base leading-relaxed text-base-content">
                                  <StaggeredMarkdown content={suffix} />
                                  <span className="lm-caret" />
                                </div>
                            )}

                            {/* 生成完毕：把整段换回正常 Markdown，收尾 */}
                            {!isStreamingHere && (
                                <Markdown content={processThinkTags(message.content, false)} />
                            )}
                          </>
                      );
                    })()
                ) : (
                    <div className="whitespace-pre-wrap">{message.content}</div>
                )}
              </div>
            </div>
          );
        })}

      <div ref={messagesEndRef} />
    </div>
  );
}
