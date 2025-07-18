import { useEffect, useMemo } from "react";
import { useInView } from "react-intersection-observer";
import PhotosToolBar from "@/components/Photos/PhotosToolBar/PhotosToolBar";
import PhotosMasonry from "@/components/Photos/PhotosMasonry/PhotosMasonry";
import FullScreenCarousel from "@/components/Photos/FullScreen/FullScreenCarousel";
import PhotosLoadingSkeleton from "@/components/Photos/PhotosLoadingSkeleton";
import ErrorFallBack from "@/pages/ErrorFallBack";
import { useAssetsContext } from "@/contexts/FetchContext";
import { usePhotosPageState } from "@/hooks/usePhotosPageState";
import {
  groupAssets,
  getFlatAssetsFromGrouped,
  findAssetIndex,
} from "@/utils/assetGrouping";

function Photos() {
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
    selectedAssetId,
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
    updateCarouselIndex,
  } = usePhotosPageState();

  const { ref, inView } = useInView({
    threshold: 0.5,
  });

  // Sync local search with context search
  useEffect(() => {
    const timeoutId = setTimeout(() => {
      setContextSearchQuery(searchQuery);
    }, 300); // Debounce search

    return () => clearTimeout(timeoutId);
  }, [searchQuery, setContextSearchQuery]);

  // Infinite scroll effect
  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  // Group and sort assets
  const groupedPhotos = useMemo(() => {
    if (!allAssets || allAssets.length === 0) return {};
    return groupAssets(allAssets, groupBy, sortOrder);
  }, [allAssets, groupBy, sortOrder]);

  // Flat array for carousel navigation
  const flatAssets = useMemo(() => {
    return getFlatAssetsFromGrouped(groupedPhotos);
  }, [groupedPhotos]);

  // Handle carousel navigation
  const handleCarouselNavigate = (newIndex: number) => {
    updateCarouselIndex(newIndex);
    // Update selected asset based on new index
    if (flatAssets[newIndex]) {
      // This will update the URL but won't trigger a re-render loop
      // because we're not changing the carousel state
      const newAssetId = flatAssets[newIndex].assetId;
      if (newAssetId) {
        updateCarouselIndex(newIndex);
      }
    }
  };

  // Get current carousel index when asset changes
  const actualCarouselIndex = useMemo(() => {
    if (!selectedAssetId || !flatAssets.length) return 0;
    const index = findAssetIndex(flatAssets, selectedAssetId);
    return index >= 0 ? index : 0;
  }, [selectedAssetId, flatAssets]);

  // Error state
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

  // Initial loading state
  if (isFetching && !isFetchingNextPage && allAssets.length === 0) {
    return <PhotosLoadingSkeleton count={12} />;
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
        onShowExifData={(assetId) => {
          // TODO: Implement EXIF data modal/drawer
          console.log("Show EXIF data for asset:", assetId);
        }}
      />

      <PhotosMasonry
        groupedPhotos={groupedPhotos}
        openCarousel={openCarousel}
        viewMode={viewMode}
        isLoading={isFetching && allAssets.length === 0}
        selectedAssetId={selectedAssetId}
      />

      {/* Sentinel element for infinite scroll trigger */}
      <div ref={ref} className="h-10 w-full" />

      {/* Loading states */}
      {isFetchingNextPage && (
        <div className="text-center p-4">
          <span className="loading loading-dots loading-md"></span>
          <div className="text-sm text-gray-500 mt-2">
            Loading more photos...
          </div>
        </div>
      )}

      {/* End of results */}
      {!hasNextPage && allAssets.length > 0 && (
        <div className="text-center p-4 text-gray-500">
          <div className="text-sm">
            Showing all {allAssets.length} photo
            {allAssets.length !== 1 ? "s" : ""}
          </div>
        </div>
      )}

      {/* Empty state */}
      {!isFetching && allAssets.length === 0 && (
        <div className="text-center py-12">
          <div className="text-gray-400 text-lg mb-2">No photos found</div>
          <div className="text-gray-500 text-sm">
            {searchQuery
              ? `No results for "${searchQuery}". Try adjusting your search.`
              : "Upload some photos to get started!"}
          </div>
        </div>
      )}

      {/* Full Screen Carousel */}
      {isCarouselOpen && selectedAssetId && flatAssets.length > 0 && (
        <FullScreenCarousel
          photos={flatAssets}
          initialSlide={actualCarouselIndex}
          onClose={closeCarousel}
          onNavigate={handleCarouselNavigate}
        />
      )}
    </div>
  );
}

export default Photos;
