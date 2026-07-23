import { lazy, Suspense, useCallback, useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import {
  BarChart3,
  ChevronDown,
  ChevronUp,
  FolderTree,
  History,
  Maximize2,
  RotateCcw,
  Sparkles,
  X,
  type LucideIcon,
} from "lucide-react";
import { Link } from "react-router-dom";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { useI18n } from "@/lib/i18n.tsx";
import { LumilioAvatar } from "./avatar/LumilioAvatar";
import { useLumilioChatStore } from "../../state/chatStore";
import { useContextStore, useDockStore } from "@/lib/assistant";
import { useSlashMacros } from "../../modules/slash/slashMacros";
import type { MentionPayload } from "../../modules/mentions/mentionSources";
import { MentionInput } from "./MentionInput";
import { ContextChips } from "./ContextChips";

const ChatMessages = lazy(() =>
  import("./ChatMessages").then((module) => ({ default: module.ChatMessages })),
);

/** Compact token formatting: 856 → "856", 12480 → "12.5k". */
function formatTokens(count: number): string {
  if (count < 1000) return String(count);
  return `${(count / 1000).toFixed(1).replace(/\.0$/, "")}k`;
}

/** Per-mode glyph, reused by the empty-state cards and the sticky mode pill so
 * a mode reads the same wherever it appears. */
const MODE_ICON: Record<string, LucideIcon> = {
  review: History,
  organize: FolderTree,
  analyze: BarChart3,
  curate: Sparkles,
};

interface ChatDockProps {
  /** "embedded" = in-flow panel (Lumilio board page); "fab" = global
   * right-edge drawer portaled above the app, launched from AgentDockLauncher. */
  variant?: "embedded" | "fab";
}

export function ChatDock({ variant = "embedded" }: ChatDockProps) {
  const { t } = useI18n();
  const QUICK_ACTIONS = useSlashMacros();
  // Sticky agent mode: set once (empty-state chip, "/" menu, or Plus button),
  // kept across turns until the user clears it. Single source of truth for the
  // tool-subset constraint sent with every message.
  const [activeMode, setActiveMode] = useState<string | null>(null);
  const collapsedOverride = useDockStore((s) => s.collapsedOverride);
  const setCollapsedOverride = useDockStore((s) => s.setCollapsed);

  // fab defaults collapsed (drawer closed); embedded defaults expanded.
  const collapsed = collapsedOverride ?? variant === "fab";
  // "fab" now renders a right-edge drawer, launched from the NavBar button.
  const isDrawer = variant === "fab";

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

  // Drawer: Escape closes it, matching the scrim click-away.
  useEffect(() => {
    if (!isDrawer || collapsed) return;
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") setCollapsedOverride(true);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [isDrawer, collapsed, setCollapsedOverride]);

  const handleSubmit = useCallback(
    (value: string, mentions: MentionPayload[]) => {
      setCollapsedOverride(false);
      // Mode is sticky — read it here, don't clear it on send.
      void sendMessage(value, {
        context: snapshotForSend(),
        mentions,
        mode: activeMode ?? undefined,
      });
      clearExclusions();
    },
    [sendMessage, snapshotForSend, clearExclusions, setCollapsedOverride, activeMode],
  );

  const modeLabels: Record<string, string> = {
    review: t("lumilio.quickActions.review.label", "Review"),
    organize: t("lumilio.quickActions.organize.label", "Organize"),
    analyze: t("lumilio.quickActions.analyze.label", "Analyze"),
    curate: t("lumilio.quickActions.curate.label", "Curate"),
  };

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
      className={`flex items-center gap-2 border-b border-base-300 px-3 py-2 ${
        isDrawer ? "" : "cursor-pointer"
      }`}
      onClick={isDrawer ? undefined : toggleCollapsed}
    >
      <LumilioAvatar start={isGenerating} size={0.13} className="shrink-0" />
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-semibold text-base-content">
          {t("lumilio.dock.title", "Lumilio Agent")}
        </div>
        <div className="flex min-w-0 items-center gap-1.5 text-xs text-base-content/55">
          {statusDot}
          <span className="truncate">
            {isGenerating ? t("lumilio.dock.busy", "Working") : t("lumilio.dock.ready", "Ready")}
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
                {formatTokens(usage.totalTokens)} {t("lumilio.dock.tokens", "tokens")}
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
        title={isDrawer ? t("lumilio.dock.close", "Close") : toggleLabel}
        onClick={(event) => {
          event.stopPropagation();
          toggleCollapsed();
        }}
      >
        {isDrawer ? <X size={18} strokeWidth={1.8} /> : <ChevronDown size={18} strokeWidth={1.8} />}
      </button>
    </header>
  );

  const bodyContent = (
    <>
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
        <div className="flex flex-col items-center gap-4 px-4 py-7">
          <LumilioAvatar size={0.3} />
          <p className="text-center text-sm text-base-content/55">{t("lumilio.chat.empty")}</p>
          <div className="grid w-full max-w-md grid-cols-1 gap-2 sm:grid-cols-2">
            {QUICK_ACTIONS.map((action) => {
              const active = activeMode === action.mode;
              const Icon = MODE_ICON[action.mode] ?? Sparkles;
              return (
                <button
                  key={action.id}
                  type="button"
                  aria-pressed={active}
                  className={`group flex items-start gap-2.5 rounded-xl border p-3 text-left transition-colors ${
                    active
                      ? "border-primary bg-primary/5"
                      : "border-base-300 hover:border-primary/40 hover:bg-base-200/50"
                  }`}
                  onClick={() => setActiveMode((cur) => (cur === action.mode ? null : action.mode))}
                >
                  <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-primary/10 text-primary">
                    <Icon size={16} strokeWidth={1.8} />
                  </span>
                  <span className="min-w-0">
                    <span className="block text-sm font-medium text-base-content">
                      {action.label}
                    </span>
                    <span className="block text-xs leading-snug text-base-content/55">
                      {action.description}
                    </span>
                  </span>
                </button>
              );
            })}
          </div>
        </div>
      ) : (
        <Suspense
          fallback={
            <div className="flex min-h-32 items-center justify-center">
              <span className="loading loading-spinner loading-sm text-primary" />
            </div>
          }
        >
          <ChatMessages messages={messages} isGenerating={isGenerating} />
        </Suspense>
      )}
    </>
  );

  const body = <div className="max-h-[calc(58vh-3.5rem)] overflow-y-auto">{bodyContent}</div>;

  const ModePillIcon = activeMode ? (MODE_ICON[activeMode] ?? Sparkles) : null;
  const modePill =
    activeMode && ModePillIcon ? (
      <span className="inline-flex items-center gap-1 rounded-full bg-primary/10 px-2 py-0.5 font-medium text-primary">
        <ModePillIcon size={11} strokeWidth={2} />
        {modeLabels[activeMode] ?? activeMode}
        <button
          type="button"
          className="ml-0.5 opacity-60 hover:opacity-100"
          onClick={() => setActiveMode(null)}
          title={t("lumilio.mode.clear", "Clear mode")}
          aria-label={t("lumilio.mode.clear", "Clear mode")}
        >
          <X size={11} />
        </button>
      </span>
    ) : undefined;

  const inputArea = (
    <>
      <ContextChips contributions={activeContributions} leading={modePill} />
      <MentionInput
        isGenerating={isGenerating}
        disabled={Boolean(agentDisabledReason)}
        placeholder={agentDisabledReason ?? undefined}
        activeMode={activeMode}
        onSetMode={setActiveMode}
        onSubmit={handleSubmit}
      />
    </>
  );

  // ── FAB variant: right-edge drawer, launched from the NavBar button ────────
  // A chrome citizen with a home, not a floating orb: it owns a real region on
  // the right, slides in over content behind a light scrim, and is dismissed by
  // the scrim, Escape, or the header's close button.
  if (isDrawer) {
    return createPortal(
      <>
        {/* Scrim: dims content, click to dismiss. Sits above the fullscreen
         * asset viewer (z-9999) so the agent is reachable from inside it. */}
        <div
          aria-hidden
          onClick={() => setCollapsedOverride(true)}
          className={`fixed inset-0 z-[10000] bg-black/20 backdrop-blur-[1px] transition-opacity duration-300 ${
            collapsed ? "pointer-events-none opacity-0" : "opacity-100"
          }`}
        />
        <section
          id="lumilio-chat-dock-panel"
          aria-hidden={collapsed}
          inert={collapsed ? true : undefined}
          className={`fixed inset-y-0 right-0 z-[10001] flex w-[min(28rem,100vw)] flex-col border-l border-base-300 bg-base-100/95 shadow-xl backdrop-blur transition-transform duration-300 ease-out ${
            collapsed ? "translate-x-full" : "translate-x-0"
          }`}
        >
          {header}
          {!collapsed && (
            <>
              <div className="min-h-0 flex-1 overflow-y-auto">{bodyContent}</div>
              <div className="border-t border-base-300 p-2">{inputArea}</div>
            </>
          )}
        </section>
      </>,
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
        {!collapsed && body}
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
          <span className="font-semibold">{t("lumilio.dock.conversation", "Conversation")}</span>
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

      {!collapsed && inputArea}
    </section>
  );
}
