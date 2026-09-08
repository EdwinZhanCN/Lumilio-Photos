import { useId, useRef } from "react";
import { MoreHorizontal } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function MusicMoreMenu({
  actions,
}: {
  actions: Array<{ label: string; onSelect: () => void; danger?: boolean }>;
}) {
  const { t } = useI18n();
  const id = useId();
  const menu = useRef<HTMLDivElement>(null);
  return (
    <>
      <button
        className="btn btn-ghost"
        popoverTarget={id}
        aria-label={t("music.actions.more", "More options")}
        onClick={(event) => {
          const rect = event.currentTarget.getBoundingClientRect();
          if (menu.current) {
            menu.current.style.left = `${Math.max(8, Math.min(rect.left, window.innerWidth - 248))}px`;
            menu.current.style.top = `${Math.max(8, Math.min(rect.bottom, window.innerHeight - actions.length * 42 - 80))}px`;
          }
        }}
      >
        <MoreHorizontal />
      </button>
      <div ref={menu} id={id} popover="auto" className="music-track-menu">
        {actions.map((action) => (
          <button
            key={action.label}
            className={action.danger ? "text-error" : ""}
            onClick={() => {
              menu.current?.hidePopover();
              action.onSelect();
            }}
          >
            {action.label}
          </button>
        ))}
      </div>
    </>
  );
}
