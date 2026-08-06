import { useLocation } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { useDockStore } from "@/lib/assistant";
import { useCapabilities } from "@/lib/capabilities/useCapabilities";
import { LumilioAvatar } from "@/components/assistant/LumilioAvatar";
import { useLumilioChatStore } from "../../state/chatStore";
import { resolveAgentAvailability } from "../../model/availability";

/** NavBar entry point for the global Agent drawer. The dot reflects verified
 * availability; it never defaults to green while capability state is unknown. */
export function AgentDockLauncher() {
  const { t } = useI18n();
  const location = useLocation();
  const collapsedOverride = useDockStore((state) => state.collapsedOverride);
  const setCollapsed = useDockStore((state) => state.setCollapsed);
  const isGenerating = useLumilioChatStore((state) => state.isGenerating);
  const connectionError = useLumilioChatStore((state) => state.connectionError);
  const capabilitiesQuery = useCapabilities(5000);
  const availability = resolveAgentAvailability({
    server: capabilitiesQuery.capabilities?.llm.availability,
    isLoading: capabilitiesQuery.isLoading,
    isError: capabilitiesQuery.isError,
    isGenerating,
    hasRuntimeError: Boolean(connectionError),
  });

  if (location.pathname === "/lumilio") return null;

  const open = collapsedOverride === false;
  const status = {
    checking: {
      label: t("lumilio.dock.checking", "Checking availability"),
      dot: "bg-base-content/30 animate-pulse",
    },
    disabled: {
      label: t("lumilio.dock.disabled", "Disabled"),
      dot: "bg-base-content/35",
    },
    not_configured: {
      label: t("lumilio.dock.notConfigured", "Not configured"),
      dot: "bg-warning",
    },
    ready: { label: t("lumilio.dock.ready", "Ready"), dot: "bg-success" },
    busy: { label: t("lumilio.dock.busy", "Working"), dot: "bg-warning animate-pulse" },
    degraded: { label: t("lumilio.dock.degraded", "Needs attention"), dot: "bg-warning" },
    unreachable: { label: t("lumilio.dock.unreachable", "Unavailable"), dot: "bg-error" },
  }[availability];

  return (
    <button
      type="button"
      className={`btn btn-sm sm:btn-md btn-ghost gap-1 rounded-full px-2 sm:gap-2 sm:px-3 ${
        open ? "btn-active" : ""
      }`}
      aria-controls="lumilio-chat-dock-panel"
      aria-expanded={open}
      aria-label={`Lumilio Agent · ${status.label}`}
      title={`Lumilio Agent · ${status.label}`}
      onClick={() => setCollapsed(open ? true : false)}
    >
      <LumilioAvatar start={isGenerating} size={0.11} />
      <span className={`h-2 w-2 shrink-0 rounded-full ${status.dot}`} aria-hidden="true" />
    </button>
  );
}
