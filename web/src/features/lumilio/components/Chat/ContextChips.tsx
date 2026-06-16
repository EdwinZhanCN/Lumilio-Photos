import { X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import {
  useContextStore,
  type ContextContribution,
} from "../../state/contextStore";

interface ContextChipsProps {
  contributions: ContextContribution[];
}

export function ContextChips({ contributions }: ContextChipsProps) {
  const { t } = useI18n();
  const exclude = useContextStore((s) => s.exclude);

  if (contributions.length === 0) return null;

  return (
    <div className="flex flex-wrap items-center gap-1.5 px-2.5 text-xs">
      {contributions.map((item) => (
        <span
          key={item.id}
          className="badge badge-ghost badge-sm gap-1 pr-1 font-normal"
        >
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
