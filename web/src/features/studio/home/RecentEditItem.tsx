import React from "react";
import { Clock, Pencil } from "lucide-react";
import { useI18n } from "@/lib/i18n";
import { PhotoThumb } from "@/features/studio/shared/PhotoThumb";
import { formatRelativeTime, type RecentEditRecord } from "./recentEditsStore";

type RecentEditItemProps = {
  item: RecentEditRecord;
  onResume: (assetId: string) => void;
};

export function RecentEditItem({
  item,
  onResume,
}: RecentEditItemProps): React.JSX.Element {
  const { t, i18n } = useI18n();

  return (
    <div className="group flex items-center gap-3 rounded-lg border border-base-300 bg-base-100 p-2 pr-3 transition-all hover:border-base-content/20 hover:shadow-sm focus-within:ring-1 focus-within:ring-primary/50">
      <PhotoThumb
        assetId={item.assetId}
        alt={item.name}
        className="h-12 w-12 shrink-0"
      />
      <div className="min-w-0 flex-1">
        <div className="truncate text-sm font-medium text-base-content">
          {item.name}
        </div>
        <div className="mt-0.5 flex items-center gap-1.5 text-xs text-base-content/55">
          <Clock size={11} />
          <span>{formatRelativeTime(item.editedAt, i18n.language)}</span>
          {item.width && item.height && (
            <>
              <span className="text-base-content/25">·</span>
              <span className="font-mono text-[10px]">
                {item.width}×{item.height}
              </span>
            </>
          )}
        </div>
      </div>
      <button
        type="button"
        onClick={() => onResume(item.assetId)}
        className="btn btn-ghost btn-xs gap-1 text-base-content/70 opacity-0 transition group-hover:opacity-100 focus:opacity-100"
      >
        <Pencil size={13} />
        {t("studio.home.resume", { defaultValue: "Resume" })}
      </button>
    </div>
  );
}
