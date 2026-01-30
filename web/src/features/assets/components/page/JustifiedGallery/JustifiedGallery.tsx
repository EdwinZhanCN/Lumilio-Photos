import { useState, useEffect, useMemo, useRef, useCallback } from "react";
import { Virtuoso } from "react-virtuoso";
import PhotosLoadingSkeleton from "../LoadingSkeleton";
import { assetUrls } from "@/lib/assets/assetUrls";
import {
  assetsToLayoutBoxes,
  createResponsiveConfig,
  type LayoutResult,
} from "@/lib/layout/justifiedLayout";
import MediaThumbnail from "../../shared/MediaThumbnail";
import { Asset } from "@/lib/assets/types";
import { useKeyboardSelection } from "@/features/assets/hooks/useSelection";
import { useI18n } from "@/lib/i18n";
import { useJustifiedLayoutService } from "@/hooks/util-hooks/useJustifiedLayoutService.ts";

// --- 辅助类型定义 ---

// 定义列表中的项类型：可能是组标题，也可能是一行图片
type GalleryItem =
  | { type: 'header'; title: string; count: number; date: string }
  | { type: 'row'; assets: Asset[]; layoutProps: Array<{ width: number; height: number; x: number }>; rowHeight: number; groupId: string };

// --- 核心组件 ---

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
  const {
    isReady: isLayoutReady,
    error: layoutError,
    calculateMultipleLayouts,
  } = useJustifiedLayoutService();

  const [containerWidth, setContainerWidth] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const virtuosoRef = useRef<any>(null);

  // 1. 测量容器宽度
  useEffect(() => {
    const node = containerRef.current;
    if (!node) return;

    const resizeObserver = new ResizeObserver((entries) => {
      for (const entry of entries) {
        // 使用 contentRect 获取精确宽度
        const width = entry.contentRect.width;
        if (width > 0 && Math.abs(width - containerWidth) > 5) {
          // 加一点阈值防止抖动
          setContainerWidth(width);
        }
      }
    });

    resizeObserver.observe(node);
    return () => resizeObserver.disconnect();
  }, [containerWidth]);

  // 2. 准备布局配置
  const layoutConfig = useMemo(() => {
    // 默认宽度或测量宽度，减去左右 padding (px-4 = 16px * 2 = 32px)
    // 注意：Virtuoso 的容器如果有 padding，这里要扣除
    const availableWidth = Math.max(containerWidth - 32, 300);
    return createResponsiveConfig(availableWidth);
  }, [containerWidth]);

  // 3. 计算布局 (核心逻辑：把分组数据转换成 Virtuoso 可用的扁平列表)
  // 我们不再使用 useJustifiedLayouts Hook 那个复杂的缓存逻辑，
  // 而是直接在这里根据 groupedPhotos 计算扁平化数据。
  // 为了性能，这部分计算应该被 memoize。

  const [flatItems, setFlatItems] = useState<GalleryItem[]>([]);
  const [, setLayoutsCache] = useState<Record<string, LayoutResult>>({});

  useEffect(() => {
    if (!isLayoutReady || Object.keys(groupedPhotos).length === 0) return;

    let isMounted = true;

    const computeLayouts = async () => {
      // 找出哪些组还没计算布局 (或者容器宽度变了需要全部重算)
      // 简单起见，这里演示全部重算。生产环境可以做更细粒度的 Diff。
      // 但因为 justified-layout 很快，且我们依赖 worker，全算通常也可接受。

      const groupsToProcess: Record<string, any[]> = {};
      Object.entries(groupedPhotos).forEach(([key, assets]) => {
        groupsToProcess[key] = assetsToLayoutBoxes(assets);
      });

      try {
        // 使用你的 Worker Service 计算
        const results = await calculateMultipleLayouts(groupsToProcess, layoutConfig);

        if (!isMounted) return;

        // 将布局结果转换为扁平列表
        const newItems: GalleryItem[] = [];

        Object.entries(groupedPhotos).forEach(([groupKey, assets]) => {
          const layout = results[groupKey];
          if (!layout) return;

          // 添加标题
          newItems.push({
            type: 'header',
            title: groupKey,
            count: assets.length,
            date: groupKey // 这里假设 groupKey 就是日期或标题
          });

          // 处理图片行
          // justified-layout 返回的是 boxes，我们需要把它们按行聚合
          // 下面是一个简化的行聚合算法
          let currentRow: { assets: Asset[]; props: any[] } = { assets: [], props: [] };
          let currentTop = -1;

          layout.positions.forEach((pos, index) => {
            const asset = assets[index];

            // 如果 top 变了，说明换行了 (注意浮点数比较)
            if (currentTop !== -1 && Math.abs(pos.top - currentTop) > 1) {
              // 提交上一行
              newItems.push({
                type: 'row',
                assets: currentRow.assets,
                layoutProps: currentRow.props,
                rowHeight: currentRow.props[0].height, // 假设一行高度一致
                groupId: groupKey
              });
              currentRow = { assets: [], props: [] };
            }

            currentTop = pos.top;
            currentRow.assets.push(asset);
            currentRow.props.push({ width: pos.width, height: pos.height, x: pos.left });
          });

          // 提交最后一行
          if (currentRow.assets.length > 0) {
            newItems.push({
              type: 'row',
              assets: currentRow.assets,
              layoutProps: currentRow.props,
              rowHeight: currentRow.props[0]?.height || 200,
              groupId: groupKey
            });
          }
        });

        setFlatItems(newItems);
        setLayoutsCache(results); // 缓存结果以备他用

      } catch (err) {
        console.error("Layout calculation failed", err);
      }
    };

    computeLayouts();

    return () => { isMounted = false; };
  }, [groupedPhotos, containerWidth, isLayoutReady, calculateMultipleLayouts, layoutConfig]);


  // 4. 键盘选择支持
  const allAssetIds = useMemo(() => {
    return Object.values(groupedPhotos).flatMap(assets =>
      assets.map(a => a.asset_id).filter((id): id is string => !!id)
    );
  }, [groupedPhotos]);

  const selection = useKeyboardSelection(allAssetIds);

  // --- 渲染逻辑 ---

  // 渲染单行内容的组件 (Memoized 以避免不必要的重渲染)
  const RowContent = useCallback(({ item }: { item: Extract<GalleryItem, { type: 'row' }> }) => {
    return (
      <div
        className="relative w-full flex"
        style={{ height: item.rowHeight, marginBottom: 8 }} // 行间距
      >
        {item.assets.map((asset, idx) => {
          const props = item.layoutProps[idx];
          if (!asset.asset_id) return null;
          const isSelected = selection.isSelected(asset.asset_id);

          return (
            <div
              key={asset.asset_id}
              className={`absolute overflow-hidden rounded-sm shadow-sm transition-transform duration-200 cursor-pointer
                 ${isSelected ? 'z-10 scale-[0.96] ring-2 ring-primary' : 'hover:brightness-110'}
              `}
              style={{
                left: props.x,
                width: props.width,
                height: props.height,
                // 我们使用 absolute 定位在行容器内
              }}
              onClick={(e) => selection.enabled ? selection.handleClick(asset.asset_id!, e) : openCarousel(asset.asset_id!)}
            >
              {/* SmartThumbnail 应该是一个 React.memo 组件 */}
              <MediaThumbnail
                asset={asset}
                thumbnailUrl={assetUrls.getThumbnailUrl(asset.asset_id, "medium")}
                isSelected={isSelected}
                isSelectionMode={selection.enabled}
              />
            </div>
          );
        })}
      </div>
    );
  }, [selection, openCarousel]);


  // Virtuoso 的 Footer (加载更多)
  const Footer = useCallback(() => {
    return (
      <div className="h-24 flex justify-center items-center py-4">
        {isLoadingMore ? (
          <div className="flex flex-col items-center gap-2 opacity-50">
            <span className="loading loading-spinner loading-md"></span>
            <span className="text-xs uppercase tracking-wider">{t("assets.justifiedGallery.loading_more")}</span>
          </div>
        ) : hasMore ? (
          <div className="h-4" /> // 占位符
        ) : (
          <div className="text-xs opacity-30 uppercase tracking-widest">{t("assets.justifiedGallery.end_of_results")}</div>
        )}
      </div>
    );
  }, [isLoadingMore, hasMore, t]);

  // --- 状态检查 ---
  const hasItems = flatItems.length > 0;
  const hasSourceData = Object.keys(groupedPhotos).length > 0;

  // Initial loading (passed from parent)
  if (isLoading && !hasSourceData) return <PhotosLoadingSkeleton />;

  // Layout service not ready
  if (!isLayoutReady && !layoutError && !hasItems) return <div className="p-12 text-center"><span className="loading loading-spinner" /></div>;

  // Layout error
  if (layoutError) return <div className="p-12 text-center text-error">{t("assets.justifiedGallery.layout_error")}</div>;

  // No assets found (only if source data is truly empty)
  if (!isLoading && !hasSourceData) return (
    <div className="text-center py-24 opacity-40">
      <div className="text-6xl mb-4">📸</div>
      <p className="text-xl font-medium">{t("assets.justifiedGallery.no_assets_found")}</p>
    </div>
  );

  // Calculating layout (source data exists but layout not ready yet)
  if (hasSourceData && !hasItems) {
    return <PhotosLoadingSkeleton />;
  }

  return (
    <div
      ref={containerRef}
      className="w-full h-full min-h-[500px] outline-none"
      onKeyDown={selection.handleKeyDown}
      tabIndex={0}
      style={{ paddingLeft: 16, paddingRight: 16 }} // 容器 padding
    >
      <Virtuoso
        ref={virtuosoRef}
        useWindowScroll // 使用窗口滚动而不是容器内滚动
        data={flatItems}
        endReached={onLoadMore}
        overscan={500} // 预渲染像素，减少白屏
        increaseViewportBy={200}
        itemContent={(_index, item) => {
          if (item.type === 'header') {
            return (
              <div className="pt-8 pb-4 flex items-baseline justify-between sticky top-14 z-20 bg-base-100/95 backdrop-blur-sm">
                <h2 className="text-xl font-bold tracking-tight text-base-content">{item.title}</h2>
                <span className="text-xs font-bold uppercase tracking-widest opacity-30">
                  {t("assets.justifiedGallery.item_count", { count: item.count })}
                </span>
              </div>
            );
          }

          return <RowContent item={item} />;
        }}
        components={{
          Footer: Footer
        }}
      />
    </div>
  );
};

export default JustifiedGallery;
