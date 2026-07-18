import { useEffect, useRef } from "react";
import { Markdown } from "./markdown/Markdown";
import { ReasoningBlock } from "./blocks/ReasoningBlock";
import { ToolCallBlock } from "./blocks/ToolCallBlock";
import { ConfirmBlock } from "./blocks/ConfirmBlock";
import { InlineWidgetCard } from "../../modules/widgets/chrome/InlineWidgetCard";
import { useLumilioChatStore } from "../../state/chatStore";
import { useI18n } from "@/lib/i18n.tsx";
import type { Block, ChatMessage, WidgetBlock } from "../../model/chatTypes";

interface ChatMessagesProps {
  messages: ChatMessage[];
  isGenerating: boolean;
}

function BlockView({ block, isAnimating = false }: { block: Block; isAnimating?: boolean }) {
  switch (block.kind) {
    case "text":
      return (
        <Markdown
          content={block.markdown}
          isAnimating={isAnimating}
          className="text-base leading-relaxed text-base-content"
        />
      );
    case "reasoning":
      return <ReasoningBlock block={block} />;
    case "tool":
      return <ToolCallBlock block={block} />;
    case "widget":
      return <WidgetBlockView block={block} />;
    case "confirm":
      return <ConfirmBlock block={block} />;
  }
}

/** Renders a chat widget inline and offers pinning it to the board. The widget
 * hydrates from the session ref; the View switcher (in the card footer) lets the
 * user retarget the agent's initial view before pinning. */
function WidgetBlockView({ block }: { block: WidgetBlock }) {
  const threadId = useLumilioChatStore((s) => s.threadId);
  if (!threadId) return null;
  return (
    <InlineWidgetCard
      refId={block.refId}
      threadId={threadId}
      widget={block.widget}
      count={block.count}
      title={block.title}
    />
  );
}

/** The conversation surface: user messages as right-aligned bubbles,
 * assistant messages as a flat column of typed blocks. */
export function ChatMessages({ messages, isGenerating }: ChatMessagesProps) {
  const { t } = useI18n();
  const endRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    endRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [messages]);

  const last = messages[messages.length - 1];
  const showSpinner = isGenerating && last?.role === "assistant" && last.blocks.length === 0;

  return (
    <div className="max-w-3xl mx-auto px-4 py-6 space-y-8">
      {messages.map((message, messageIndex) => {
        if (message.role === "user") {
          const text = message.blocks
            .map((block) => (block.kind === "text" ? block.markdown : ""))
            .join("");
          return (
            <div key={message.id} className="flex justify-end">
              <div className="bg-primary text-primary-content rounded-2xl px-4 py-2.5 max-w-[80%] text-sm leading-relaxed whitespace-pre-wrap">
                {text}
              </div>
            </div>
          );
        }

        const isLastMessage = messageIndex === messages.length - 1;

        return (
          <div key={message.id} className="w-full">
            {message.blocks.map((block) => (
              <BlockView key={block.id} block={block} isAnimating={isGenerating && isLastMessage} />
            ))}
          </div>
        );
      })}

      {showSpinner && (
        <div className="flex items-center gap-2 text-sm text-base-content/50">
          <span className="w-4 h-4 border-2 border-base-content/30 border-t-base-content/60 rounded-full animate-spin" />
          <span>{t("lumilio.messages.thinking")}</span>
        </div>
      )}

      <div ref={endRef} />
    </div>
  );
}
