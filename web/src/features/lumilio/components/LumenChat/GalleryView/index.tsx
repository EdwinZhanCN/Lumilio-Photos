import { useCallback } from "react";
import JustifiedGallery from "../../../../assets/components/page/JustifiedGallery/JustifiedGallery";
import { Asset } from "@/services";

interface GalleryViewProps {
  title: string;
  count: number;
  type: string;
  onClear?: () => void;
  assets?: Asset[]; // 实际的资产数据
}

/**
 * GalleryView component that displays filtered assets from agent commands
 * Uses the assets feature's JustifiedGallery component for optimal display
 */
export function GalleryView({
  title,
  count,
  onClear,
  assets,
}: GalleryViewProps) {
  // Create grouped photos for JustifiedGallery directly from provided assets
  const groupedPhotos = useCallback(() => {
    if (!assets || assets.length === 0) return {};

    return {
      [title]: assets,
    };
  }, [assets, title]);

  // Open carousel handler
  const openCarousel = useCallback((assetId: string) => {
    console.log("Open carousel for asset:", assetId);
    // TODO: Implement carousel functionality
    // This could open a modal or navigate to a detail view
  }, []);

  if (!assets || assets.length === 0) {
    return (
      <div className="flex-1 bg-base-100 p-6 overflow-y-auto border-r border-base-300">
        <div className="animate-fade-in">
          <div className="flex justify-between items-end mb-6">
            <div>
              <h2 className="text-2xl font-bold text-base-content">{title}</h2>
              <p className="text-base-content/60 text-sm mt-1">
                Found {count} assets via Agent Command
              </p>
            </div>
            {onClear && (
              <button
                onClick={onClear}
                className="text-xs text-primary hover:underline"
              >
                Clear View
              </button>
            )}
          </div>
          <div className="text-center py-12">
            <div className="text-gray-400 text-lg mb-2">No assets found</div>
            <div className="text-gray-500 text-sm">
              Try adjusting your filter criteria
            </div>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="flex-1 bg-base-100 overflow-y-auto border-r border-base-300">
      <div className="p-6">
        <div className="animate-fade-in">
          <div className="flex justify-between items-end mb-6">
            <div>
              <h2 className="text-2xl font-bold text-base-content">{title}</h2>
              <p className="text-base-content/60 text-sm mt-1">
                Found {count} assets via Agent Command
              </p>
            </div>
            {onClear && (
              <button
                onClick={onClear}
                className="text-xs text-primary hover:underline"
              >
                Clear View
              </button>
            )}
          </div>

          {/* JustifiedGallery for displaying assets */}
          <div className="h-96">
            <JustifiedGallery
              groupedPhotos={groupedPhotos()}
              openCarousel={openCarousel}
              isLoading={false}
              onLoadMore={undefined} // No pagination for filtered results
              hasMore={false} // No pagination for filtered results
              isLoadingMore={false} // No pagination for filtered results
            />
          </div>
        </div>
      </div>
    </div>
  );
}

export default GalleryView;
