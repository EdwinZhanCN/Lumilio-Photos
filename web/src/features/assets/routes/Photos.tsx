import { useEffect, useMemo } from "react";
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
  const { isCarouselOpen, groupBy, searchQuery } = state;

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
    if (!assetId || flatAssets.length === 0) return -1;
    return findAssetIndex(flatAssets, assetId);
  }, [flatAssets, assetId]);

  if (error) {
    throw new Error(error);
  }

  return (
    <div>
      <PhotosToolBar
        groupBy={groupBy}
        searchQuery={searchQuery}
        onGroupByChange={(v) => dispatch({ type: "SET_GROUP_BY", payload: v })}
        onSearchQueryChange={(q) =>
          dispatch({ type: "SET_SEARCH_QUERY", payload: q })
        }
        onShowExifData={() => {}}
      />

      {isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton count={12} />
      ) : (
        <PhotosMasonry
          groupedPhotos={groupedPhotos}
          openCarousel={openCarousel}
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

      {isCarouselOpen && currentAssetIndex !== -1 && flatAssets.length > 0 && (
        <FullScreenCarousel
          photos={flatAssets}
          initialSlide={currentAssetIndex}
          onClose={closeCarousel}
          onNavigate={openCarousel}
        />
      )}
    </div>
  );
}

export default Photos;
