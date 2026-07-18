import type { ReactNode } from "react";
import { X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useContextStore, type ContextContribution } from "@/lib/assistant";

interface ContextChipsProps {
  contributions: ContextContribution[];
  /** Rendered before the context chips, in the same row (e.g. the mode pill).
   * When present the row shows even if there are no context contributions. */
  leading?: ReactNode;
}

export function ContextChips({ contributions, leading }: ContextChipsProps) {
  const { t } = useI18n();
  const exclude = useContextStore((s) => s.exclude);

  if (!leading && contributions.length === 0) return null;

  return (
    <div className="flex flex-wrap items-center gap-1.5 px-2.5 text-xs">
      {leading}
      {contributions.map((item) => (
        <span key={item.id} className="badge badge-ghost badge-sm gap-1 pr-1 font-normal">
          {item.label}
          <button
            type="button"
            className="btn btn-ghost btn-xs btn-circle min-h-0 h-4 w-4"
            title={t("lumilio.context.remove", "Remove")}
            onClick={() => exclude(item.id)}
          >
            <X size={12} />
          </button>
        </span>
      ))}
    </div>
  );
}
