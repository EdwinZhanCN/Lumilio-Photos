import { AlertTriangle, ImageOff, RotateCw } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import type { WidgetSizeKey } from "../types";

/** Mosaic cell counts / grid template per tier (shared with MosaicView). */
const MOSAIC_GRID: Record<WidgetSizeKey, { cells: number; cls: string }> = {
  s: { cells: 4, cls: "grid-cols-2 grid-rows-2" },
  m: { cells: 9, cls: "grid-cols-3 grid-rows-3" },
  l: { cells: 12, cls: "grid-cols-4 grid-rows-3" },
};

/** Per-view loading skeletons, shaped like the view they stand in for so the
 * tile doesn't visibly reflow when data lands. */
export function LoadingState({ view, size }: { view: string; size: WidgetSizeKey }) {
  if (view === "spark_card") {
    const bars = size === "s" ? 7 : 12;
    return (
      <div className="flex flex-1 items-end gap-[3px] p-3">
        {Array.from({ length: bars }).map((_, i) => (
          <div
            key={i}
            className="skeleton flex-1 rounded-t-[3px]"
            style={{ height: `${25 + ((i * 37) % 70)}%` }}
          />
        ))}
      </div>
    );
  }
  if (view === "number_card") {
    return (
      <div className="flex flex-1 flex-col items-center justify-center gap-2">
        <div className="skeleton h-9 w-24 rounded-lg" />
        <div className="skeleton h-3 w-16 rounded" />
      </div>
    );
  }
  if (view === "mosaic_card") {
    const grid = MOSAIC_GRID[size];
    return (
      <div className={`grid flex-1 gap-[2px] ${grid.cls}`}>
        {Array.from({ length: grid.cells }).map((_, i) => (
          <div key={i} className="skeleton" />
        ))}
      </div>
    );
  }
  // Cover: full-bleed skeleton with two faint bars mimicking title + meta.
  return (
    <div className="skeleton absolute inset-0 flex items-end p-3">
      <div className="w-full space-y-2">
        <div className="skeleton h-4 w-2/3 rounded bg-base-100/40" />
        <div className="skeleton h-3 w-1/3 rounded bg-base-100/40" />
      </div>
    </div>
  );
}

export function ErrorState({ size, onRetry }: { size: WidgetSizeKey; onRetry?: () => void }) {
  const { t } = useI18n();
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-1.5 bg-base-100 px-3 text-center">
      <AlertTriangle className="text-error/80" size={size === "s" ? 20 : 28} strokeWidth={1.75} />
      {size !== "s" && (
        <div className="text-xs font-semibold text-base-content/70">
          {t("lumilio.widgets.state.error", "Couldn't load")}
        </div>
      )}
      <button
        type="button"
        className="btn btn-ghost btn-xs gap-1 text-base-content/60"
        onClick={(e) => {
          e.stopPropagation();
          onRetry?.();
        }}
      >
        <RotateCw size={14} strokeWidth={1.75} />
        {size !== "s" && t("lumilio.widgets.state.retry", "Retry")}
      </button>
    </div>
  );
}

export function EmptyState({ size }: { size: WidgetSizeKey }) {
  const { t } = useI18n();
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-1.5 bg-base-100 px-3 text-center">
      <ImageOff className="text-base-content/25" size={size === "s" ? 20 : 28} strokeWidth={1.75} />
      {size !== "s" && (
        <div className="text-xs font-medium text-base-content/45">
          {t("lumilio.widgets.state.empty", "No photos here")}
        </div>
      )}
    </div>
  );
}
