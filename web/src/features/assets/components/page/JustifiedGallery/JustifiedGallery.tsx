import { useState, useEffect, useRef, useCallback, useMemo } from "react";
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
  const [layoutsLoading, setLayoutsLoading] = useState(false);
  const [containerWidth, setContainerWidth] = useState(0);
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);

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

  // Measure container width using ResizeObserver
  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;

    const resizeObserver = new ResizeObserver((entries) => {
      const entry = entries[0];
      if (entry) {
        const width = entry.contentRect.width;
        if (width > 0 && width !== containerWidth) {
          setContainerWidth(width);
        }
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
    };
  }, [containerWidth]);

  const layoutConfig = useMemo(() => {
    if (containerWidth === 0) {
      // Return default config while measuring
      return justifiedLayoutService.createResponsiveConfig(800);
    }

    const config =
      justifiedLayoutService.createResponsiveConfig(containerWidth);

    return config;
  }, [containerWidth]);

  // Update layouts when photos or config changes
  useEffect(() => {
    if (Object.keys(groupedPhotos).length === 0) {
      setLayouts({});
      return;
    }

    const calculateLayouts = async () => {
      setLayoutsLoading(true);
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
      } finally {
        setLayoutsLoading(false);
      }
    };

    calculateLayouts();
  }, [groupedPhotos, layoutConfig]);

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
        rootMargin: "100px",
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
          // Skip assets that don't exist or have no asset_id
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

  if (isLoading || layoutsLoading) {
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
        <div className="text-gray-400 text-lg mb-2">No photos found</div>
        <div className="text-gray-500 text-sm">
          Try adjusting your filters or search query
        </div>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="w-full">
      {Object.entries(groupedPhotos).map(([groupKey, photos]) => {
        const layout = positionedAssetsByGroup[groupKey];

        if (!layout || photos.length === 0) {
          return (
            <div key={groupKey} className="mb-8 px-4">
              <div className="mb-4">
                <div className="flex items-center justify-between mb-2">
                  <h2 className="text-xl font-bold text-left">{groupKey}</h2>
                  <span className="text-sm text-gray-500">
                    {photos.length} item{photos.length !== 1 ? "s" : ""}
                  </span>
                </div>
              </div>
              <div className="flex justify-center py-8">
                <span className="loading loading-spinner loading-md"></span>
              </div>
            </div>
          );
        }

        return (
          <div key={groupKey} className="mb-8 px-4">
            {/* Section Header */}
            <div className="mb-4">
              <div className="flex items-center justify-between mb-2">
                <h2 className="text-xl font-bold text-left">{groupKey}</h2>
                <div className="flex items-center gap-3">
                  {process.env.NODE_ENV === "development" &&
                    containerWidth > 0 && (
                      <span className="text-xs text-blue-500 font-mono">
                        {Math.floor(
                          (layoutConfig.rowWidth + layoutConfig.spacing) / 154,
                        )}
                        c ({containerWidth}px)
                      </span>
                    )}
                  <span className="text-sm text-gray-500">
                    {photos.length} item{photos.length !== 1 ? "s" : ""}
                  </span>
                </div>
              </div>
            </div>

            {/* Justified Layout Container */}
            <div
              className="relative w-full select-none"
              style={{
                height: `${layout.containerHeight}px`,
              }}
            >
              {layout.positions.map(
                (positioned: PositionedAsset, index: number) => {
                  // Defensive check: ensure asset and asset_id exist
                  if (!positioned.asset || !positioned.asset.asset_id) {
                    console.warn(
                      "Skipping asset with missing asset_id at index",
                      index,
                    );
                    return null;
                  }

                  const thumbnailUrl = assetService.getThumbnailUrl(
                    positioned.asset.asset_id,
                    "small",
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
                      onClick={() => openCarousel(positioned.asset.asset_id!)}
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
          </div>
        );
      })}

      {/* Load more trigger */}
      {hasMore && (
        <div ref={loadMoreRef} className="flex justify-center py-8">
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
