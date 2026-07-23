import { useI18n } from "@/lib/i18n.tsx";
import { useDockStore } from "@/lib/assistant";
import { LumilioAvatar } from "./LumilioAvatar";

interface AgentDockButtonProps {
  className?: string;
}

export function AgentDockButton({ className }: AgentDockButtonProps) {
  const { t } = useI18n();
  const setCollapsed = useDockStore((s) => s.setCollapsed);
  const isGenerating = useDockStore((s) => s.isGenerating);
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
