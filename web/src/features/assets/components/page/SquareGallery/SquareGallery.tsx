import React, { memo, useCallback, useEffect, useMemo, useRef, useState } from "react";
import MediaThumbnail from "@/features/assets/components/shared/MediaThumbnail";
import StackedThumbnail from "@/features/assets/components/shared/StackedThumbnail";
import { useOptionalKeyboardSelection } from "@/features/assets/hooks/useSelection";
import { assetUrls } from "@/lib/assets/assetUrls";
import { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { AssetGalleryProps } from "../gallery.types";
import { DEFAULT_GROUP_KEYS, formatAssetGroupLabel } from "@/features/assets/utils/assetGroups";
import EmptyState from "@/components/EmptyState";
import { getBrowseItemAsset } from "@/features/assets/utils/browseItems";
import type { BrowseItem } from "@/features/assets/types/assets.type";
import { useGalleryInfiniteScroll } from "@/features/assets/hooks/useGalleryInfiniteScroll";
import {
  intersectsGalleryWindow,
  useGalleryViewportWindow,
} from "@/features/assets/hooks/useGalleryViewportWindow";

interface SquareGalleryProps extends AssetGalleryProps {
  renderTileCaption?: (asset: Asset, index: number, groupKey: string) => React.ReactNode;
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
 * Only viewport-windowed rows reach this component, so leaving the overscan
 * region releases thumbnail DOM and browser media resources.
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
    const assetId = asset.asset_id;
    const stackInfo = asset.stack;

    return (
      <div
        className={render3DCard ? "hover-3d relative aspect-square" : "relative aspect-square"}
        role="listitem"
        data-asset-id={assetId}
      >
        <>
          <figure className="h-full w-full rounded-[0.25rem]">
            {stackInfo && stackInfo.stack_size && stackInfo.stack_size > 1 ? (
              <StackedThumbnail
                asset={asset}
                thumbnailUrl={thumbnailUrl}
                stackInfo={stackInfo}
                browseStack={item.type === "stack" ? item : undefined}
                onClick={(event) => onItemClick(item, asset, event)}
                isSelected={isSelected}
                isSelectionMode={isSelectionMode}
                className="rounded-[0.25rem] focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/70"
              />
            ) : (
              <MediaThumbnail
                asset={asset}
                thumbnailUrl={thumbnailUrl}
                onClick={(event) => onItemClick(item, asset, event)}
                isSelected={isSelected}
                isSelectionMode={isSelectionMode}
                className="rounded-[0.25rem] focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/70"
              />
            )}
            {caption && (
              <div className="pointer-events-none absolute inset-x-0 bottom-0 z-30 rounded-b-[0.25rem] bg-gradient-to-t from-black/70 via-black/20 to-transparent px-4 pb-3 pt-10 text-sm text-white">
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
      </div>
    );
  },
);
SquareGalleryItem.displayName = "SquareGalleryItem";

const GRID_GAP_PX = 2;

type VirtualSquareGridProps = {
  groupKey: string;
  items: BrowseItem[];
  columns: number;
  render3DCard: boolean;
  renderTileCaption?: (asset: Asset, index: number, groupKey: string) => React.ReactNode;
  selection: ReturnType<typeof useOptionalKeyboardSelection>;
  onItemClick: SquareGalleryItemProps["onItemClick"];
};

function VirtualSquareGrid({
  groupKey,
  items,
  columns,
  render3DCard,
  renderTileCaption,
  selection,
  onItemClick,
}: VirtualSquareGridProps) {
  const gridRef = useRef<HTMLDivElement>(null);
  const [containerWidth, setContainerWidth] = useState(0);
  const [isDesktop, setIsDesktop] = useState(() =>
    typeof window === "undefined" ? true : window.matchMedia("(min-width: 768px)").matches,
  );
  const columnCount = isDesktop ? Math.max(1, columns) : 2;
  const cellSize =
    containerWidth > 0
      ? Math.max(1, (containerWidth - GRID_GAP_PX * (columnCount - 1)) / columnCount)
      : 200;
  const rowCount = Math.ceil(items.length / columnCount);
  const contentHeight = Math.max(0, rowCount * cellSize + Math.max(0, rowCount - 1) * GRID_GAP_PX);
  const viewportWindow = useGalleryViewportWindow(gridRef, contentHeight);

  useEffect(() => {
    const element = gridRef.current;
    if (!element) return;
    const media = window.matchMedia("(min-width: 768px)");
    const update = () => {
      setContainerWidth(element.getBoundingClientRect().width || element.clientWidth);
      setIsDesktop(media.matches);
    };
    update();
    const observer = typeof ResizeObserver === "undefined" ? null : new ResizeObserver(update);
    observer?.observe(element);
    media.addEventListener("change", update);
    window.addEventListener("resize", update, { passive: true });
    return () => {
      observer?.disconnect();
      media.removeEventListener("change", update);
      window.removeEventListener("resize", update);
    };
  }, []);

  return (
    <div
      ref={gridRef}
      className="relative w-full"
      role="list"
      style={{ height: contentHeight }}
      data-gallery-total={items.length}
    >
      {items.map((item, index) => {
        const row = Math.floor(index / columnCount);
        const column = index % columnCount;
        const top = row * (cellSize + GRID_GAP_PX);
        if (!intersectsGalleryWindow(top, cellSize, viewportWindow)) return null;
        const asset = getBrowseItemAsset(item);
        const assetId = asset.asset_id;
        const thumbnailUrl = assetId ? assetUrls.getThumbnailUrl(assetId, "medium") : undefined;

        return (
          <div
            key={`${groupKey}-${item.id}`}
            className="absolute"
            style={{
              top,
              left: column * (cellSize + GRID_GAP_PX),
              width: cellSize,
              height: cellSize,
            }}
          >
            <SquareGalleryItem
              item={item}
              asset={asset}
              thumbnailUrl={thumbnailUrl}
              render3DCard={render3DCard}
              caption={renderTileCaption?.(asset, index, groupKey)}
              isSelected={selection.isSelected(item.id)}
              isSelectionMode={selection.enabled}
              onItemClick={onItemClick}
            />
          </div>
        );
      })}
    </div>
  );
}

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
  emptyStateTitle,
  emptyStateDescription,
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
    (item: BrowseItem, asset: Asset, event: React.MouseEvent | React.KeyboardEvent) => {
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
    return (
      <EmptyState
        className={className}
        title={emptyStateTitle}
        description={emptyStateDescription}
      />
    );
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
        const showHeader = groupEntries.length > 1 || !DEFAULT_GROUP_KEYS.has(groupKey);
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

            <VirtualSquareGrid
              groupKey={groupKey}
              items={items}
              columns={columns}
              render3DCard={render3DCard}
              renderTileCaption={renderTileCaption}
              selection={selection}
              onItemClick={handleAssetClick}
            />
          </section>
        );
      })}

      {hasMore && supportsIntersectionObserver && <div ref={sentinelRef} className="h-10 w-full" />}

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
              <button className="btn btn-sm btn-outline" onClick={onLoadMore} type="button">
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
