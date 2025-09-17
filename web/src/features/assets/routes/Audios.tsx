import { useEffect, useMemo } from "react";
import { useInView } from "react-intersection-observer";
import PhotosLoadingSkeleton from "../components/Photos/PhotosLoadingSkeleton";
import { useAssetsContext } from "../hooks/useAssetsContext";
import { useAssetsPageContext } from "@/features/assets";
import { groupAssets } from "@/lib/utils/assetGrouping.ts";

function Audios() {
  const {
    assets: allAssets,
    error,
    isLoading: isFetching,
    isLoadingNextPage: isFetchingNextPage,
    fetchNextPage,
    hasMore: hasNextPage,
  } = useAssetsContext();

  const { state } = useAssetsPageContext();
  const { groupBy } = state;

  const { ref, inView } = useInView({
    threshold: 0.5,
  });

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  // Filter only audio assets
  const audioAssets = useMemo(
    () =>
      allAssets.filter((asset: Asset) =>
        (asset.mime_type || "").startsWith("audio/"),
      ),
    [allAssets],
  );

  const groupedAudios = useMemo(
    () => groupAssets(audioAssets, groupBy),
    [audioAssets, groupBy],
  );

  if (error) {
    throw new Error(`Failed to load audio files: ${error}`);
  }

  return (
    <div className="p-4 w-full max-w-screen-lg mx-auto">
      <div className="text-center py-12">
        <div className="text-gray-400 text-lg mb-2">Audio Files</div>
        <div className="text-gray-500 text-sm">
          Audio functionality is not yet implemented
        </div>
        <div className="text-gray-500 text-xs mt-2">
          Found {audioAssets.length} audio assets
        </div>
      </div>

      {isFetching && allAssets.length === 0 ? (
        <PhotosLoadingSkeleton />
      ) : (
        <div className="space-y-4">
          {Object.keys(groupedAudios).map((groupKey) => (
            <div key={groupKey} className="border rounded-lg p-4">
              <h3 className="font-semibold mb-2">{groupKey}</h3>
              <div className="text-sm text-gray-600">
                {groupedAudios[groupKey].length} audio file(s)
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

export default Audios;
