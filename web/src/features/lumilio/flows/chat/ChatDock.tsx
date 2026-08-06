import { lazy, Suspense, useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import { useAuth } from "@/features/auth";
import { useI18n } from "@/lib/i18n.tsx";
import { LumilioAvatar } from "@/components/assistant/LumilioAvatar";
import { useLumilioChatStore } from "../../state/chatStore";
import { useContextStore, useDockStore } from "@/lib/assistant";
import { useSlashMacros } from "../../modules/slash/slashMacros";
import type { MentionPayload } from "../../modules/mentions/mentionSources";
import type { AgentMode } from "../../model/chatTypes";
import { resolveAgentAvailability } from "../../model/availability";
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

/** Per-mode glyph, reused by the empty-state cards and the next-turn mode pill so
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
  const { user } = useAuth();
  const QUICK_ACTIONS = useSlashMacros();
  // Quick-action mode applies to one immutable turn. It is cleared only after
  // that turn is handed to the store, so a later question never silently
  // inherits a previous tool-subset constraint.
  const [activeMode, setActiveMode] = useState<Exclude<AgentMode, "free"> | null>(null);
  const drawerRef = useRef<HTMLElement>(null);
  const restoreFocusRef = useRef<HTMLElement | null>(null);
  const collapsedOverride = useDockStore((s) => s.collapsedOverride);
  const setCollapsedOverride = useDockStore((s) => s.setCollapsed);
  const setGenerating = useDockStore((s) => s.setGenerating);

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
  const isStopping = useLumilioChatStore((s) => s.isStopping);
  const awaitingConfirmation = useLumilioChatStore((s) => s.awaitingConfirmation);

  useEffect(() => {
    setGenerating(isGenerating);
  }, [isGenerating, setGenerating]);
  const connectionError = useLumilioChatStore((s) => s.connectionError);
  const usage = useLumilioChatStore((s) => s.usage);
  const sendMessage = useLumilioChatStore((s) => s.sendMessage);
  const newConversation = useLumilioChatStore((s) => s.newConversation);
  const stopGeneration = useLumilioChatStore((s) => s.stopGeneration);
  const capabilitiesQuery = useCapabilities(5000);
  const { capabilities } = capabilitiesQuery;
  const availability = resolveAgentAvailability({
    server: capabilities?.llm.availability,
    isLoading: capabilitiesQuery.isLoading,
    isError: capabilitiesQuery.isError,
    isGenerating,
    hasRuntimeError: Boolean(connectionError),
  });
  const replyCount = messages.filter(
    (message) => message.role === "assistant" && message.blocks.length > 0,
  ).length;

  const availabilityCopy = {
    checking: {
      label: t("lumilio.dock.checking", "Checking availability"),
      reason: t("lumilio.agent.checking", "Checking whether Lumilio Agent is available…"),
      dot: "bg-base-content/30 animate-pulse",
    },
    disabled: {
      label: t("lumilio.dock.disabled", "Disabled"),
      reason: t("lumilio.agent.disabled"),
      dot: "bg-base-content/35",
    },
    not_configured: {
      label: t("lumilio.dock.notConfigured", "Not configured"),
      reason: t("lumilio.agent.notConfigured"),
      dot: "bg-warning",
    },
    ready: {
      label: t("lumilio.dock.ready", "Ready"),
      reason: null,
      dot: "bg-success",
    },
    busy: {
      label: t("lumilio.dock.busy", "Working"),
      reason: null,
      dot: "bg-warning animate-pulse",
    },
    degraded: {
      label: t("lumilio.dock.degraded", "Needs attention"),
      reason: null,
      dot: "bg-warning",
    },
    unreachable: {
      label: t("lumilio.dock.unreachable", "Unavailable"),
      reason: t(
        "lumilio.agent.unreachable",
        "Lumilio Agent availability could not be verified. Check the server connection and retry.",
      ),
      dot: "bg-error",
    },
  } as const;
  const status = availabilityCopy[availability];
  const agentUnavailableReason = status.reason;

  const toggleCollapsed = useCallback(() => {
    setCollapsedOverride(!collapsed);
  }, [collapsed, setCollapsedOverride]);

  // The global drawer is a real modal region: opening moves focus inside,
  // Tab stays within it, Escape closes it, and focus returns to the launcher.
  useEffect(() => {
    if (!isDrawer || collapsed) return undefined;

    restoreFocusRef.current = document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null;
    const frame = window.requestAnimationFrame(() => {
      const panel = drawerRef.current;
      const target = panel?.querySelector<HTMLElement>(
        'button:not([disabled]), a[href], input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
      );
      (target ?? panel)?.focus();
    });

    const onKey = (event: KeyboardEvent) => {
      const panel = drawerRef.current;
      if (!panel) return;
      if (event.key === "Escape") {
        event.preventDefault();
        setCollapsedOverride(true);
        return;
      }
      if (event.key !== "Tab") return;

      const focusable = [...panel.querySelectorAll<HTMLElement>(
        'button:not([disabled]), a[href], input:not([disabled]), textarea:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])',
      )].filter((item) => item.getClientRects().length > 0);
      if (focusable.length === 0) {
        event.preventDefault();
        panel.focus();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (event.shiftKey && document.activeElement === first) {
        event.preventDefault();
        last.focus();
      } else if (!event.shiftKey && document.activeElement === last) {
        event.preventDefault();
        first.focus();
      }
    };

    document.addEventListener("keydown", onKey);
    return () => {
      window.cancelAnimationFrame(frame);
      document.removeEventListener("keydown", onKey);
      const restore = restoreFocusRef.current;
      restoreFocusRef.current = null;
      if (restore?.isConnected) restore.focus();
    };
  }, [isDrawer, collapsed, setCollapsedOverride]);

  const handleSubmit = useCallback(
    (value: string, mentions: MentionPayload[]) => {
      setCollapsedOverride(false);
      void sendMessage(value, {
        context: snapshotForSend(),
        mentions,
        mode: activeMode ?? "free",
      });
      setActiveMode(null);
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
      className={`h-3 w-3 shrink-0 rounded-full ${status.dot}`}
      aria-label={status.label}
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
        <div
          id={isDrawer ? "lumilio-chat-dock-title" : undefined}
          className="truncate text-sm font-semibold text-base-content"
        >
          {t("lumilio.dock.title", "Lumilio Agent")}
        </div>
        <div className="flex min-w-0 items-center gap-1.5 text-xs text-base-content/55">
          {statusDot}
          <span className="truncate">
            {status.label}
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
            void newConversation();
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
        aria-label={isDrawer ? t("lumilio.dock.close", "Close") : toggleLabel}
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
      {agentUnavailableReason && (
        <div
          className={`border-b border-base-300 px-3 py-2 text-xs text-base-content/80 ${
            availability === "unreachable" ? "bg-error/10" : "bg-warning/10"
          }`}
          role="status"
        >
          <span>{agentUnavailableReason}</span>{" "}
          {user?.role === "admin" && availability !== "unreachable" && (
            <Link className="underline hover:opacity-80" to="/settings?tab=ai">
              {t("lumilio.chat.openAiSettings")}
            </Link>
          )}
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
          <p className="max-w-md text-center text-xs leading-relaxed text-base-content/45">
            {t(
              "lumilio.chat.sessionDisclosure",
              "Conversation memory is temporary. Pins and completed actions remain in Lumilio Photos.",
            )}
          </p>
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

  const body = (
    <div data-lumilio-chat-scroll className="max-h-[calc(58vh-3.5rem)] overflow-y-auto">
      {bodyContent}
    </div>
  );

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
        awaitingConfirmation={awaitingConfirmation}
        isStopping={isStopping}
        disabled={Boolean(agentUnavailableReason)}
        placeholder={agentUnavailableReason ?? undefined}
        activeMode={activeMode}
        onSetMode={setActiveMode}
        onSubmit={handleSubmit}
        onStop={() => void stopGeneration()}
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
        <div
          aria-hidden
          onClick={() => setCollapsedOverride(true)}
          className={`fixed inset-0 z-agent bg-black/20 backdrop-blur-[1px] transition-opacity duration-300 ${
            collapsed ? "pointer-events-none opacity-0" : "opacity-100"
          }`}
        />
        <section
          ref={drawerRef}
          id="lumilio-chat-dock-panel"
          role="dialog"
          aria-modal="true"
          aria-labelledby="lumilio-chat-dock-title"
          aria-hidden={collapsed}
          tabIndex={-1}
          inert={collapsed ? true : undefined}
          className={`fixed inset-y-0 right-0 z-agent isolate flex w-[min(28rem,100vw)] flex-col border-l border-base-300 bg-base-100/95 shadow-xl backdrop-blur transition-transform duration-300 ease-out ${
            collapsed ? "translate-x-full" : "translate-x-0"
          }`}
        >
          {header}
          {!collapsed && (
            <>
              <div data-lumilio-chat-scroll className="min-h-0 flex-1 overflow-y-auto">{bodyContent}</div>
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
    <section className="absolute bottom-4 left-1/2 z-30 isolate flex w-[min(42rem,calc(100%-2rem))] -translate-x-1/2 flex-col gap-2.5">
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
          <span className={`ml-1 h-3 w-3 rounded-full ${status.dot}`} aria-label={status.label} />
        </button>
      </div>

      {!collapsed && inputArea}
    </section>
  );
}
