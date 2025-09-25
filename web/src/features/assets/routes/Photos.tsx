import { useMemo, useCallback, useState } from "react";
import { useParams } from "react-router-dom";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import PhotosMasonry from "@/features/assets/components/Photos/PhotosMasonry/PhotosMasonry";
import FullScreenCarousel from "@/features/assets/components/Photos/FullScreen/FullScreenCarousel/FullScreenCarousel";
import PhotosLoadingSkeleton from "@/features/assets/components/Photos/PhotosLoadingSkeleton";
import {
  useAssetsContext,
  useAssetsNavigation,
} from "@/features/assets/hooks/useAssetsContext";
import { useCurrentTabAssets } from "@/features/assets/hooks/useAssetsView";
import { useAssetActions } from "@/features/assets/hooks/useAssetActions";
import {
  groupAssets,
  getFlatAssetsFromGrouped,
  findAssetIndex,
} from "@/lib/utils/assetGrouping.ts";
import { FilterDTO } from "@/features/assets/components/Photos/PhotosToolBar/FilterTool";

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
  } = useCurrentTabAssets({
    withGroups: true,
    groupBy,
  });

  const handleFiltersChange = useCallback((filters: FilterDTO) => {
    // Filters are now handled directly in AssetsPageHeader
    console.log("Filters changed:", filters);
  }, []);

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

  const currentAssetIndex = useMemo(() => {
    if (!assetId || flatAssets.length === 0) {
      return -1;
    }
    const index = findAssetIndex(flatAssets, assetId);
    return index;
  }, [flatAssets, assetId]);

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
        onFiltersChange={handleFiltersChange}
      />

      {isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton />
      ) : (
        <PhotosMasonry
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

      {isCarouselOpen && flatAssets.length > 0 && (
        <FullScreenCarousel
          photos={flatAssets}
          initialSlide={currentAssetIndex >= 0 ? currentAssetIndex : 0}
          onClose={() => {
            closeCarousel();
          }}
          onNavigate={(id: string) => {
            openCarousel(id);
          }}
          onAssetUpdate={handleAssetUpdate}
          onAssetDelete={handleAssetDelete}
        />
      )}
    </div>
  );
}

export default Photos;
