import { useRef } from "react";
import { MoreHorizontal } from "lucide-react";
import { useI18n } from "@/lib/i18n";

export default function MusicMoreMenu({
  actions,
}: {
  actions: Array<{ label: string; onSelect: () => void; danger?: boolean }>;
}) {
  const { t } = useI18n();
  const details = useRef<HTMLDetailsElement>(null);
  return (
    <details ref={details} className="dropdown dropdown-end">
      <summary
        className="btn btn-ghost btn-sm"
        aria-label={t("music.actions.more", "More options")}
      >
        <MoreHorizontal />
      </summary>
      <ul className="menu dropdown-content bg-base-200 rounded-box z-dropdown w-48 p-2 shadow-xl">
        {actions.map((action) => (
          <li key={action.label}>
            <button
              className={action.danger ? "text-error" : ""}
              onClick={() => {
                details.current?.removeAttribute("open");
                action.onSelect();
              }}
            >
              {action.label}
            </button>
          </li>
        ))}
      </ul>
    </details>
  );
}
