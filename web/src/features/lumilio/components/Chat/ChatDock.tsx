import { useCallback, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { ChevronDown, ChevronUp, Maximize2, RotateCcw } from "lucide-react";
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
}

export function ChatDock({ variant = "embedded" }: ChatDockProps) {
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
      className="flex cursor-pointer items-center gap-2 border-b border-base-300 px-3 py-2"
      onClick={toggleCollapsed}
    >
      <LumilioAvatar
        start={isGenerating}
        size={0.13}
        className="shrink-0"
      />
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
      {variant === "fab" && (
        <Link
          to="/lumilio"
          className="btn btn-ghost btn-sm btn-circle shrink-0 text-base-content/60"
          title={t("lumilio.dock.openBoard", "Open full board")}
          aria-label={t("lumilio.dock.openBoard", "Open full board")}
          onClick={(event) => event.stopPropagation()}
        >
          <Maximize2 size={16} strokeWidth={1.8} />
        </Link>
      )}
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

  // ── FAB variant: morphing pill ⇄ portaled expanded panel ──────────────────
  // The collapsed pill carries the avatar; expanding grows it into the panel
  // (whose header also shows the avatar), so the avatar stays put and morphs.
  if (variant === "fab") {
    return createPortal(
      <section className="fixed bottom-10 left-1/2 z-[10000] flex w-[min(28rem,calc(100vw-2rem))] -translate-x-1/2 flex-col gap-2">
        {/* Expanded: panel + input */}
        <div
          aria-hidden={collapsed}
          inert={collapsed ? true : undefined}
          className={`flex origin-bottom flex-col gap-2 overflow-hidden transition-[max-height,opacity,transform] duration-300 ease-out ${
            collapsed
              ? "pointer-events-none max-h-0 translate-y-2 scale-[0.98] opacity-0"
              : "max-h-[80vh] translate-y-0 scale-100 opacity-100"
          }`}
        >
          <div
            id="lumilio-chat-dock-panel"
            className="overflow-hidden rounded-box border border-base-300 bg-base-100/95 backdrop-blur"
          >
            {header}
            {body}
          </div>
          {inputArea}
        </div>

        {/* Collapsed: morphing avatar pill */}
        <div
          aria-hidden={!collapsed}
          inert={!collapsed ? true : undefined}
          className={`flex justify-center overflow-hidden transition-[max-height,opacity,transform] duration-[250ms] ease-out ${
            collapsed
              ? "max-h-16 translate-y-0 opacity-100"
              : "pointer-events-none max-h-0 translate-y-2 opacity-0"
          }`}
        >
          <button
            type="button"
            onMouseEnter={() => setFabHovered(true)}
            onMouseLeave={() => setFabHovered(false)}
            className="btn h-auto min-h-0 items-center gap-1.5 rounded-full border border-base-300 bg-base-100/95 py-1.5 pl-2 pr-4 hover:bg-base-100"
            aria-controls="lumilio-chat-dock-panel"
            aria-expanded={false}
            title={toggleLabel}
            onClick={toggleCollapsed}
          >
            <LumilioAvatar start={fabHovered || isGenerating} size={0.13} />
            <span className="text-sm font-semibold">
              {t("lumilio.dock.title", "Lumilio Agent")}
            </span>
            {statusDot}
          </button>
        </div>
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
