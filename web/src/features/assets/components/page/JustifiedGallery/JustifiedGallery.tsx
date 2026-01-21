import { useState, useEffect, useCallback, useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import PhotosLoadingSkeleton from "../LoadingSkeleton";
import { assetService } from "@/services/assetsService";
import {
  justifiedLayoutService,
  type LayoutResult,
} from "@/services/justifiedLayoutService";
import MediaThumbnail from "../../shared/MediaThumbnail";
import { Asset } from "@/services";
import { useKeyboardSelection } from "@/features/assets/hooks/useSelection";

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
  const [serviceReady, setServiceReady] = useState(justifiedLayoutService.isReady());
  const [layouts, setLayouts] = useState<Record<string, LayoutResult>>({});
  const [containerWidth, setContainerWidth] = useState(0);
  const [scrollElement, setScrollElement] = useState<HTMLElement | null>(null);
  
  const containerRef = useRef<HTMLDivElement>(null);
  const lastWidthRef = useRef(0);

  // 1. Initialize service
  useEffect(() => {
    if (!serviceReady) {
      justifiedLayoutService.initialize().then(() => setServiceReady(true));
    }
  }, [serviceReady]);

  // 2. Setup measurements once container is available
  useEffect(() => {
    const node = containerRef.current;
    if (!node) return;

    const findScrollParent = (el: HTMLElement | null): HTMLElement => {
      if (!el || el === document.body) return document.documentElement;
      const style = window.getComputedStyle(el);
      if (style.overflowY === "auto" || style.overflowY === "scroll") return el;
      return findScrollParent(el.parentElement);
    };

    const parent = findScrollParent(node);
    setScrollElement(prev => prev === parent ? prev : parent);

    const updateWidth = () => {
      const width = node.getBoundingClientRect().width;
      if (width > 0 && Math.abs(width - lastWidthRef.current) > 1) {
        lastWidthRef.current = width;
        setContainerWidth(width);
      }
    };

    updateWidth();

    const observer = new ResizeObserver(updateWidth);
    observer.observe(node);
    return () => observer.disconnect();
  }, []);

  // 3. Keyboard selection
  const allAssetIds = useMemo(() => {
    return Object.values(groupedPhotos).flatMap(assets => 
      assets.map(a => a.asset_id).filter((id): id is string => !!id)
    );
  }, [groupedPhotos]);

  const selection = useKeyboardSelection(allAssetIds);

  // 4. Layout calculation
  const layoutConfig = useMemo(() => {
    const width = containerWidth || 800;
    const horizontalPadding = width < 640 ? 16 : width < 1024 ? 24 : 32;
    return justifiedLayoutService.createResponsiveConfig(Math.max(width - horizontalPadding * 2, 300));
  }, [containerWidth]);

  useEffect(() => {
    if (Object.keys(groupedPhotos).length === 0 || containerWidth === 0) return;

    const calculate = async () => {
      const groups: Record<string, any> = {};
      Object.entries(groupedPhotos).forEach(([key, assets]) => {
        if (assets.length > 0) groups[key] = justifiedLayoutService.assetsToLayoutBoxes(assets);
      });

      const results = await justifiedLayoutService.calculateMultipleLayouts(groups, layoutConfig);
      setLayouts(results);
    };

    calculate();
  }, [groupedPhotos, layoutConfig, containerWidth]);

  // 5. Positioned assets and virtual groups
  const positionedAssetsByGroup = useMemo(() => {
    const results: Record<string, GroupLayout> = {};
    Object.entries(groupedPhotos).forEach(([key, assets]) => {
      const layout = layouts[key];
      if (!layout) return;
      results[key] = {
        positions: layout.positions.map((pos, i) => ({
          asset: assets[i], ...pos
        })).filter(p => p.asset?.asset_id),
        containerHeight: layout.containerHeight,
        containerWidth: layout.containerWidth
      };
    });
    return results;
  }, [groupedPhotos, layouts]);

  const virtualGroups = useMemo(() => {
    const groups: VirtualGroup[] = [];
    let offset = 0;
    Object.entries(groupedPhotos).forEach(([key, assets]) => {
      const layout = positionedAssetsByGroup[key];
      const height = 60 + (layout?.containerHeight || 200) + 32;
      groups.push({ key, title: key, assets, layout, totalHeight: height, offsetTop: offset });
      offset += height;
    });
    return groups;
  }, [groupedPhotos, positionedAssetsByGroup]);

  // 6. Virtualizer
  const rowVirtualizer = useVirtualizer({
    count: virtualGroups.length,
    getScrollElement: () => scrollElement,
    estimateSize: (index) => virtualGroups[index]?.totalHeight || 300,
    overscan: 2,
  });

  // 7. Infinite scroll ref callback (React 19 style)
  const loadMoreRef = useCallback((node: HTMLDivElement | null) => {
    if (!node || !hasMore || isLoadingMore) return;
    const observer = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting) onLoadMore?.();
    }, { threshold: 0.1, rootMargin: "400px" });
    observer.observe(node);
    return () => observer.disconnect();
  }, [hasMore, isLoadingMore, onLoadMore]);

  // Render logic
  const renderContent = () => {
    if (isLoading && Object.keys(groupedPhotos).length === 0) return <PhotosLoadingSkeleton />;
    if (!serviceReady) return <div className="flex justify-center py-12"><span className="loading loading-spinner"></span></div>;
    if (Object.keys(groupedPhotos).length === 0) return (
      <div className="text-center py-12 opacity-60">
        <div className="text-4xl mb-4">🔍</div>
        <p>No assets found</p>
      </div>
    );

    return (
      <div style={{ height: `${rowVirtualizer.getTotalSize()}px`, width: "100%", position: "relative" }}>
        {rowVirtualizer.getVirtualItems().map((virtualRow) => {
          const group = virtualGroups[virtualRow.index];
          if (!group) return null;
          const { layout, assets, title, key } = group;

          return (
            <div key={key} style={{ position: "absolute", top: 0, left: 0, width: "100%", transform: `translateY(${virtualRow.start}px)` }}>
              <div className="mb-8 px-4">
                <div className="mb-4 flex items-center justify-between">
                  <h2 className="text-xl font-bold">{title}</h2>
                  <span className="text-sm opacity-40">{assets.length} items</span>
                </div>

                {layout ? (
                  <div className="relative w-full" style={{ height: `${layout.containerHeight}px` }}>
                    {layout.positions.map((p: PositionedAsset) => (
                      <div
                        key={p.asset.asset_id}
                        className={`absolute overflow-hidden rounded-sm shadow-md transition-all duration-200 cursor-pointer ${selection.isSelected(p.asset.asset_id!) ? 'z-10 scale-[0.98]' : 'hover:shadow-xl hover:-translate-y-1'}`}
                        style={{ top: `${p.top}px`, left: `${p.left}px`, width: `${p.width}px`, height: `${p.height}px` }}
                        onClick={(e) => selection.enabled ? selection.handleClick(p.asset.asset_id!, e) : openCarousel(p.asset.asset_id!)}
                      >
                        <MediaThumbnail
                          asset={p.asset}
                          thumbnailUrl={assetService.getThumbnailUrl(p.asset.asset_id!, "medium")}
                          isSelected={selection.isSelected(p.asset.asset_id!)}
                          isSelectionMode={selection.enabled}
                        />
                      </div>
                    ))}
                  </div>
                ) : (
                  <div className="flex justify-center py-12 bg-base-200/20 rounded-xl border border-dashed border-base-300">
                    <span className="loading loading-dots loading-md opacity-20"></span>
                  </div>
                )}
              </div>
            </div>
          );
        })}
      </div>
    );
  };

  return (
    <div ref={containerRef} className="w-full outline-none" onKeyDown={selection.handleKeyDown} tabIndex={0}>
      {renderContent()}
      {hasMore && <div ref={loadMoreRef} className="flex justify-center py-12"><span className="loading loading-dots loading-md opacity-30"></span></div>}
    </div>
  );
};

export default JustifiedGallery;
