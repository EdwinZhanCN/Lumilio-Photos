import { useLocation } from "react-router-dom";
import { useI18n } from "@/lib/i18n.tsx";
import { useDockStore } from "@/lib/assistant";
import { LumilioAvatar } from "@/components/assistant/LumilioAvatar";
import { useLumilioChatStore } from "../../state/chatStore";

/** NavBar entry point for the global agent drawer. Lives in the right cluster
 * beside Messages and the upload queue — the agent is a chrome citizen with a
 * home, not a floating orb. Hidden on `/lumilio`, which embeds its own dock. */
export function AgentDockLauncher() {
  const { t } = useI18n();
  const location = useLocation();
  const collapsedOverride = useDockStore((s) => s.collapsedOverride);
  const setCollapsed = useDockStore((s) => s.setCollapsed);
  const isGenerating = useLumilioChatStore((s) => s.isGenerating);

  // Mirrors AppShellLayout: the board page owns its embedded dock.
  if (location.pathname === "/lumilio") return null;

  // Drawer defaults closed; only an explicit `false` means open.
  const open = collapsedOverride === false;

  return (
    <button
      type="button"
      className={`btn btn-sm sm:btn-md btn-ghost gap-1 sm:gap-2 rounded-full px-2 sm:px-3 ${
        open ? "btn-active" : ""
      }`}
      aria-controls="lumilio-chat-dock-panel"
      aria-expanded={open}
      title={t("lumilio.dock.title", "Lumilio Agent")}
      onClick={() => setCollapsed(open ? true : false)}
    >
      <LumilioAvatar start={isGenerating} size={0.11} />
      <span
        className={`h-2 w-2 shrink-0 rounded-full ${
          isGenerating ? "bg-warning animate-pulse" : "bg-success"
        }`}
      />
    </button>
  );
}
