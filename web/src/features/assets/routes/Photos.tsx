import { useMemo, useCallback, useState, useEffect } from "react";
import { useParams } from "react-router-dom";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import {
  useAssetsContext,
  useAssetsNavigation,
} from "@/features/assets/hooks/useAssetsContext";
import { useCurrentTabAssets } from "@/features/assets/hooks/useAssetsView";
import { useAssetActions } from "@/features/assets/hooks/useAssetActions";
import { selectView } from "@/features/assets/reducers/views.reducer";
import {
  groupAssets,
  getFlatAssetsFromGrouped,
  findAssetIndex,
} from "@/lib/utils/assetGrouping.ts";
import { Asset } from "@/lib/http-commons";

function Photos() {
  const { assetId } = useParams<{ assetId: string }>();
  const { state, dispatch } = useAssetsContext();
  const { openCarousel, closeCarousel } = useAssetsNavigation();
  const { deleteAsset } = useAssetActions();

  // Local state to track asset updates
  const [updatedAssets, setUpdatedAssets] = useState<Map<string, Asset>>(
    new Map(),
  );

  const { groupBy, isCarouselOpen } = state.ui;

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

  const viewState = selectView(state.views, viewKey);
  const hasFetchedOnce = (viewState?.lastFetchAt ?? 0) > 0;

  const handleLoadMore = useCallback(() => {
    if (hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [hasNextPage, isFetchingNextPage, fetchNextPage]);

  // Use grouped assets if available, otherwise group manually
  const finalGroupedPhotos = useMemo(() => {
    if (groupedAssets && Object.keys(groupedAssets).length > 0) {
      return groupedAssets;
    }
    return groupAssets(allAssets, groupBy);
  }, [groupedAssets, allAssets, groupBy]);

  const flatAssets = useMemo(() => {
    const baseAssets = getFlatAssetsFromGrouped(finalGroupedPhotos);
    // Apply any local updates to assets
    return baseAssets.map((asset) => {
      const updated = asset.asset_id
        ? updatedAssets.get(asset.asset_id)
        : undefined;
      return updated || asset;
    });
  }, [finalGroupedPhotos, updatedAssets]);

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
      deleteAsset(deletedAssetId).catch((error) => {
        console.error("Failed to delete asset:", error);
      });
    },
    [deleteAsset],
  );

  return (
    <div>
      <AssetsPageHeader
        groupBy={groupBy}
        onGroupByChange={(v) => dispatch({ type: "SET_GROUP_BY", payload: v })}
        onFiltersChange={() => {}}
      />

      {isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton />
      ) : (
        <JustifiedGallery
          groupedPhotos={finalGroupedPhotos}
          openCarousel={(id: string) => {
            openCarousel(id);
          }}
          onLoadMore={handleLoadMore}
          hasMore={hasNextPage}
          isLoadingMore={isFetchingNextPage}
        />
      )}

      {!hasNextPage && allAssets.length > 0 && (
        <div className="text-center p-4 text-gray-500">End of results.</div>
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
                  <p className="text-lg font-medium mb-2">Locating asset...</p>
                  {hasNextPage && !isFetching && !isFetchingNextPage ? (
                    <p className="text-sm text-gray-300">
                      Loading more data to find the asset...
                    </p>
                  ) : (
                    <p className="text-sm text-gray-300">
                      Asset may not be available in the current view.
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
              <p>Loading assets...</p>
            </div>
          </div>
        ))}
    </div>
  );
}

export default Photos;
