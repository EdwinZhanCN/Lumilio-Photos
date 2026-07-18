import React, { useCallback, useEffect, memo, useMemo, useRef, useState } from "react";
import MediaThumbnail from "../../media/MediaThumbnail";
import StackedThumbnail from "../../media/StackedThumbnail";
import { useOptionalKeyboardSelection } from "../../../hooks/useSelection";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useJustifiedLayoutService } from "./useJustifiedLayoutService";
import type { LayoutResult } from "@/lib/layout/justifiedLayout";
import { assetsToLayoutBoxes, createResponsiveConfig } from "@/lib/layout/justifiedLayout";
import { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { AssetGalleryProps } from "../gallery.types";
import { DEFAULT_GROUP_KEYS, formatAssetGroupLabel } from "../../../utils/assetGroups";
import EmptyState from "@/components/ui/EmptyState";
import type { BrowseGroup, BrowseItem } from "../../../types";
import { getBrowseItemAsset } from "../../../utils/browseItems";

import { useGalleryInfiniteScroll } from "../../../hooks/useGalleryInfiniteScroll";
import {
  intersectsGalleryWindow,
  useGalleryViewportWindow,
} from "../../../hooks/useGalleryViewportWindow";

interface AbsoluteGalleryItemProps {
  top: number;
  left: number;
  width: number;
  height: number;
  dataAssetId?: string;
  allowOverflow?: boolean;
  children: React.ReactNode;
}

/**
 * Shell element for each justified-layout tile.
 * Only viewport-windowed items reach this component. The parent layout keeps
 * the complete scroll height while offscreen tiles leave the DOM entirely.
 */
const AbsoluteGalleryItem = memo(
  ({
    top,
    left,
    width,
    height,
    dataAssetId,
    allowOverflow = false,
    children,
  }: AbsoluteGalleryItemProps) => {
    const visibilityStyle = allowOverflow
      ? {}
      : {
          contentVisibility: "auto",
          containIntrinsicSize: `${width}px ${height}px`,
        };

    return (
      <div
        className="absolute"
        role="listitem"
        style={
          {
            top,
            left,
            width,
            height,
            ...visibilityStyle,
          } as React.CSSProperties
        }
        data-asset-id={dataAssetId}
      >
        {children}
      </div>
    );
  },
);
AbsoluteGalleryItem.displayName = "AbsoluteGalleryItem";

type VirtualizedJustifiedGroupProps = {
  groupKey: string;
  items: BrowseItem[];
  layout: LayoutResult;
  selection: ReturnType<typeof useOptionalKeyboardSelection>;
  onItemClick: (
    item: BrowseItem,
    asset: Asset,
    event: React.MouseEvent | React.KeyboardEvent,
  ) => void;
};

function VirtualizedJustifiedGroup({
  groupKey,
  items,
  layout,
  selection,
  onItemClick,
}: VirtualizedJustifiedGroupProps) {
  const layoutRef = useRef<HTMLDivElement>(null);
  const viewportWindow = useGalleryViewportWindow(layoutRef, layout.containerHeight);

  return (
    <div
      ref={layoutRef}
      className="relative w-full"
      role="list"
      style={{ height: layout.containerHeight }}
      data-gallery-total={items.length}
    >
      {items.map((item, index) => {
        const position = layout.positions[index];
        if (!position || !intersectsGalleryWindow(position.top, position.height, viewportWindow)) {
          return null;
        }
        const width = Math.max(1, position.width);
        const height = Math.max(1, position.height);
        const asset = getBrowseItemAsset(item);
        const assetId = asset.asset_id;
        const stackInfo = asset.stack;
        const hasStackOverlay = Boolean(stackInfo?.stack_size) && (stackInfo?.stack_size ?? 0) > 1;
        const thumbnailUrl = assetId
          ? assetUrls.getThumbnailUrl(assetId, getThumbnailSize(width))
          : undefined;

        return (
          <AbsoluteGalleryItem
            key={`${groupKey}-${item.id}`}
            top={position.top}
            left={position.left}
            width={width}
            height={height}
            dataAssetId={assetId}
            allowOverflow={hasStackOverlay}
          >
            {stackInfo && stackInfo.stack_size && stackInfo.stack_size > 1 ? (
              <StackedThumbnail
                asset={asset}
                thumbnailUrl={thumbnailUrl}
                stackInfo={stackInfo}
                browseStack={item.type === "stack" ? item : undefined}
                onClick={(event) => onItemClick(item, asset, event)}
                isSelected={selection.isSelected(item.id)}
                isSelectionMode={selection.enabled}
                className="rounded-[0.25rem] focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/70"
              />
            ) : (
              <MediaThumbnail
                asset={asset}
                thumbnailUrl={thumbnailUrl}
                onClick={(event) => onItemClick(item, asset, event)}
                isSelected={selection.isSelected(item.id)}
                isSelectionMode={selection.enabled}
                className="rounded-[0.25rem] focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/70"
              />
            )}
          </AbsoluteGalleryItem>
        );
      })}
    </div>
  );
}

type LayoutState = {
  signature: string;
  layouts: Record<string, LayoutResult>;
  groups: BrowseGroup[];
};

const getThumbnailSize = (width: number) => {
  if (width >= 520) return "large";
  if (width >= 260) return "medium";
  return "small";
};

const readContainerWidth = (element: HTMLElement | null): number => {
  if (!element) return 0;
  const computedStyle = window.getComputedStyle(element);
  const paddingLeft = Number.parseFloat(computedStyle.paddingLeft) || 0;
  const paddingRight = Number.parseFloat(computedStyle.paddingRight) || 0;
  const rectWidth = element.getBoundingClientRect().width;
  const width = rectWidth || element.clientWidth;
  const contentWidth = width - paddingLeft - paddingRight;
  return Math.max(0, Math.round(contentWidth));
};

const JustifiedGallery: React.FC<AssetGalleryProps> = ({
  browseGroups,
  openCarousel,
  onLoadMore,
  hasMore,
  isLoadingMore,
  isLoading = false,
  className = "",
  emptyStateTitle,
  emptyStateDescription,
}) => {
  const { t, i18n } = useI18n();
  const containerRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const layoutRequestRef = useRef(0);
  const [containerWidth, setContainerWidth] = useState(0);
  const [layoutState, setLayoutState] = useState<LayoutState>({
    signature: "",
    layouts: {},
    groups: [],
  });
  const [layoutError, setLayoutError] = useState<string | null>(null);

  const { calculateMultipleLayouts, error: layoutServiceError } = useJustifiedLayoutService();

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

  const layoutInputs = useMemo(() => {
    const inputs: Record<string, ReturnType<typeof assetsToLayoutBoxes>> = {};
    groupEntries.forEach((group) => {
      inputs[group.key] = assetsToLayoutBoxes(group.items.map(getBrowseItemAsset));
    });
    return inputs;
  }, [groupEntries]);

  const layoutConfig = useMemo(
    () => (containerWidth > 0 ? createResponsiveConfig(containerWidth) : null),
    [containerWidth],
  );

  const layoutSignature = useMemo(() => {
    if (!layoutConfig) return "";

    return JSON.stringify({
      config: layoutConfig,
      groups: groupEntries.map((group) => ({
        key: group.key,
        items: group.items.map((item, index) => ({
          id: item.id,
          box: layoutInputs[group.key]?.[index],
        })),
      })),
    });
  }, [groupEntries, layoutConfig, layoutInputs]);

  const hasCurrentLayouts = layoutState.signature === layoutSignature;
  const hasCompleteCurrentLayout =
    hasCurrentLayouts && groupEntries.every((group) => Boolean(layoutState.layouts[group.key]));
  const displayGroupEntries =
    hasCurrentLayouts || layoutState.groups.length === 0 ? groupEntries : layoutState.groups;
  const layouts = layoutState.layouts;

  useEffect(() => {
    const element = containerRef.current;
    if (!element) return;

    let rafId = 0;
    const updateWidth = (width: number) => {
      const nextWidth = Math.round(width);
      if (nextWidth <= 0) return;
      if (rafId) cancelAnimationFrame(rafId);
      rafId = requestAnimationFrame(() => {
        setContainerWidth(nextWidth);
      });
    };

    updateWidth(readContainerWidth(element));

    if (typeof ResizeObserver === "undefined") {
      const handleResize = () => updateWidth(element.clientWidth);
      window.addEventListener("resize", handleResize);
      return () => {
        window.removeEventListener("resize", handleResize);
        if (rafId) cancelAnimationFrame(rafId);
      };
    }

    const observer = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) updateWidth(entry.contentRect.width);
    });

    observer.observe(element);
    return () => {
      observer.disconnect();
      if (rafId) cancelAnimationFrame(rafId);
    };
  }, []);

  useEffect(() => {
    const measuredWidth = readContainerWidth(containerRef.current);
    if (measuredWidth > 0 && measuredWidth !== containerWidth) {
      setContainerWidth(measuredWidth);
      return;
    }

    if (!layoutConfig || groupEntries.length === 0) {
      setLayoutState((current) => {
        if (isLoading && current.groups.length > 0) return current;
        if (current.signature === "" && current.groups.length === 0) {
          return current;
        }
        return { signature: "", layouts: {}, groups: [] };
      });
      return;
    }

    if (hasCompleteCurrentLayout) {
      return;
    }

    layoutRequestRef.current += 1;
    const requestId = layoutRequestRef.current;
    let isCancelled = false;
    setLayoutError(null);

    calculateMultipleLayouts(layoutInputs, layoutConfig)
      .then((results) => {
        if (isCancelled || requestId !== layoutRequestRef.current) return;
        setLayoutState({
          signature: layoutSignature,
          layouts: results,
          groups: groupEntries,
        });
      })
      .catch((err) => {
        if (isCancelled || requestId !== layoutRequestRef.current) return;
        const message = err instanceof Error ? err.message : "Layout failed";
        setLayoutError(message);
      });

    return () => {
      isCancelled = true;
    };
  }, [
    calculateMultipleLayouts,
    containerWidth,
    groupEntries.length,
    groupEntries,
    hasCompleteCurrentLayout,
    isLoading,
    layoutConfig,
    layoutInputs,
    layoutSignature,
  ]);

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

  const isLayoutPending =
    displayGroupEntries.length > 0 &&
    layoutConfig !== null &&
    displayGroupEntries.some((group) => !layouts[group.key]);

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
      ref={containerRef}
      className={`w-full p-4 pb-8 transition-all ${className}`}
      aria-busy={isLoading || isLoadingMore}
      tabIndex={selection.enabled ? 0 : -1}
      onKeyDown={selection.enabled ? selection.handleKeyDown : undefined}
    >
      {(layoutError || layoutServiceError) && (
        <div className="mb-4 rounded-lg border border-error/30 bg-error/10 px-4 py-3 text-sm text-error">
          {layoutError || layoutServiceError}
        </div>
      )}

      {isLayoutPending && (
        <div className="flex items-center justify-center py-4 text-base-content/50">
          <span className="loading loading-spinner loading-sm"></span>
        </div>
      )}

      {displayGroupEntries.map((group) => {
        const groupKey = group.key;
        const items = group.items;
        const layout = layouts[groupKey];
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

            {layout ? (
              <VirtualizedJustifiedGroup
                groupKey={groupKey}
                items={items}
                layout={layout}
                selection={selection}
                onItemClick={handleAssetClick}
              />
            ) : (
              <div className="flex h-16 items-center justify-center text-base-content/50">
                <span className="loading loading-spinner loading-sm"></span>
              </div>
            )}
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

export default JustifiedGallery;
