import { useEffect, useMemo, useCallback } from "react";
import { useInView } from "react-intersection-observer";
import { useParams } from "react-router-dom";
import PhotosToolBar from "@/features/assets/components/Photos/PhotosToolBar/PhotosToolBar";
import PhotosMasonry from "@/features/assets/components/Photos/PhotosMasonry/PhotosMasonry";
import FullScreenCarousel from "@/features/assets/components/Photos/FullScreen/FullScreenCarousel/FullScreenCarousel";
import PhotosLoadingSkeleton from "@/features/assets/components/Photos/PhotosLoadingSkeleton";
import { useAssetsContext } from "../hooks/useAssetsContext";
import {
  useAssetsPageContext,
  useAssetsPageNavigation,
} from "@/features/assets";
import {
  groupAssets,
  getFlatAssetsFromGrouped,
  findAssetIndex,
} from "@/lib/utils/assetGrouping.ts";
import { FilterDTO } from "@/features/assets/components/Photos/PhotosToolBar/FilterTool";

function Photos() {
  const { assetId } = useParams<{ assetId: string }>();
  const {
    assets: allAssets,
    error,
    isLoading: isFetching,
    isLoadingNextPage: isFetchingNextPage,
    fetchNextPage,
    hasMore: hasNextPage,
  } = useAssetsContext();

  const { state, dispatch } = useAssetsPageContext();
  const { openCarousel, closeCarousel } = useAssetsPageNavigation();
  const { isCarouselOpen, groupBy } = state;

  const handleFiltersChange = useCallback((filters: FilterDTO) => {
    // Filters are handled in PhotosToolBar and passed to AssetsContext
    console.log("Filters changed:", filters);
  }, []);

  const { ref, inView } = useInView({
    threshold: 0.5,
  });

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const groupedPhotos = useMemo(
    () => groupAssets(allAssets, groupBy),
    [allAssets, groupBy],
  );

  const flatAssets = useMemo(
    () => getFlatAssetsFromGrouped(groupedPhotos),
    [groupedPhotos],
  );

  const currentAssetIndex = useMemo(() => {
    if (!assetId || flatAssets.length === 0) {
      console.log("currentAssetIndex: -1 (no assetId or empty flatAssets)");
      return -1;
    }
    const index = findAssetIndex(flatAssets, assetId);
    console.log("currentAssetIndex:", index, "for assetId:", assetId);
    return index;
  }, [flatAssets, assetId]);

  // Debug logging
  console.log("Photos Debug:", {
    assetId,
    isCarouselOpen,
    allAssetsCount: allAssets.length,
    flatAssetsCount: flatAssets.length,
    currentAssetIndex,
    groupBy,
  });

  if (error) {
    throw new Error(error);
  }

  return (
    <div>
      <PhotosToolBar
        groupBy={groupBy}
        onGroupByChange={(v) => dispatch({ type: "SET_GROUP_BY", payload: v })}
        onShowExifData={() => {}}
        onFiltersChange={handleFiltersChange}
      />

      {isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton />
      ) : (
        <PhotosMasonry
          groupedPhotos={groupedPhotos}
          openCarousel={(id: string) => {
            console.log("PhotosMasonry openCarousel clicked with id:", id);
            openCarousel(id);
          }}
        />
      )}

      <div ref={ref} className="h-10 w-full" />

      {isFetchingNextPage && (
        <div className="text-center p-4">
          <span className="loading loading-dots loading-md"></span>
        </div>
      )}

      {!hasNextPage && allAssets.length > 0 && (
        <div className="text-center p-4 text-gray-500">End of results.</div>
      )}

      {isCarouselOpen && flatAssets.length > 0 && (
        <FullScreenCarousel
          photos={flatAssets}
          initialSlide={currentAssetIndex >= 0 ? currentAssetIndex : 0}
          onClose={() => {
            console.log("FullScreenCarousel onClose called");
            closeCarousel();
          }}
          onNavigate={(id: string) => {
            console.log("FullScreenCarousel onNavigate called with id:", id);
            openCarousel(id);
          }}
        />
      )}
    </div>
  );
}

export default Photos;
