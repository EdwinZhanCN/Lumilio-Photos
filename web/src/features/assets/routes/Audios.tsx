import { useEffect } from "react";
import { useInView } from "react-intersection-observer";
import PhotosLoadingSkeleton from "@/features/assets/components/Photos/PhotosLoadingSkeleton";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import { useAssetsContext } from "@/features/assets/hooks/useAssetsContext";
import { useCurrentTabAssets } from "@/features/assets/hooks/useAssetsView";
import { FilterDTO } from "@/features/assets/components/Photos/PhotosToolBar/FilterTool";
import { useCallback } from "react";

function Audios() {
  const { state, dispatch } = useAssetsContext();
  const { groupBy } = state.ui;

  const { ref, inView } = useInView({
    threshold: 0.5,
  });

  // Get audio assets using the new hook
  const {
    assets: allAssets,
    groups: groupedAudios,
    isLoading: isFetching,
    isLoadingMore: isFetchingNextPage,
    fetchMore: fetchNextPage,
    hasMore: hasNextPage,
    error,
  } = useCurrentTabAssets({
    withGroups: true,
    groupBy,
  });

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  const handleFiltersChange = useCallback((filters: FilterDTO) => {
    // Filters are handled in AssetsPageHeader
    console.log("Audio filters changed:", filters);
  }, []);

  if (error) {
    throw new Error(`Failed to load audio files: ${error}`);
  }

  return (
    <div>
      <AssetsPageHeader
        groupBy={groupBy}
        onGroupByChange={(v) => dispatch({ type: "SET_GROUP_BY", payload: v })}
        onFiltersChange={handleFiltersChange}
      />

      <div className="p-4 w-full max-w-screen-lg mx-auto">
        <div className="text-center py-12">
          <div className="text-gray-400 text-lg mb-2">Audio Files</div>
          <div className="text-gray-500 text-sm">
            Audio functionality is not yet implemented
          </div>
          <div className="text-gray-500 text-xs mt-2">
            Found {allAssets.length} audio assets
          </div>
        </div>

        {isFetching && allAssets.length === 0 ? (
          <PhotosLoadingSkeleton />
        ) : (
          <div className="space-y-4">
            {Object.keys(groupedAudios || {}).map((groupKey) => (
              <div key={groupKey} className="border rounded-lg p-4">
                <h3 className="font-semibold mb-2">{groupKey}</h3>
                <div className="text-sm text-gray-600">
                  {(groupedAudios?.[groupKey] || []).length} audio file(s)
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
    </div>
  );
}

export default Audios;
