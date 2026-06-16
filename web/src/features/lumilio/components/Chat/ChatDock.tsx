import { useCallback, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronDown, ChevronUp, RotateCcw } from "lucide-react";
import { Link } from "react-router-dom";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n.tsx";
import { LumilioAvatar } from "../LumilioAvatar/LumilioAvatar";
import { useLumilioChatStore } from "../../state/chatStore";
import { useContextStore } from "../../state/contextStore";
import { useDockStore } from "../../state/dockStore";
import { QUICK_ASKS } from "../../slash/slashMacros";
import type { MentionPayload } from "../../mentions/mentionSources";
import { ChatMessages } from "./ChatMessages";
import { MentionInput } from "./MentionInput";
import { ContextChips } from "./ContextChips";

/** Compact token formatting: 856 → "856", 12480 → "12.5k". */
function formatTokens(count: number): string {
  if (count < 1000) return String(count);
  return `${(count / 1000).toFixed(1).replace(/\.0$/, "")}k`;
}

interface ChatDockProps {
  /** "embedded" = in-flow panel (Lumilio board page); "fab" = bottom-right FAB
   * whose expanded panel portals above gallery/carousel. */
  variant?: "embedded" | "fab";
  /** Hide the collapsed FAB trigger (e.g. while the carousel is open and renders
   * its own trigger) while keeping the dock mounted so the panel can still open. */
  hideTrigger?: boolean;
}

export function ChatDock({ variant = "embedded", hideTrigger = false }: ChatDockProps) {
  const { t } = useI18n();
  const [fabHovered, setFabHovered] = useState(false);
  const collapsedOverride = useDockStore((s) => s.collapsedOverride);
  const setCollapsedOverride = useDockStore((s) => s.setCollapsed);

  // fab defaults collapsed (just a button); embedded defaults expanded.
  const collapsed = collapsedOverride ?? variant === "fab";

  const contributions = useContextStore((s) => s.contributions);
  const excluded = useContextStore((s) => s.excluded);
  const snapshotForSend = useContextStore((s) => s.snapshotForSend);
  const clearExclusions = useContextStore((s) => s.clearExclusions);

  const activeContributions = useMemo(
    () => [...contributions.values()].filter((c) => !excluded.has(c.id)),
    [contributions, excluded],
  );

  const messages = useLumilioChatStore((s) => s.messages);
  const isGenerating = useLumilioChatStore((s) => s.isGenerating);
  const connectionError = useLumilioChatStore((s) => s.connectionError);
  const usage = useLumilioChatStore((s) => s.usage);
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

  const toggleCollapsed = useCallback(() => {
    setCollapsedOverride(!collapsed);
  }, [collapsed, setCollapsedOverride]);

  const handleSubmit = useCallback(
    (value: string, mentions: MentionPayload[]) => {
      setCollapsedOverride(false);
      void sendMessage(value, {
        context: snapshotForSend(),
        mentions,
      });
      clearExclusions();
    },
    [sendMessage, snapshotForSend, clearExclusions, setCollapsedOverride],
  );

  const toggleLabel = collapsed
    ? t("lumilio.dock.expand", "Expand chat")
    : t("lumilio.dock.collapse", "Collapse chat");

  const statusDot = (
    <span
      className={`h-3 w-3 shrink-0 rounded-full ${
        isGenerating ? "bg-warning animate-pulse" : "bg-success"
      }`}
    />
  );

  const header = (
    <header
      className="flex cursor-pointer items-center gap-3 border-b border-base-300 px-3 py-2"
      onClick={toggleCollapsed}
    >
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-semibold text-base-content">
          {t("lumilio.dock.title", "Lumilio Agent")}
        </div>
        <div className="flex min-w-0 items-center gap-1.5 text-xs text-base-content/55">
          {statusDot}
          <span className="truncate">
            {isGenerating
              ? t("lumilio.dock.busy", "Working")
              : t("lumilio.dock.ready", "Ready")}
            {usage && (
              <span
                title={t("lumilio.dock.usageHint", {
                  defaultValue:
                    "Last model call: {{prompt}} context + {{completion}} output tokens",
                  prompt: usage.promptTokens,
                  completion: usage.completionTokens,
                })}
              >
                <span aria-hidden="true"> · </span>
                {formatTokens(usage.totalTokens)}{" "}
                {t("lumilio.dock.tokens", "tokens")}
              </span>
            )}
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
  );

  const body = (
    <div className="max-h-[calc(58vh-3.5rem)] overflow-y-auto">
      {agentDisabledReason && (
        <div className="border-b border-base-300 bg-warning/10 px-3 py-2 text-xs text-base-content/80">
          <span>{agentDisabledReason}</span>{" "}
          <Link className="underline hover:opacity-80" to="/settings?tab=ai">
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
        <div className="flex min-h-32 flex-col items-center justify-center gap-3 px-3 py-6 text-center text-sm text-base-content/50">
          <p>{t("lumilio.chat.empty")}</p>
          <div className="flex flex-wrap justify-center gap-2">
            {QUICK_ASKS.map((prompt) => (
              <button
                key={prompt}
                type="button"
                className="btn btn-ghost btn-xs rounded-full border border-base-300"
                onClick={() => handleSubmit(prompt, [])}
              >
                {prompt}
              </button>
            ))}
          </div>
        </div>
      ) : (
        <ChatMessages messages={messages} isGenerating={isGenerating} />
      )}
    </div>
  );

  const inputArea = (
    <>
      <ContextChips contributions={activeContributions} />
      <MentionInput
        isGenerating={isGenerating}
        disabled={Boolean(agentDisabledReason)}
        placeholder={agentDisabledReason ?? undefined}
        onSubmit={handleSubmit}
      />
    </>
  );

  // ── FAB variant: collapsed button + portaled expanded panel ──────────────
  if (variant === "fab") {
    if (collapsed) {
      if (hideTrigger) return null;
      return (
        <button
          type="button"
          onMouseEnter={() => setFabHovered(true)}
          onMouseLeave={() => setFabHovered(false)}
          className="fixed bottom-20 right-4 z-40 flex w-12 justify-center transition-transform hover:scale-110"
          aria-controls="lumilio-chat-dock-panel"
          aria-expanded={false}
          title={toggleLabel}
          onClick={toggleCollapsed}
        >
          <LumilioAvatar start={fabHovered || isGenerating} size={0.2} />
        </button>
      );
    }

    return createPortal(
      <section className="fixed bottom-4 right-4 z-[10000] flex w-[min(28rem,calc(100vw-2rem))] flex-col gap-2">
        <div
          id="lumilio-chat-dock-panel"
          className="overflow-hidden rounded-box border border-base-300 bg-base-100/95 shadow-2xl backdrop-blur"
        >
          {header}
          {body}
        </div>
        {inputArea}
      </section>,
      document.body,
    );
  }

  // ── Embedded variant: in-flow centered panel (Lumilio board page) ─────────
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
        {header}
        {body}
      </div>

      <div
        aria-hidden={!collapsed}
        inert={!collapsed ? true : undefined}
        className={`flex justify-center overflow-hidden transition-[max-height,opacity,transform] duration-[250ms] ease-out ${
          collapsed
            ? "max-h-14 translate-y-0 opacity-100"
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

      {inputArea}
    </section>
  );
}
