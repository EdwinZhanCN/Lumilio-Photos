import { useI18n } from "@/lib/i18n.tsx";
import { useDockStore } from "@/lib/assistant";
import { LumilioAvatar } from "./avatar/LumilioAvatar";
import { useLumilioChatStore } from "../../state/chatStore";

interface AgentDockViewerButtonProps {
  /** Positioning/layout classes owned by the host (e.g. the viewer chrome). */
  className?: string;
}

/** Avatar button that summons the global agent drawer from inside the
 * fullscreen asset viewer, where the NavBar launcher is out of reach. The
 * viewed asset is already registered as agent context, so opening the drawer
 * carries it in. */
export function AgentDockViewerButton({ className }: AgentDockViewerButtonProps) {
  const { t } = useI18n();
  const setCollapsed = useDockStore((s) => s.setCollapsed);
  const isGenerating = useLumilioChatStore((s) => s.isGenerating);
  const label = t("lumilio.viewer.ask", "Ask Lumilio about this photo");

  return (
    <button
      type="button"
      className={`cursor-pointer ${className ?? ""}`}
      aria-controls="lumilio-chat-dock-panel"
      title={label}
      aria-label={label}
      onClick={() => setCollapsed(false)}
    >
      <LumilioAvatar start={isGenerating} size={0.16} />
    </button>
  );
}
