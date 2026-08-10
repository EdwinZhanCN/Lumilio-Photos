import { useEffect, useRef } from "react";
import { AlertTriangle, AtSign, Images, WandSparkles } from "lucide-react";
import { Markdown } from "./markdown/Markdown";
import { ToolCallBlock } from "./blocks/ToolCallBlock";
import { ConfirmBlock } from "./blocks/ConfirmBlock";
import { InlineWidgetCard } from "../../modules/widgets/chrome/InlineWidgetCard";
import { useLumilioChatStore } from "../../state/chatStore";
import { useI18n } from "@/lib/i18n.tsx";
import type { AgentTurnSnapshot, Block, ChatMessage, WidgetBlock } from "../../model/chatTypes";

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
    case "tool":
      return <ToolCallBlock block={block} />;
    case "widget":
      return <WidgetBlockView block={block} />;
    case "confirm":
      return <ConfirmBlock block={block} />;
  }
}

function WidgetBlockView({ block }: { block: WidgetBlock }) {
  const threadId = useLumilioChatStore((state) => state.threadId);
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

function TurnScope({ request }: { request: AgentTurnSnapshot }) {
  const { t } = useI18n();
  if (request.mode === "free" && request.context.length === 0 && request.mentions.length === 0) {
    return null;
  }
  return (
    <div
      className="mt-1.5 flex max-w-[80%] flex-wrap justify-end gap-1 text-[11px] text-base-content/60"
      aria-label={t("lumilio.turnScope.label", "Scope sent to Lumilio Agent")}
    >
      {request.mode !== "free" && (
        <span className="badge badge-sm gap-1 border-primary/20 bg-primary/10 text-primary">
          <WandSparkles size={11} />
          {t(`lumilio.quickActions.${request.mode}.label`, request.mode)}
        </span>
      )}
      {request.context.map((item) => (
        <span key={`${item.type}:${item.id}`} className="badge badge-ghost badge-sm gap-1">
          <Images size={11} />
          {item.label} · {item.count}
        </span>
      ))}
      {request.mentions.map((mention) => (
        <span
          key={`${mention.type}:${mention.id}`}
          className={`badge badge-sm gap-1 ${
            mention.status === "dropped"
              ? "border-error/30 bg-error/10 text-error"
              : "badge-ghost"
          }`}
          title={
            mention.status === "dropped"
              ? t("lumilio.turnScope.mentionDropped", {
                  defaultValue: "This mention was not available to the Agent ({{reason}})",
                  reason: mention.reason ?? "rejected",
                })
              : undefined
          }
        >
          {mention.status === "dropped" ? <AlertTriangle size={11} /> : <AtSign size={11} />}
          {mention.label}
        </span>
      ))}
    </div>
  );
}

/** The conversation surface: user messages retain an immutable copy of the
 * mode, media scope, and entity bindings sent with that exact turn. */
export function ChatMessages({ messages, isGenerating }: ChatMessagesProps) {
  const { t } = useI18n();
  const endRef = useRef<HTMLDivElement>(null);
  const followLatestRef = useRef(true);

  useEffect(() => {
    const scrollContainer = endRef.current?.closest<HTMLElement>("[data-lumilio-chat-scroll]");
    if (!scrollContainer) return undefined;
    const updateFollow = () => {
      const distance = scrollContainer.scrollHeight - scrollContainer.scrollTop - scrollContainer.clientHeight;
      followLatestRef.current = distance < 96;
    };
    updateFollow();
    scrollContainer.addEventListener("scroll", updateFollow, { passive: true });
    return () => scrollContainer.removeEventListener("scroll", updateFollow);
  }, []);

  useEffect(() => {
    const last = messages[messages.length - 1];
    if (!followLatestRef.current && last?.role !== "user") return;
    const reduceMotion = window.matchMedia?.("(prefers-reduced-motion: reduce)").matches;
    endRef.current?.scrollIntoView({ behavior: reduceMotion ? "auto" : "smooth" });
  }, [messages]);

  const last = messages[messages.length - 1];
  const showSpinner = isGenerating && last?.role === "assistant" && last.blocks.length === 0;

  return (
    <div className="mx-auto max-w-3xl space-y-8 px-4 py-6" aria-live="polite" aria-relevant="additions text">
      {messages.map((message, messageIndex) => {
        if (message.role === "user") {
          const text = message.blocks
            .map((block) => (block.kind === "text" ? block.markdown : ""))
            .join("");
          return (
            <div key={message.id} className="flex flex-col items-end">
              <div className="max-w-[80%] whitespace-pre-wrap rounded-2xl bg-primary px-4 py-2.5 text-sm leading-relaxed text-primary-content">
                {text}
              </div>
              {message.request && <TurnScope request={message.request} />}
            </div>
          );
        }

        const isLastMessage = messageIndex === messages.length - 1;
        return (
          <div key={message.id} className="w-full">
            {message.blocks.map((block) => (
              <BlockView key={block.id} block={block} isAnimating={isGenerating && isLastMessage} />
            ))}
            {message.status === "stopped" && (
              <div className="mt-2 text-xs font-medium text-base-content/45">
                {t("lumilio.messages.stopped", "Stopped")}
              </div>
            )}
          </div>
        );
      })}

      {showSpinner && (
        <div className="flex items-center gap-2 text-sm text-base-content/50">
          <span className="h-4 w-4 animate-spin rounded-full border-2 border-base-content/30 border-t-base-content/60" />
          <span>{t("lumilio.messages.thinking")}</span>
        </div>
      )}
      <div ref={endRef} />
    </div>
  );
}
