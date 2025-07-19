import { useEffect, useMemo } from "react";
import { useInView } from "react-intersection-observer";
import { useParams, useNavigate } from "react-router-dom";
import PhotosToolBar from "@/components/Photos/PhotosToolBar/PhotosToolBar";
import PhotosMasonry from "@/components/Photos/PhotosMasonry/PhotosMasonry";
import FullScreenCarousel from "@/components/Photos/FullScreen/FullScreenCarousel/FullScreenCarousel";
import PhotosLoadingSkeleton from "@/components/Photos/PhotosLoadingSkeleton";
import ErrorFallBack from "@/pages/ErrorFallBack";
import { useAssetsContext } from "@/contexts/FetchContext";
import { usePhotosPageState } from "@/hooks/page-hooks/usePhotosPageState";
import {
  groupAssets,
  getFlatAssetsFromGrouped,
  findAssetIndex,
} from "@/utils/assetGrouping";

function Photos() {
  const { assetId } = useParams<{ assetId: string }>();
  const navigate = useNavigate();
  const {
    assets: allAssets,
    error,
    isLoading: isFetching,
    isLoadingNextPage: isFetchingNextPage,
    fetchNextPage,
    hasMore: hasNextPage,
    setSearchQuery: setContextSearchQuery,
  } = useAssetsContext();

  const {
    isCarouselOpen,
    groupBy,
    sortOrder,
    viewMode,
    searchQuery,
    openCarousel,
    closeCarousel,
    setGroupBy,
    setSortOrder,
    setViewMode,
    setSearchQuery,
  } = usePhotosPageState();

  const { ref, inView } = useInView({
    threshold: 0.5,
  });

  useEffect(() => {
    const handler = setTimeout(() => setContextSearchQuery(searchQuery), 300);
    return () => clearTimeout(handler);
  }, [searchQuery, setContextSearchQuery]);

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const groupedPhotos = useMemo(
    () => groupAssets(allAssets, groupBy, sortOrder),
    [allAssets, groupBy, sortOrder],
  );

  const flatAssets = useMemo(
    () => getFlatAssetsFromGrouped(groupedPhotos),
    [groupedPhotos],
  );

  const currentAssetIndex = useMemo(() => {
    if (!assetId || flatAssets.length === 0) return -1;
    return findAssetIndex(flatAssets, assetId);
  }, [flatAssets, assetId]);

  const handleCarouselNavigation = (newAssetId: string) => {
    navigate(`/photos/${newAssetId}`);
  };

  if (error) {
    return (
      <ErrorFallBack
        code="500"
        title="Failed to Load Photos"
        message={error}
        reset={() => window.location.reload()}
      />
    );
  }

  return (
    <div className="p-4 w-full max-w-screen-lg mx-auto">
      <PhotosToolBar
        groupBy={groupBy}
        sortOrder={sortOrder}
        viewMode={viewMode}
        searchQuery={searchQuery}
        onGroupByChange={setGroupBy}
        onSortOrderChange={setSortOrder}
        onViewModeChange={setViewMode}
        onSearchQueryChange={setSearchQuery}
        onShowExifData={() => {}}
      />

      {isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton count={12} />
      ) : (
        <PhotosMasonry
          groupedPhotos={groupedPhotos}
          openCarousel={openCarousel}
          viewMode={viewMode}
        />
      )}

      <div ref={ref} className="h-10 w-full" />

      {isFetchingNextPage && (
        <div className="text-center p-4">
          <span className="loading loading-dots loading-md"></span>
        </div>
      )}

      {!hasNextPage && allAssets.length > 0 && (
        <div className="text-center p-4 text-gray-500">
          End of results.
        </div>
      )}

      {isCarouselOpen &&
        currentAssetIndex !== -1 &&
        flatAssets.length > 0 && (
          <FullScreenCarousel
            photos={flatAssets}
            initialSlide={currentAssetIndex}
            onClose={closeCarousel}
            onNavigate={handleCarouselNavigation}
          />
        )}
    </div>
  );
}

export default Photos;
