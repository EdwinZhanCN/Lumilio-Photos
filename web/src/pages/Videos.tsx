import { useEffect, useMemo, useRef } from "react";
import { useInView } from "react-intersection-observer";
import PhotosLoadingSkeleton from "@/components/Photos/PhotosLoadingSkeleton";
import ErrorFallBack from "@/pages/ErrorFallBack";
import { useAssetsContext } from "@/contexts/FetchContext";
import { useAssetsPageState } from "@/hooks/page-hooks/useAssetsPageState";
import { groupAssets } from "@/utils/assetGrouping";

function Videos() {
  const {
    assets: allAssets,
    error,
    isLoading: isFetching,
    isLoadingNextPage: isFetchingNextPage,
    fetchNextPage,
    hasMore: hasNextPage,
    setSearchQuery: setContextSearchQuery,
  } = useAssetsContext();

  const { groupBy, sortOrder, searchQuery } = useAssetsPageState();

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

  // Filter only video assets
  const videoAssets = useMemo(
    () => allAssets.filter((asset: Asset) => asset.type === "VIDEO"),
    [allAssets],
  );

  const groupedVideos = useMemo(
    () => groupAssets(videoAssets, groupBy, sortOrder),
    [videoAssets, groupBy, sortOrder],
  );

  if (error) {
    return (
      <ErrorFallBack
        code="500"
        title="Failed to Load Videos"
        message={error}
        reset={() => window.location.reload()}
      />
    );
  }

  return (
    <div className="p-4 w-full max-w-screen-lg mx-auto">
      <div className="text-center py-12">
        <div className="text-gray-400 text-lg mb-2">Videos Page</div>
        <div className="text-gray-500 text-sm">
          Video functionality is not yet implemented
        </div>
        <div className="text-gray-500 text-xs mt-2">
          Found {videoAssets.length} video assets
        </div>
      </div>

      {isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton count={12} />
      ) : (
        <div className="space-y-4">
          {Object.keys(groupedVideos).map((groupKey) => (
            <div key={groupKey} className="border rounded-lg p-4">
              <h3 className="font-semibold mb-2">{groupKey}</h3>
              <div className="text-sm text-gray-600">
                {groupedVideos[groupKey].length} video(s)
              </div>
            </div>
          ))}
        </div>
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
    </div>
  );
}

export default Videos;
