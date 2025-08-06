import { useEffect, useMemo, useRef } from "react";
import { useInView } from "react-intersection-observer";
import { useParams, useNavigate, useLocation } from "react-router-dom";
import PhotosToolBar from "@/features/assets/components/Photos/PhotosToolBar/PhotosToolBar";
import PhotosMasonry from "@/features/assets/components/Photos/PhotosMasonry/PhotosMasonry";
import FullScreenCarousel from "@/features/assets/components/Photos/FullScreen/FullScreenCarousel/FullScreenCarousel";
import PhotosLoadingSkeleton from "@/features/assets/components/Photos/PhotosLoadingSkeleton";
import { useAssetsContext } from "../hooks/useAssetsContext";
import { useAssetsPageState } from "@/features/assets/hooks/useAssetsPageState";
import {
  groupAssets,
  getFlatAssetsFromGrouped,
  findAssetIndex,
} from "@/lib/assetGrouping";

function Photos() {
  const { assetId } = useParams<{ assetId: string }>();
  const navigate = useNavigate();
  const location = useLocation();
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
    searchQuery,
    openCarousel,
    closeCarousel,
    setGroupBy,
    setSortOrder,
    setSearchQuery,
  } = useAssetsPageState();

  const { ref, inView } = useInView({
    threshold: 0.5,
  });

  const isInitialMount = useRef(true);

  useEffect(() => {
    if (isInitialMount.current) {
      isInitialMount.current = false;
      return;
    }
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
    // Determine current path base (photos, videos, or audios)
    const path = location.pathname;
    if (path.includes("/videos")) {
      navigate(`/assets/videos/${newAssetId}`);
    } else if (path.includes("/audios")) {
      navigate(`/assets/audios/${newAssetId}`);
    } else {
      // Default to photos
      navigate(`/assets/photos/${newAssetId}`);
    }
  };

  if (error) {
    throw new Error(error);
  }

  return (
    <div className="p-4 w-full mx-auto">
      <PhotosToolBar
        groupBy={groupBy}
        sortOrder={sortOrder}
        searchQuery={searchQuery}
        onGroupByChange={setGroupBy}
        onSortOrderChange={setSortOrder}
        onSearchQueryChange={setSearchQuery}
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
          onNavigate={handleCarouselNavigation}
        />
      )}
    </div>
  );
}

export default Photos;
