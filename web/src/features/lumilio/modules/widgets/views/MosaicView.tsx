import { useI18n } from "@/lib/i18n.tsx";
import { fmt } from "../format";
import type { ViewBodyProps, WidgetSizeKey } from "../types";
import { useWidgetAssetsPreview } from "../useWidgetAssets";
import { WidgetAssetThumbnail } from "../WidgetAssetThumbnail";

/** Cells / grid template per tier. L features the first cell at 2×2. */
const GRID: Record<WidgetSizeKey, { cells: number; cls: string }> = {
  s: { cells: 4, cls: "grid-cols-2 grid-rows-2" },
  m: { cells: 9, cls: "grid-cols-3 grid-rows-3" },
  l: { cells: 12, cls: "grid-cols-4 grid-rows-3" },
};

/** Mosaic — a grid of representative thumbnails, denser as the tier grows. The
 * last visible cell shows a "+N" overflow badge when the set is larger than the
 * grid. Pure thumbnails; no facets required. */
export function MosaicView({ data, size, source }: ViewBodyProps) {
  const { t, i18n } = useI18n();
  const locale = i18n.language;
  const grid = GRID[size];
  const { assets, isLoading } = useWidgetAssetsPreview(source, grid.cells);

  const tiles = assets.slice(0, grid.cells);
  const shown = tiles.length;
  const extra = Math.max(0, data.count - shown);

  if (isLoading) {
    return (
      <div className={`grid h-full w-full gap-[2px] ${grid.cls}`}>
        {Array.from({ length: grid.cells }).map((_, i) => (
          <div key={i} className="skeleton" />
        ))}
      </div>
    );
  }

  return (
    <div className={`grid h-full w-full gap-[2px] ${grid.cls}`}>
      {tiles.map((asset, i) => {
        const featured = size === "l" && i === 0;
        const isLastShown = i === shown - 1 && extra > 0;
        return (
          <div
            key={asset.asset_id ?? i}
            className={`relative overflow-hidden bg-base-300 ${
              featured ? "col-span-2 row-span-2" : ""
            }`}
          >
            <WidgetAssetThumbnail
              asset={asset}
              source={source}
              className="absolute inset-0 h-full w-full object-cover transition-transform duration-500 group-hover:scale-[1.04]"
            />
            {isLastShown && (
              <div
                className={`absolute inset-0 flex items-center justify-center bg-black/55 font-bold tabular-nums text-white backdrop-blur-[1px] ${
                  size === "s" ? "text-xs" : "text-base"
                }`}
              >
                {t("lumilio.widgets.mosaic.overflow", "+{{value}}", { value: fmt(extra, locale) })}
              </div>
            )}
          </div>
        );
      })}
      {Array.from({ length: grid.cells - shown }).map((_, i) => (
        <div key={`empty-${i}`} className="bg-base-200/60" />
      ))}
    </div>
  );
}
