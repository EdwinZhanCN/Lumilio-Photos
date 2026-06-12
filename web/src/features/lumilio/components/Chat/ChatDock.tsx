import { useCallback, useState } from "react";
import { ChevronDown, ChevronUp, RotateCcw } from "lucide-react";
import { Link } from "react-router-dom";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n.tsx";
import { useLumilioChatStore } from "../../state/chatStore";
import { ChatMessages } from "./ChatMessages";
import { ChatInput } from "./ChatInput";

export function ChatDock() {
  const { t } = useI18n();
  const [collapsed, setCollapsed] = useState(false);
  const messages = useLumilioChatStore((s) => s.messages);
  const isGenerating = useLumilioChatStore((s) => s.isGenerating);
  const connectionError = useLumilioChatStore((s) => s.connectionError);
  const sendMessage = useLumilioChatStore((s) => s.sendMessage);
  const newConversation = useLumilioChatStore((s) => s.newConversation);
  const { capabilities } = useCapabilities(5000);
  const replyCount = messages.filter(
    (message) => message.role === "assistant" && message.blocks.length > 0,
  ).length;

  const agentDisabledReason =
    capabilities && !capabilities.llm.agentEnabled
      ? t("lumilio.agent.disabled")
      : capabilities && !capabilities.llm.configured
        ? t("lumilio.agent.notConfigured")
        : null;

  const toggleCollapsed = useCallback(
    () => setCollapsed((value) => !value),
    [],
  );
  const handleSubmit = useCallback(
    (value: string) => {
      setCollapsed(true);
      void sendMessage(value);
    },
    [sendMessage],
  );
  const toggleLabel = collapsed
    ? t("lumilio.dock.expand", "Expand chat")
    : t("lumilio.dock.collapse", "Collapse chat");

  return (
    <section className="absolute bottom-4 left-1/2 z-20 flex w-[min(42rem,calc(100%-2rem))] -translate-x-1/2 flex-col gap-2.5">
      <div
        id="lumilio-chat-dock-panel"
        aria-hidden={collapsed}
        inert={collapsed ? true : undefined}
        className={`origin-bottom overflow-hidden rounded-box border border-base-300 bg-base-100/95 backdrop-blur transition-[max-height,opacity,transform,margin] duration-300 ease-out ${
          collapsed
            ? "pointer-events-none -mb-2 max-h-0 translate-y-2 scale-[0.98] opacity-0"
            : "mb-0 max-h-[58vh] translate-y-0 scale-100 opacity-100"
        }`}
      >
        <header
          className="flex cursor-pointer items-center gap-3 border-b border-base-300 px-3 py-2"
          onClick={toggleCollapsed}
        >
          <div className="min-w-0 flex-1">
            <div className="truncate text-sm font-semibold text-base-content">
              {t("lumilio.dock.title", "Lumilio Agent")}
            </div>
            <div className="flex min-w-0 items-center gap-1.5 text-xs text-base-content/55">
              <span
                className={`h-3 w-3 shrink-0 rounded-full ${
                  isGenerating ? "bg-warning animate-pulse" : "bg-success"
                }`}
              />
              <span className="truncate">
                {isGenerating
                  ? t("lumilio.dock.busy", "Working")
                  : t("lumilio.dock.ready", "Ready")}
                <span aria-hidden="true"> · </span>
                {t("lumilio.dock.subtitle", "drives the board")}
              </span>
            </div>
          </div>
          {messages.length > 0 && (
            <button
              type="button"
              className="btn btn-ghost btn-sm btn-circle shrink-0 text-base-content/60"
              title={t("lumilio.chat.newConversation", "New conversation")}
              onClick={(event) => {
                event.stopPropagation();
                newConversation();
              }}
            >
              <RotateCcw size={18} strokeWidth={1.8} />
            </button>
          )}
          <button
            type="button"
            className="btn btn-ghost btn-sm btn-circle shrink-0 text-base-content/60"
            aria-controls="lumilio-chat-dock-panel"
            aria-expanded={!collapsed}
            title={toggleLabel}
            onClick={(event) => {
              event.stopPropagation();
              toggleCollapsed();
            }}
          >
            <ChevronDown size={18} strokeWidth={1.8} />
          </button>
        </header>

        <div className="max-h-[calc(58vh-3.5rem)] overflow-y-auto">
          {agentDisabledReason && (
            <div className="border-b border-base-300 bg-warning/10 px-3 py-2 text-xs text-base-content/80">
              <span>{agentDisabledReason}</span>{" "}
              <Link
                className="underline hover:opacity-80"
                to="/settings?tab=ai"
              >
                {t("lumilio.chat.openAiSettings")}
              </Link>
            </div>
          )}
          {connectionError && (
            <div className="border-b border-base-300 bg-error/10 px-3 py-1.5 text-xs text-error">
              {connectionError}
            </div>
          )}
          {messages.length === 0 ? (
            <div className="flex min-h-32 items-center justify-center px-3 py-6 text-center text-sm text-base-content/50">
              {t("lumilio.chat.empty")}
            </div>
          ) : (
            <ChatMessages messages={messages} isGenerating={isGenerating} />
          )}
        </div>
      </div>

      <div
        aria-hidden={!collapsed}
        inert={!collapsed ? true : undefined}
        className={`flex justify-center overflow-hidden transition-[max-height,opacity,transform] duration-[250ms] ease-out ${
          collapsed
            ? "max-h-12 translate-y-0 opacity-100"
            : "pointer-events-none max-h-0 translate-y-2 opacity-0"
        }`}
      >
        <button
          type="button"
          className="btn min-h-0 rounded-full border border-base-300 bg-base-100/95 px-3 text-sm backdrop-blur hover:bg-base-100"
          aria-controls="lumilio-chat-dock-panel"
          aria-expanded={!collapsed}
          title={toggleLabel}
          onClick={toggleCollapsed}
        >
          <ChevronUp size={16} strokeWidth={1.8} />
          <span className="font-semibold">
            {t("lumilio.dock.conversation", "Conversation")}
          </span>
          <span className="text-base-content/70">
            ·{" "}
            {t("lumilio.dock.replyCount", "{{count}} reply", {
              count: replyCount,
            })}
          </span>
          <span
            className={`ml-1 h-3 w-3 rounded-full ${
              isGenerating ? "bg-warning animate-pulse" : "bg-success"
            }`}
          />
        </button>
      </div>

      <ChatInput
        isGenerating={isGenerating}
        disabled={Boolean(agentDisabledReason)}
        disabledHint={agentDisabledReason ?? undefined}
        onSubmit={handleSubmit}
      />
    </section>
  );
}
