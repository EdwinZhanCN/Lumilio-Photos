import { useMemo, useCallback, useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import {
  useAssetsNavigation,
} from "@/features/assets/hooks/useAssetsNavigation";
import { useCurrentTabAssets } from "@/features/assets/hooks/useAssetsView";
import { useAssetActions } from "@/features/assets/hooks/useAssetActions";
import {
  useGroupBy,
  useIsCarouselOpen,
  useView,
  useUIActions
} from "@/features/assets/selectors";
import {
  groupAssets,
  getFlatAssetsFromGrouped,
  findAssetIndex,
} from "@/lib/utils/assetGrouping.ts";
import { Asset } from "@/lib/http-commons";
import { useI18n } from "@/lib/i18n";

function Audios() {
  const { assetId } = useParams<{ assetId: string }>();
  const { openCarousel, closeCarousel } = useAssetsNavigation();
  const { deleteAsset } = useAssetActions();
  const { t } = useI18n();

  // Selectors
  const groupBy = useGroupBy();
  const isCarouselOpen = useIsCarouselOpen();
  const { setGroupBy } = useUIActions();

  // Local state to track asset updates
  const [updatedAssets, setUpdatedAssets] = useState<Map<string, Asset>>(
    new Map(),
  );

  // Get assets for current view using the new hook
  const {
    assets: allAssets,
    groups: groupedAssets,
    isLoading: isFetching,
    isLoadingMore: isFetchingNextPage,
    fetchMore: fetchNextPage,
    hasMore: hasNextPage,
    error,
    viewKey,
  } = useCurrentTabAssets({
    withGroups: true,
    groupBy,
  });

  const handleLoadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  const viewState = useView(viewKey);
  const hasFetchedOnce = (viewState?.lastFetchAt ?? 0) > 0;

  // Use grouped assets if available, otherwise group manually
  const finalGroupedAudios = useMemo(() => {
    if (groupedAssets && Object.keys(groupedAssets).length > 0) {
      return groupedAssets;
    }
    return groupAssets(allAssets, groupBy);
  }, [groupedAssets, allAssets, groupBy]);

  const flatAssets = useMemo(() => {
    const baseAssets = getFlatAssetsFromGrouped(finalGroupedAudios);
    // Apply any local updates to assets
    return baseAssets.map((asset) => {
      const updated = asset.asset_id
        ? updatedAssets.get(asset.asset_id)
        : undefined;
      return updated || asset;
    });
  }, [finalGroupedAudios, updatedAssets]);

  // Track the slide index for controlled carousel navigation
  const [slideIndex, setSlideIndex] = useState<number>(-1);
  const [isLocatingAsset, setIsLocatingAsset] = useState(false);

  // Unified effect for asset location and auto-fetching
  useEffect(() => {
    if (!isCarouselOpen) return;

    // If we have a valid assetId and data is loaded
    if (assetId && hasFetchedOnce) {
      const index = findAssetIndex(flatAssets, assetId);

      if (index >= 0) {
        // Asset found in current data
        setSlideIndex(index);
        setIsLocatingAsset(false);
        return;
      }

      // Asset not found in current data
      setIsLocatingAsset(true);

      // Try to fetch more pages if available
      if (hasNextPage && !isFetching && !isFetchingNextPage) {
        fetchNextPage();
        return;
      }

      // No more pages and not loading - asset doesn't exist in current view
      if (!hasNextPage && !isFetching && !isFetchingNextPage) {
        // Give a small delay before closing to show feedback
        const timer = setTimeout(() => {
          closeCarousel();
        }, 500);
        return () => clearTimeout(timer);
      }
    }
  }, [
    assetId,
    flatAssets,
    isCarouselOpen,
    hasFetchedOnce,
    hasNextPage,
    isFetching,
    isFetchingNextPage,
    fetchNextPage,
    closeCarousel,
  ]);

  // Update slideIndex when asset is found in flatAssets
  useEffect(() => {
    if (assetId && flatAssets.length > 0) {
      const index = findAssetIndex(flatAssets, assetId);
      if (index >= 0) {
        setSlideIndex(index);
        setIsLocatingAsset(false);
      }
    }
  }, [assetId, flatAssets]);

  if (error) {
    throw new Error(error);
  }

  const handleAssetUpdate = useCallback((updatedAsset: Asset) => {
    if (updatedAsset.asset_id) {
      setUpdatedAssets(
        (prev) => new Map(prev.set(updatedAsset.asset_id!, updatedAsset)),
      );
    }
  }, []);

  const handleAssetDelete = useCallback(
    (deletedAssetId: string) => {
      // Remove from updated assets map if it exists there
      setUpdatedAssets((prev) => {
        const newMap = new Map(prev);
        newMap.delete(deletedAssetId);
        return newMap;
      });

      // Delete via the actions hook which handles the store updates
      deleteAsset(deletedAssetId).catch((error: any) => {
        console.error("Failed to delete asset:", error);
      });
    },
    [deleteAsset],
  );

  return (
    <div>
      <AssetsPageHeader
        groupBy={groupBy}
        onGroupByChange={(v) => setGroupBy(v)}
        onFiltersChange={() => { }}
      />

      {isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton />
      ) : (
        <JustifiedGallery
          groupedPhotos={finalGroupedAudios}
          openCarousel={(id: string) => {
            openCarousel(id);
          }}
          onLoadMore={handleLoadMore}
          hasMore={hasNextPage}
          isLoadingMore={isFetchingNextPage}
        />
      )}

      {!hasNextPage && allAssets.length > 0 && (
        <div className="text-center p-4 text-gray-500">{t("assets.audios.end_of_results")}</div>
      )}

      {isCarouselOpen &&
        (flatAssets.length > 0 ? (
          <>
            <FullScreenCarousel
              photos={flatAssets}
              initialSlide={slideIndex >= 0 ? slideIndex : 0}
              slideIndex={slideIndex >= 0 ? slideIndex : undefined}
              onClose={() => {
                closeCarousel();
              }}
              onNavigate={(id: string) => {
                openCarousel(id);
              }}
              onAssetUpdate={handleAssetUpdate}
              onAssetDelete={handleAssetDelete}
            />
            {isLocatingAsset && (
              <div className="fixed inset-0 bg-black/70 z-[60] flex items-center justify-center">
                <div className="text-white text-center bg-black/50 backdrop-blur-sm rounded-2xl p-8 max-w-md">
                  <div className="loading loading-spinner loading-lg mb-4"></div>
                  <p className="text-lg font-medium mb-2">{t("assets.audios.locating_asset")}</p>
                  {hasNextPage && !isFetching && !isFetchingNextPage ? (
                    <p className="text-sm text-gray-300">
                      {t("assets.audios.loading_more_data")}
                    </p>
                  ) : (
                    <p className="text-sm text-gray-300">
                      {t("assets.audios.asset_not_available")}
                    </p>
                  )}
                </div>
              </div>
            )}
          </>
        ) : (
          <div className="fixed inset-0 bg-black/90 z-50 flex items-center justify-center">
            <div className="text-white text-center">
              <div className="loading loading-spinner loading-lg mb-4"></div>
              <p>{t("assets.audios.loading_assets")}</p>
            </div>
          </div>
        ))}
    </div>
  );
}

export default Audios;
