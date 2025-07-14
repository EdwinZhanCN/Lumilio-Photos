import PhotosToolBar from "@/components/Photos/PhotosToolBar/PhotosToolBar";
import PhotosMasonry from "@/components/Photos/PhotosMasonry/PhotosMasonry";
import FullScreen from "@/components/Photos/FullScreen/FullScreen";
import { useAssetsContext } from "@/contexts/FetchContext";
import { useInView } from "react-intersection-observer";
import { useEffect, useState } from "react";

function Photos() {
  const {
    assets: allAssets,
    error,
    isLoading: isFetching,
    isLoadingNextPage: isFetchingNextPage,
    fetchNextPage,
    hasMore: hasNextPage,
  } = useAssetsContext();

  const [isCarouselOpen, setIsCarouselOpen] = useState(false);

  const openCarousel = (assetId: string) => {
    // TODO: implement carousel opening
    setIsCarouselOpen(true);
    console.log(`Open carousel for asset ${assetId}`);
  };

  const { ref, inView } = useInView({
    threshold: 0.5,
  });

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  // This is a placeholder for the grouping logic
  const groupedPhotos = {
    "July 2024": allAssets,
  };

  return (
    <div className="p-4 w-full max-w-screen-lg mx-auto">
      <PhotosToolBar />

      {isFetching && !isFetchingNextPage && <p>Loading...</p>}
      {error?.message && <p>Error: {error.message}</p>}

      <PhotosMasonry
        groupedPhotos={groupedPhotos}
        openCarousel={openCarousel}
      />

      {/* Sentinel element for infinite scroll trigger */}
      <div ref={ref} className="h-10 w-full" />

      {isFetchingNextPage && (
        <div className="text-center p-4">Loading more...</div>
      )}
      {!hasNextPage && allAssets.length > 0 && (
        <div className="text-center p-4 text-gray-500">
          No more photos to load.
        </div>
      )}

      {isCarouselOpen && <FullScreen />}
    </div>
  );
}

export default Photos;
