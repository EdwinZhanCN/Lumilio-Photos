import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import MediaThumbnail from "@/features/assets/components/shared/MediaThumbnail";
import { useKeyboardSelection } from "@/features/assets";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useJustifiedLayoutService } from "@/hooks/util-hooks/useJustifiedLayoutService";
import type { LayoutResult } from "@/lib/layout/justifiedLayout";
import {
  assetsToLayoutBoxes,
  createResponsiveConfig,
} from "@/lib/layout/justifiedLayout";
import { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";

interface JustifiedGalleryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string) => void;
  onLoadMore: () => void;
  hasMore: boolean;
  isLoadingMore: boolean;
  isLoading?: boolean;
  className?: string;
}

const DEFAULT_GROUP_LABELS = new Set(["All Results", "All Assets"]);

const getThumbnailSize = (width: number) => {
  if (width >= 520) return "large";
  if (width >= 260) return "medium";
  return "small";
};

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

const JustifiedGallery: React.FC<JustifiedGalleryProps> = ({
  groupedPhotos,
  openCarousel,
  onLoadMore,
  hasMore,
  isLoadingMore,
  isLoading = false,
  className = "",
}) => {
  const { t } = useI18n();
  const containerRef = useRef<HTMLDivElement>(null);
  const sentinelRef = useRef<HTMLDivElement>(null);
  const layoutRequestRef = useRef(0);
  const [containerWidth, setContainerWidth] = useState(0);
  const [layouts, setLayouts] = useState<Record<string, LayoutResult>>({});
  const [layoutError, setLayoutError] = useState<string | null>(null);

  const { calculateMultipleLayouts, error: layoutServiceError } =
    useJustifiedLayoutService();

  const groupEntries = useMemo(
    () =>
      Object.entries(groupedPhotos).filter(
        ([, assets]) => assets && assets.length > 0,
      ),
    [groupedPhotos],
  );

  const totalAssetCount = useMemo(
    () =>
      groupEntries.reduce((count, [, assets]) => count + assets.length, 0),
    [groupEntries],
  );

  const flatAssetIds = useMemo(
    () =>
      groupEntries
        .flatMap(([, assets]) => assets.map((asset) => asset.asset_id))
        .filter((id): id is string => Boolean(id)),
    [groupEntries],
  );

  const selection = useKeyboardSelection(flatAssetIds);

  const layoutInputs = useMemo(() => {
    const inputs: Record<string, ReturnType<typeof assetsToLayoutBoxes>> = {};
    groupEntries.forEach(([groupKey, assets]) => {
      inputs[groupKey] = assetsToLayoutBoxes(assets);
    });
    return inputs;
  }, [groupEntries]);

  const layoutConfig = useMemo(
    () => (containerWidth > 0 ? createResponsiveConfig(containerWidth) : null),
    [containerWidth],
  );

  useEffect(() => {
    const element = containerRef.current;
    if (!element) return;

    let rafId = 0;
    const updateWidth = (width: number) => {
      if (rafId) cancelAnimationFrame(rafId);
      rafId = requestAnimationFrame(() => {
        setContainerWidth(Math.max(1, Math.round(width)));
      });
    };

    updateWidth(element.clientWidth);

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
    if (!layoutConfig || groupEntries.length === 0) {
      setLayouts({});
      return;
    }

    layoutRequestRef.current += 1;
    const requestId = layoutRequestRef.current;
    let isCancelled = false;
    setLayoutError(null);

    calculateMultipleLayouts(layoutInputs, layoutConfig)
      .then((results) => {
        if (isCancelled || requestId !== layoutRequestRef.current) return;
        setLayouts(results);
      })
      .catch((err) => {
        if (isCancelled || requestId !== layoutRequestRef.current) return;
        const message = err instanceof Error ? err.message : "Layout failed";
        setLayoutError(message);
      });

    return () => {
      isCancelled = true;
    };
  }, [calculateMultipleLayouts, groupEntries.length, layoutConfig, layoutInputs]);

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

  const isLayoutPending =
    groupEntries.length > 0 &&
    layoutConfig !== null &&
    groupEntries.some(([groupKey]) => !layouts[groupKey]);

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
    return (
      <div className={`w-full p-8 text-center text-base-content/60 ${className}`}>
        {t("assets.justifiedGallery.no_assets_found")}
      </div>
    );
  }

  return (
    <div
      ref={containerRef}
      className={`w-full px-4 pb-8 transition-all ${className}`}
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

      {groupEntries.map(([groupKey, assets]) => {
        const layout = layouts[groupKey];
        const showHeader =
          groupEntries.length > 1 || !DEFAULT_GROUP_LABELS.has(groupKey);

        return (
          <section key={groupKey} className="mb-10 last:mb-0">
            {showHeader && (
              <div className="mb-3 flex items-center justify-between text-xs uppercase tracking-widest text-base-content/60">
                <span className="font-semibold">{groupKey}</span>
                <span>
                  {t("assets.justifiedGallery.item_count", {
                    count: assets.length,
                  })}
                </span>
              </div>
            )}

            {layout ? (
              <div
                className="relative w-full"
                role="list"
                style={{ height: layout.containerHeight }}
              >
                {assets.map((asset, index) => {
                  const position = layout.positions[index];
                  if (!position) return null;
                  const width = Math.max(1, position.width);
                  const height = Math.max(1, position.height);
                  const assetId = asset.asset_id;
                  const thumbnailUrl = assetId
                    ? assetUrls.getThumbnailUrl(
                        assetId,
                        getThumbnailSize(width),
                      )
                    : undefined;

                  return (
                    <div
                      key={`${groupKey}-${assetId || index}`}
                      className="absolute"
                      role="listitem"
                      style={{
                        top: position.top,
                        left: position.left,
                        width,
                        height,
                      }}
                      data-asset-id={assetId}
                    >
                      <MediaThumbnail
                        asset={asset}
                        thumbnailUrl={thumbnailUrl}
                        onClick={(event) => handleAssetClick(asset, event)}
                        isSelected={assetId ? selection.isSelected(assetId) : false}
                        isSelectionMode={selection.enabled}
                        className="rounded-lg shadow-sm focus-visible:outline focus-visible:outline-2 focus-visible:outline-primary/70"
                      />
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="flex h-16 items-center justify-center text-base-content/50">
                <span className="loading loading-spinner loading-sm"></span>
              </div>
            )}
          </section>
        );
      })}

      <div ref={sentinelRef} className="h-10 w-full" />

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

export default JustifiedGallery;