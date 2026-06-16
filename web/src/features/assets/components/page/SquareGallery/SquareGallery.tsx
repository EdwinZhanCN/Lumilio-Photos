import React, { memo, useCallback, useMemo, useRef } from "react";
import MediaThumbnail from "@/features/assets/components/shared/MediaThumbnail";
import StackedThumbnail from "@/features/assets/components/shared/StackedThumbnail";
import { useOptionalKeyboardSelection } from "@/features/assets/hooks/useSelection";
import { assetUrls } from "@/lib/assets/assetUrls";
import { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { AssetGalleryProps } from "../gallery.types";
import {
  DEFAULT_GROUP_KEYS,
  formatAssetGroupLabel,
} from "@/features/assets/utils/assetGroups";
import EmptyState from "@/components/EmptyState";
import { getBrowseItemAsset } from "@/features/assets/utils/browseItems";
import type { BrowseItem } from "@/features/assets/types/assets.type";
import { useGalleryInfiniteScroll } from "@/features/assets/hooks/useGalleryInfiniteScroll";
import { useVisibleOnce } from "@/features/assets/hooks/useVisibleOnce";

interface SquareGalleryProps extends AssetGalleryProps {
  renderTileCaption?: (
    asset: Asset,
    index: number,
    groupKey: string,
  ) => React.ReactNode;
  render3DCard?: boolean;
}

// ---------------------------------------------------------------------------
// SquareGalleryItem
// ---------------------------------------------------------------------------

interface SquareGalleryItemProps {
  item: BrowseItem;
  asset: Asset;
  thumbnailUrl?: string;
  render3DCard: boolean;
  caption?: React.ReactNode;
  isSelected: boolean;
  isSelectionMode: boolean;
  onItemClick: (
    item: BrowseItem,
    asset: Asset,
    event: React.MouseEvent | React.KeyboardEvent,
  ) => void;
}

/**
 * Individual cell in the square grid.
 * - Outer div (grid cell) always stays in the DOM.
 * - Content mounts once the cell enters the viewport (useVisibleOnce).
 * - content-visibility: auto + containIntrinsicSize skips paint for
 *   off-screen cells. "auto <fallback>" lets the browser remember the real
 *   rendered size so the scrollbar stays stable after first paint.
 */
const SquareGalleryItem = memo(
  ({
    item,
    asset,
    thumbnailUrl,
    render3DCard,
    caption,
    isSelected,
    isSelectionMode,
    onItemClick,
  }: SquareGalleryItemProps) => {
    const [ref, mounted] = useVisibleOnce();
    const assetId = asset.asset_id;
    const stackInfo = asset.stack;
    const hasStackOverlay =
      Boolean(stackInfo?.stack_size) && (stackInfo?.stack_size ?? 0) > 1;
    const allowOverflow = render3DCard || hasStackOverlay;
    const visibilityStyle = allowOverflow
      ? {}
      : {
          contentVisibility: "auto",
          // "auto" = remember last rendered size; 200px = initial fallback.
          containIntrinsicSize: "auto 200px",
        };

    return (
      <div
        ref={ref}
        className={
          render3DCard
            ? "hover-3d relative aspect-square"
            : "relative aspect-square"
        }
        role="listitem"
        data-asset-id={assetId}
        style={visibilityStyle as React.CSSProperties}
      >
        {mounted ? (
          <>
            <figure className="h-full w-full rounded-[1.25rem]">
              {stackInfo && stackInfo.stack_size && stackInfo.stack_size > 1 ? (
                <StackedThumbnail
                  asset={asset}
                  thumbnailUrl={thumbnailUrl}
                  stackInfo={stackInfo}
                  browseStack={item.type === "stack" ? item : undefined}
                  onClick={(event) => onItemClick(item, asset, event)}
                  isSelected={isSelected}
                  isSelectionMode={isSelectionMode}
                  className="rounded-[1.25rem] focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/70"
                />
              ) : (
                <MediaThumbnail
                  asset={asset}
                  thumbnailUrl={thumbnailUrl}
                  onClick={(event) => onItemClick(item, asset, event)}
                  isSelected={isSelected}
                  isSelectionMode={isSelectionMode}
                  className="rounded-[1.25rem] focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/70"
                />
              )}
              {caption && (
                <div className="pointer-events-none absolute inset-x-0 bottom-0 z-30 rounded-b-2xl bg-gradient-to-t from-black/70 via-black/20 to-transparent px-4 pb-3 pt-10 text-sm text-white">
                  {caption}
                </div>
              )}
            </figure>
            {render3DCard && (
              <>
                <div />
                <div />
                <div />
                <div />
                <div />
                <div />
                <div />
                <div />
              </>
            )}
          </>
        ) : (
          <div className="skeleton absolute inset-0 h-full w-full rounded-[1.25rem] bg-base-300" />
        )}
      </div>
    );
  },
);
SquareGalleryItem.displayName = "SquareGalleryItem";

// ---------------------------------------------------------------------------

const SquareGallery: React.FC<SquareGalleryProps> = ({
  browseGroups,
  openCarousel,
  onLoadMore,
  hasMore,
  isLoadingMore,
  isLoading = false,
  columns = 4,
  className = "",
  renderTileCaption,
  render3DCard = false,
}) => {
  const { t, i18n } = useI18n();
  const sentinelRef = useRef<HTMLDivElement>(null);

  const groupEntries = useMemo(
    () => browseGroups.filter((group) => group.items.length > 0),
    [browseGroups],
  );

  const totalAssetCount = useMemo(
    () => groupEntries.reduce((count, group) => count + group.items.length, 0),
    [groupEntries],
  );

  const flatAssetIds = useMemo(
    () => groupEntries.flatMap((group) => group.items.map((item) => item.id)),
    [groupEntries],
  );

  const selection = useOptionalKeyboardSelection(flatAssetIds);

  const handleAssetClick = useCallback(
    (
      item: BrowseItem,
      asset: Asset,
      event: React.MouseEvent | React.KeyboardEvent,
    ) => {
      if (!asset.asset_id) return;
      if (selection.enabled) {
        selection.handleClick(item.id, event as any);
        return;
      }
      openCarousel(asset.asset_id);
    },
    [openCarousel, selection],
  );

  const { supportsIntersectionObserver } = useGalleryInfiniteScroll({
    sentinelRef,
    hasMore: Boolean(hasMore),
    isLoadingMore: Boolean(isLoadingMore),
    isLoading: Boolean(isLoading),
    onLoadMore,
    totalAssetCount,
  });

  if (!isLoading && totalAssetCount === 0) {
    return <EmptyState className={className} />;
  }

  return (
    <div
      className={`w-full p-4 pb-8 transition-all ${className}`}
      aria-busy={isLoading || isLoadingMore}
      tabIndex={selection.enabled ? 0 : -1}
      onKeyDown={selection.enabled ? selection.handleKeyDown : undefined}
    >
      {groupEntries.map((group) => {
        const groupKey = group.key;
        const items = group.items;
        const showHeader =
          groupEntries.length > 1 || !DEFAULT_GROUP_KEYS.has(groupKey);
        const groupLabel = formatAssetGroupLabel(
          groupKey,
          t,
          i18n.resolvedLanguage || i18n.language,
        );

        return (
          <section key={groupKey} className="mb-10 last:mb-0">
            {showHeader && (
              <div className="mb-3 flex items-center justify-between text-xs uppercase tracking-widest text-base-content/60">
                <span className="font-semibold">{groupLabel}</span>
                <span>
                  {t("assets.justifiedGallery.item_count", {
                    count: items.length,
                  })}
                </span>
              </div>
            )}

            <div
              className="grid grid-cols-2 gap-0.5 md:[grid-template-columns:repeat(var(--square-gallery-columns),minmax(0,1fr))]"
              style={
                {
                  "--square-gallery-columns": String(columns),
                } as React.CSSProperties
              }
            >
              {items.map((item, index) => {
                const asset = getBrowseItemAsset(item);
                const assetId = asset.asset_id;
                const thumbnailUrl = assetId
                  ? assetUrls.getThumbnailUrl(assetId, "medium")
                  : undefined;
                const caption = renderTileCaption?.(asset, index, groupKey);

                return (
                  <SquareGalleryItem
                    key={`${groupKey}-${item.id}`}
                    item={item}
                    asset={asset}
                    thumbnailUrl={thumbnailUrl}
                    render3DCard={render3DCard}
                    caption={caption}
                    isSelected={selection.isSelected(item.id)}
                    isSelectionMode={selection.enabled}
                    onItemClick={handleAssetClick}
                  />
                );
              })}
            </div>
          </section>
        );
      })}

      {hasMore && supportsIntersectionObserver && (
        <div ref={sentinelRef} className="h-10 w-full" />
      )}

      {hasMore && (isLoadingMore || !supportsIntersectionObserver) && (
        <div
          className="flex items-center justify-center py-4 text-xs text-base-content/60"
          aria-live="polite"
        >
          {isLoadingMore ? (
            <>
              <span className="loading loading-spinner loading-sm mr-2"></span>
              {t("assets.justifiedGallery.loading_more")}
            </>
          ) : (
            !supportsIntersectionObserver && (
              <button
                className="btn btn-sm btn-outline"
                onClick={onLoadMore}
                type="button"
              >
                {t("assets.justifiedGallery.loading_more")}
              </button>
            )
          )}
        </div>
      )}
    </div>
  );
};

export default SquareGallery;
