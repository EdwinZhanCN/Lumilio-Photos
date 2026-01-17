import { useState, useEffect, useRef, useCallback, useMemo } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import PhotosLoadingSkeleton from "../LoadingSkeleton";
import { assetService } from "@/services/assetsService";
import {
  justifiedLayoutService,
  type LayoutResult,
} from "@/services/justifiedLayoutService";
import MediaThumbnail from "../../shared/MediaThumbnail";
import { Asset } from "@/services";

interface JustifiedGalleryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string) => void;
  isLoading?: boolean;
  onLoadMore?: () => void;
  hasMore?: boolean;
  isLoadingMore?: boolean;
}

interface PositionedAsset {
  asset: Asset;
  top: number;
  left: number;
  width: number;
  height: number;
}

interface GroupLayout {
  positions: PositionedAsset[];
  containerHeight: number;
  containerWidth: number;
}

interface VirtualGroup {
  key: string;
  title: string;
  assets: Asset[];
  layout?: GroupLayout;
  totalHeight: number;
  offsetTop: number;
}

const JustifiedGallery = ({
  groupedPhotos,
  openCarousel,
  isLoading = false,
  onLoadMore,
  hasMore = false,
  isLoadingMore = false,
}: JustifiedGalleryProps) => {
  const [serviceReady, setServiceReady] = useState(
    justifiedLayoutService.isReady(),
  );
  const [layouts, setLayouts] = useState<Record<string, LayoutResult>>({});
  const [containerWidth, setContainerWidth] = useState(0);
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const scrollElementRef = useRef<HTMLElement | null>(null);
  const resizeTimeoutRef = useRef<NodeJS.Timeout | null>(null);

  // Initialize service on mount
  useEffect(() => {
    if (!serviceReady) {
      justifiedLayoutService
        .initialize()
        .then(() => setServiceReady(true))
        .catch((error) => {
          console.error(
            "Failed to initialize justified layout service:",
            error,
          );
          setServiceReady(true); // Continue with fallback layouts
        });
    }
  }, [serviceReady]);

  // Get scroll container (main content area or window)
  useEffect(() => {
    // Find the scrollable parent
    const findScrollParent = (element: HTMLElement | null): HTMLElement => {
      if (!element || element === document.body) {
        return document.documentElement;
      }

      const overflowY = window.getComputedStyle(element).overflowY;
      const isScrollable = overflowY === "auto" || overflowY === "scroll";

      if (isScrollable && element.scrollHeight > element.clientHeight) {
        return element;
      }

      return findScrollParent(element.parentElement);
    };

    if (containerRef.current) {
      scrollElementRef.current = findScrollParent(containerRef.current);
    }
  }, []);

  // Measure container width using ResizeObserver with debouncing
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const updateWidth = (width: number) => {
      if (width > 0 && Math.abs(width - containerWidth) > 10) {
        // Only update if difference is significant (>10px)
        setContainerWidth(width);
      }
    };

    const resizeObserver = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) {
        const width = entry.contentRect.width;

        // Debounce resize events
        if (resizeTimeoutRef.current) {
          clearTimeout(resizeTimeoutRef.current);
        }

        resizeTimeoutRef.current = setTimeout(() => {
          updateWidth(width);
        }, 150); // 150ms debounce
      }
    });

    resizeObserver.observe(container);

    // Set initial width
    const initialWidth = container.getBoundingClientRect().width;
    if (initialWidth > 0) {
      setContainerWidth(initialWidth);
    }

    return () => {
      resizeObserver.disconnect();
      if (resizeTimeoutRef.current) {
        clearTimeout(resizeTimeoutRef.current);
      }
    };
  }, []); // Remove containerWidth from dependencies to avoid loops

  // Calculate layout config with proper margins
  const layoutConfig = useMemo(() => {
    if (containerWidth === 0) {
      return justifiedLayoutService.createResponsiveConfig(800);
    }

    // Add horizontal padding/margins (responsive)
    const horizontalPadding =
      containerWidth < 640 ? 16 : containerWidth < 1024 ? 24 : 32;
    const availableWidth = containerWidth - horizontalPadding * 2;

    const config = justifiedLayoutService.createResponsiveConfig(
      Math.max(availableWidth, 300),
    );

    return config;
  }, [containerWidth]);

  // Update layouts when photos or config changes
  useEffect(() => {
    if (Object.keys(groupedPhotos).length === 0) {
      setLayouts({});
      return;
    }

    // Skip layout calculation if container width is not ready
    if (containerWidth === 0) {
      return;
    }

    const calculateLayouts = async () => {
      try {
        const groups: Record<string, any> = {};

        // Convert grouped photos to layout boxes
        Object.entries(groupedPhotos).forEach(([groupKey, assets]) => {
          if (assets.length > 0) {
            groups[groupKey] =
              justifiedLayoutService.assetsToLayoutBoxes(assets);
          }
        });

        // Calculate all layouts
        const results = await justifiedLayoutService.calculateMultipleLayouts(
          groups,
          layoutConfig,
        );
        setLayouts(results);
      } catch (error) {
        console.error("Failed to calculate layouts:", error);

        // Fallback to grid layouts for all groups
        const fallbackLayouts: Record<string, LayoutResult> = {};
        Object.entries(groupedPhotos).forEach(([groupKey, assets]) => {
          if (assets.length > 0) {
            const boxes = justifiedLayoutService.assetsToLayoutBoxes(assets);
            fallbackLayouts[groupKey] =
              justifiedLayoutService.createFallbackLayout(boxes, layoutConfig);
          }
        });
        setLayouts(fallbackLayouts);
      }
    };

    calculateLayouts();
  }, [groupedPhotos, layoutConfig, containerWidth]);

  // Convert layout results to positioned assets
  const positionedAssetsByGroup = useMemo(() => {
    const results: Record<string, GroupLayout> = {};

    Object.entries(groupedPhotos).forEach(([groupKey, assets]) => {
      const layout = layouts[groupKey];
      if (!layout || assets.length === 0) {
        return;
      }

      const positions: PositionedAsset[] = layout.positions
        .map((pos, index) => {
          const asset = assets[index];
          if (!asset || !asset.asset_id) {
            console.warn(
              "Skipping asset with missing asset_id in group",
              groupKey,
              "at index",
              index,
            );
            return null;
          }
          return {
            asset,
            top: pos.top,
            left: pos.left,
            width: pos.width,
            height: pos.height,
          };
        })
        .filter(
          (positioned): positioned is PositionedAsset => positioned !== null,
        );

      results[groupKey] = {
        positions,
        containerHeight: layout.containerHeight,
        containerWidth: layout.containerWidth,
      };
    });

    return results;
  }, [groupedPhotos, layouts]);

  // Prepare virtual groups with accumulated heights
  const virtualGroups = useMemo(() => {
    const groups: VirtualGroup[] = [];
    let accumulatedHeight = 0;
    const headerHeight = 60; // Height for section header
    const sectionSpacing = 32; // Spacing between sections

    Object.entries(groupedPhotos).forEach(([groupKey, assets]) => {
      const layout = positionedAssetsByGroup[groupKey];
      const contentHeight = layout?.containerHeight || 200; // Fallback height
      const totalHeight = headerHeight + contentHeight + sectionSpacing;

      groups.push({
        key: groupKey,
        title: groupKey,
        assets,
        layout,
        totalHeight,
        offsetTop: accumulatedHeight,
      });

      accumulatedHeight += totalHeight;
    });

    return groups;
  }, [groupedPhotos, positionedAssetsByGroup]);

  // Setup virtualizer for groups
  const rowVirtualizer = useVirtualizer({
    count: virtualGroups.length,
    getScrollElement: () => scrollElementRef.current,
    estimateSize: (index) => virtualGroups[index]?.totalHeight || 300,
    overscan: 2, // Render 2 groups above and below viewport
    measureElement:
      typeof window !== "undefined" &&
      navigator.userAgent.indexOf("Firefox") === -1
        ? (element) => element.getBoundingClientRect().height
        : undefined,
  });

  // Intersection observer for infinite scroll
  const handleLoadMore = useCallback(() => {
    if (hasMore && !isLoadingMore && onLoadMore) {
      onLoadMore();
    }
  }, [hasMore, isLoadingMore, onLoadMore]);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        const target = entries[0];
        if (target.isIntersecting) {
          handleLoadMore();
        }
      },
      {
        threshold: 0.1,
        rootMargin: "200px", // Trigger earlier
      },
    );

    const currentRef = loadMoreRef.current;
    if (currentRef && hasMore) {
      observer.observe(currentRef);
    }

    return () => {
      if (currentRef) {
        observer.unobserve(currentRef);
      }
    };
  }, [handleLoadMore, hasMore]);

  // Calculate horizontal padding
  const horizontalPadding = useMemo(() => {
    if (containerWidth < 640) return 16;
    if (containerWidth < 1024) return 24;
    return 32;
  }, [containerWidth]);

  if (isLoading && Object.keys(groupedPhotos).length === 0) {
    return <PhotosLoadingSkeleton />;
  }

  if (!serviceReady) {
    return (
      <div className="flex justify-center items-center py-12">
        <span className="loading loading-spinner loading-lg"></span>
        <span className="ml-2 text-gray-500">Initializing gallery...</span>
      </div>
    );
  }

  if (Object.keys(groupedPhotos).length === 0) {
    return (
      <div className="text-center py-12">
        <div className="text-gray-400 text-lg mb-2">No assets found</div>
        <div className="text-gray-500 text-sm">
          Try adjusting your filters or search query
        </div>
      </div>
    );
  }

  const virtualItems = rowVirtualizer.getVirtualItems();

  return (
    <div ref={containerRef} className="w-full">
      {/* Virtual scrolling container */}
      <div
        style={{
          height: `${rowVirtualizer.getTotalSize()}px`,
          width: "100%",
          position: "relative",
        }}
      >
        {virtualItems.map((virtualRow) => {
          const group = virtualGroups[virtualRow.index];
          if (!group) return null;

          const { layout, assets, title, key } = group;

          return (
            <div
              key={key}
              data-index={virtualRow.index}
              ref={rowVirtualizer.measureElement}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                transform: `translateY(${virtualRow.start}px)`,
              }}
            >
              <div
                className="mb-8"
                style={{
                  paddingLeft: `${horizontalPadding}px`,
                  paddingRight: `${horizontalPadding}px`,
                }}
              >
                {/* Section Header */}
                <div className="mb-4">
                  <div className="flex items-center justify-between mb-2">
                    <h2 className="text-xl font-bold text-left">{title}</h2>
                    <div className="flex items-center gap-3">
                      {process.env.NODE_ENV === "development" &&
                        containerWidth > 0 && (
                          <span className="text-xs text-blue-500 font-mono">
                            {Math.floor(
                              (layoutConfig.rowWidth + layoutConfig.spacing) /
                                154,
                            )}
                            c ({containerWidth}px)
                          </span>
                        )}
                      <span className="text-sm text-gray-500">
                        {assets.length} item{assets.length !== 1 ? "s" : ""}
                      </span>
                    </div>
                  </div>
                </div>

                {/* Justified Layout Container */}
                {layout ? (
                  <div
                    className="relative w-full select-none"
                    style={{
                      height: `${layout.containerHeight}px`,
                    }}
                  >
                    {layout.positions.map(
                      (positioned: PositionedAsset, index: number) => {
                        if (!positioned.asset || !positioned.asset.asset_id) {
                          console.warn(
                            "Skipping asset with missing asset_id at index",
                            index,
                          );
                          return null;
                        }

                        const thumbnailUrl = assetService.getThumbnailUrl(
                          positioned.asset.asset_id,
                          "medium",
                        );

                        return (
                          <div
                            key={positioned.asset.asset_id}
                            className="absolute overflow-hidden rounded-sm shadow-md hover:shadow-xl transition-all duration-200 hover:-translate-y-1 animate-fade-in cursor-pointer"
                            style={{
                              top: `${positioned.top}px`,
                              left: `${positioned.left}px`,
                              width: `${positioned.width}px`,
                              height: `${positioned.height}px`,
                            }}
                            onClick={() =>
                              openCarousel(positioned.asset.asset_id!)
                            }
                          >
                            <MediaThumbnail
                              asset={positioned.asset}
                              thumbnailUrl={thumbnailUrl}
                              className="cursor-pointer"
                            />
                          </div>
                        );
                      },
                    )}
                  </div>
                ) : (
                  <div className="flex justify-center py-8">
                    <span className="loading loading-spinner loading-md"></span>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>

      {/* Load more trigger */}
      {hasMore && (
        <div
          ref={loadMoreRef}
          className="flex justify-center py-8"
          style={{
            paddingLeft: `${horizontalPadding}px`,
            paddingRight: `${horizontalPadding}px`,
          }}
        >
          {isLoadingMore ? (
            <div className="flex items-center space-x-2">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500"></div>
              <span className="text-gray-500">Loading more photos...</span>
            </div>
          ) : (
            <div className="text-gray-400">Loading more...</div>
          )}
        </div>
      )}
    </div>
  );
};

export default JustifiedGallery;
