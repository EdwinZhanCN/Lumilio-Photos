import { useState, useEffect, useMemo, useRef } from "react";
import PhotosLoadingSkeleton from "../LoadingSkeleton";
import { assetService } from "@/services/assetsService";
import {
  justifiedLayoutService,
  type LayoutResult,
} from "@/services/justifiedLayoutService";
import MediaThumbnail from "../../shared/MediaThumbnail";
import { Asset } from "@/services";
import { useKeyboardSelection } from "@/features/assets/hooks/useSelection";
import { useI18n } from "@/lib/i18n";

interface JustifiedGalleryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string) => void;
  isLoading?: boolean;
  onLoadMore?: () => void;
  hasMore?: boolean;
  isLoadingMore?: boolean;
}

const JustifiedGallery = ({
  groupedPhotos,
  openCarousel,
  isLoading = false,
  onLoadMore,
  hasMore = false,
  isLoadingMore = false,
}: JustifiedGalleryProps) => {
  const { t } = useI18n();
  const [serviceReady, setServiceReady] = useState(justifiedLayoutService.isReady());
  const [layouts, setLayouts] = useState<Record<string, LayoutResult>>({});
  const [containerWidth, setContainerWidth] = useState(0);
  
  const containerRef = useRef<HTMLDivElement>(null);
  const loadMoreRef = useRef<HTMLDivElement>(null);
  const lastWidthRef = useRef(0);

  // 1. Initialize service
  useEffect(() => {
    if (!serviceReady) {
      justifiedLayoutService.initialize().then(() => setServiceReady(true));
    }
  }, [serviceReady]);

  // 2. Measure container width
  useEffect(() => {
    const node = containerRef.current;
    if (!node) return;

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
    // Use fixed 16px padding (px-4) to align with header and ensure layout fits container
    const horizontalPadding = 16;
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

  // 5. Infinite scroll observer
  useEffect(() => {
    const node = loadMoreRef.current;
    if (!node || !hasMore || isLoadingMore) return;

    const observer = new IntersectionObserver((entries) => {
      if (entries[0].isIntersecting) {
        onLoadMore?.();
      }
    }, { threshold: 0.1, rootMargin: "800px" });

    observer.observe(node);
    return () => observer.disconnect();
  }, [hasMore, isLoadingMore, onLoadMore]);

  if (isLoading && Object.keys(groupedPhotos).length === 0) return <PhotosLoadingSkeleton />;
  if (!serviceReady) return <div className="flex justify-center py-12"><span className="loading loading-spinner"></span></div>;

  if (Object.keys(groupedPhotos).length === 0) {
    return (
      <div className="text-center py-24 opacity-40">
        <div className="text-6xl mb-4">📸</div>
        <p className="text-xl font-medium">{t("assets.justifiedGallery.no_assets_found")}</p>
      </div>
    );
  }

  return (
    <div ref={containerRef} className="w-full outline-none" onKeyDown={selection.handleKeyDown} tabIndex={0}>
      {Object.entries(groupedPhotos).map(([title, assets]) => {
        const layout = layouts[title];
        
        return (
          <div key={title} className="mb-4 px-4 animate-in fade-in duration-500">
            <div className="mb-4 flex items-center justify-between">
              <h2 className="text-xl font-bold tracking-tight">{title}</h2>
              <span className="text-xs font-bold uppercase tracking-widest opacity-30">
                {t("assets.justifiedGallery.item_count", { count: assets.length })}
              </span>
            </div>

            {layout ? (
              <div className="relative w-full" style={{ height: `${layout.containerHeight}px` }}>
                {layout.positions.map((pos, i) => {
                  const asset = assets[i];
                  if (!asset?.asset_id) return null;
                  const isSelected = selection.isSelected(asset.asset_id);

                  return (
                    <div
                      key={asset.asset_id}
                      className={`absolute overflow-hidden rounded-sm shadow-md transition-all duration-300 cursor-pointer
                        ${isSelected ? 'z-10 scale-[0.97] shadow-none' : 'hover:shadow-xl hover:-translate-y-1'}
                      `}
                      style={{ 
                        top: `${pos.top}px`,
                        left: `${pos.left}px`,
                        width: `${pos.width}px`,
                        height: `${pos.height}px`
                      }}
                      onClick={(e) => selection.enabled ? selection.handleClick(asset.asset_id!, e) : openCarousel(asset.asset_id!)}
                    >
                      <MediaThumbnail
                        asset={asset}
                        thumbnailUrl={assetService.getThumbnailUrl(asset.asset_id, "medium")}
                        isSelected={isSelected}
                        isSelectionMode={selection.enabled}
                      />
                    </div>
                  );
                })}
              </div>
            ) : (
              <div className="flex justify-center py-20 bg-base-200/10 rounded-2xl border border-dashed border-base-300/50">
                <span className="loading loading-dots loading-md opacity-20"></span>
              </div>
            )}
          </div>
        );
      })}

      {/* Load more trigger */}
      <div ref={loadMoreRef} className="h-24 flex justify-center items-center">
        {hasMore && (
          <div className="flex flex-col items-center gap-2 opacity-30">
            <span className="loading loading-ring loading-md"></span>
            <span className="text-xs font-bold uppercase tracking-widest">{t("assets.justifiedGallery.loading_more")}</span>
          </div>
        )}
      </div>
    </div>
  );
};

export default JustifiedGallery;
