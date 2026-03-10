import React, { useCallback, useEffect, useMemo, useRef } from "react";
import MediaThumbnail from "@/features/assets/components/shared/MediaThumbnail";
import { useKeyboardSelection } from "@/features/assets";
import { assetUrls } from "@/lib/assets/assetUrls";
import { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { AssetGalleryProps } from "../gallery.types";
import {
  DEFAULT_GROUP_KEYS,
  formatAssetGroupLabel,
} from "@/features/assets/utils/assetGroups";
import EmptyState from "@/components/EmptyState";

interface SquareGalleryProps extends AssetGalleryProps {
  renderTileCaption?: (
    asset: Asset,
    index: number,
    groupKey: string,
  ) => React.ReactNode;
}

const getScrollParent = (element: HTMLElement | null): HTMLElement | null => {
  if (!element || typeof window === "undefined") return null;
  let current = element.parentElement;
  while (current) {
    const style = window.getComputedStyle(current);
    if (/(auto|scroll|overlay)/.test(style.overflowY)) {
      return current;
    }
    current = current.parentElement;
  }
  return null;
};

const SquareGallery: React.FC<SquareGalleryProps> = ({
  groups,
  openCarousel,
  onLoadMore,
  hasMore,
  isLoadingMore,
  isLoading = false,
  columns = 4,
  className = "",
  renderTileCaption,
}) => {
  const { t, i18n } = useI18n();
  const sentinelRef = useRef<HTMLDivElement>(null);

  const groupEntries = useMemo(
    () => groups.filter((group) => group.assets && group.assets.length > 0),
    [groups],
  );

  const totalAssetCount = useMemo(
    () =>
      groupEntries.reduce((count, group) => count + group.assets.length, 0),
    [groupEntries],
  );

  const flatAssetIds = useMemo(
    () =>
      groupEntries
        .flatMap((group) => group.assets.map((asset) => asset.asset_id))
        .filter((id): id is string => Boolean(id)),
    [groupEntries],
  );

  const selection = useKeyboardSelection(flatAssetIds);

  const handleAssetClick = useCallback(
    (asset: Asset, event: React.MouseEvent | React.KeyboardEvent) => {
      if (!asset.asset_id) return;
      if (selection.enabled) {
        selection.handleClick(asset.asset_id, event as any);
        return;
      }
      openCarousel(asset.asset_id);
    },
    [openCarousel, selection],
  );

  const supportsIntersectionObserver = useMemo(() => {
    if (typeof window === "undefined") return false;
    return "IntersectionObserver" in window;
  }, []);

  const onLoadMoreRef = useRef(onLoadMore);
  const hasMoreRef = useRef(hasMore);
  const isLoadingMoreRef = useRef(isLoadingMore);
  const isLoadingRef = useRef(isLoading);

  useEffect(() => {
    onLoadMoreRef.current = onLoadMore;
  }, [onLoadMore]);

  useEffect(() => {
    hasMoreRef.current = hasMore;
  }, [hasMore]);

  useEffect(() => {
    isLoadingMoreRef.current = isLoadingMore;
  }, [isLoadingMore]);

  useEffect(() => {
    isLoadingRef.current = isLoading;
  }, [isLoading]);

  useEffect(() => {
    if (!supportsIntersectionObserver) return;
    const sentinel = sentinelRef.current;
    if (!sentinel) return;

    const root = getScrollParent(sentinel);
    let lastLoad = 0;

    const observer = new IntersectionObserver(
      (entries) => {
        const entry = entries[0];
        if (!entry || !entry.isIntersecting) return;
        const now = Date.now();
        if (now - lastLoad < 400) return;
        if (!hasMoreRef.current) return;
        if (isLoadingRef.current || isLoadingMoreRef.current) return;
        lastLoad = now;
        onLoadMoreRef.current();
      },
      {
        root,
        rootMargin: "600px 0px",
        threshold: 0.01,
      },
    );

    observer.observe(sentinel);
    return () => observer.disconnect();
  }, [supportsIntersectionObserver]);

  if (!isLoading && totalAssetCount === 0) {
    return <EmptyState className={className} />;
  }

  return (
    <div
      className={`w-full px-4 pb-8 transition-all ${className}`}
      aria-busy={isLoading || isLoadingMore}
      tabIndex={selection.enabled ? 0 : -1}
      onKeyDown={selection.enabled ? selection.handleKeyDown : undefined}
    >
      {groupEntries.map((group) => {
        const groupKey = group.key;
        const assets = group.assets;
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
                    count: assets.length,
                  })}
                </span>
              </div>
            )}

            <div
              className="grid grid-cols-2 gap-1 md:[grid-template-columns:repeat(var(--square-gallery-columns),minmax(0,1fr))]"
              style={
                {
                  "--square-gallery-columns": String(columns),
                } as React.CSSProperties
              }
            >
              {assets.map((asset, index) => {
                const assetId = asset.asset_id;
                const thumbnailUrl = assetId
                  ? assetUrls.getThumbnailUrl(assetId, "medium")
                  : undefined;
                const caption = renderTileCaption?.(asset, index, groupKey);

                return (
                  <div
                    key={`${groupKey}-${assetId || index}`}
                    className="relative aspect-square"
                    role="listitem"
                    data-asset-id={assetId}
                  >
                    <MediaThumbnail
                      asset={asset}
                      thumbnailUrl={thumbnailUrl}
                      onClick={(event) => handleAssetClick(asset, event)}
                      isSelected={
                        assetId ? selection.isSelected(assetId) : false
                      }
                      isSelectionMode={selection.enabled}
                      className="rounded-[1.25rem] focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/70"
                    />
                    {caption && (
                      <div className="pointer-events-none absolute inset-x-0 bottom-0 z-30 rounded-b-[1.25rem] bg-gradient-to-t from-black/70 via-black/20 to-transparent px-4 pb-3 pt-10 text-sm text-white">
                        {caption}
                      </div>
                    )}
                  </div>
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
