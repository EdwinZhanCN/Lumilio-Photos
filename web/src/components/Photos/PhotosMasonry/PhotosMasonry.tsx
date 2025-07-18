import PhotosThumbnail from "../PhotosThumbnail/PhotosThumbnail";
import { ViewModeType } from "@/hooks/usePhotosPageState";

interface PhotosMasonryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string, index?: number) => void;
  viewMode: ViewModeType;
  isLoading?: boolean;
  selectedAssetId?: string | null;
}

const PhotosMasonry = ({
  groupedPhotos,
  openCarousel,
  viewMode,
  isLoading = false,
  selectedAssetId,
}: PhotosMasonryProps) => {
  if (isLoading) {
    return (
      <div className="space-y-6">
        {[1, 2].map((groupIndex) => (
          <div key={groupIndex} className="my-6">
            <div className="skeleton h-6 w-32 mb-4"></div>
            <div className={getViewModeClasses(viewMode)}>
              {Array.from({ length: 8 }).map((_, index) => (
                <div
                  key={index}
                  className="break-inside-avoid mb-4 overflow-hidden rounded-lg shadow-md"
                >
                  <div
                    className="skeleton w-full"
                    style={{
                      height:
                        viewMode === "grid"
                          ? "200px"
                          : `${Math.floor(Math.random() * 200) + 150}px`,
                    }}
                  ></div>
                </div>
              ))}
            </div>
          </div>
        ))}
      </div>
    );
  }

  if (Object.keys(groupedPhotos).length === 0) {
    return (
      <div className="text-center py-12">
        <div className="text-gray-400 text-lg mb-2">No photos found</div>
        <div className="text-gray-500 text-sm">
          Try adjusting your filters or search query
        </div>
      </div>
    );
  }

  // Calculate flat index for carousel navigation
  const getFlatIndex = (groupKey: string, assetIndex: number): number => {
    let flatIndex = 0;
    const groupKeys = Object.keys(groupedPhotos);

    for (const key of groupKeys) {
      if (key === groupKey) {
        return flatIndex + assetIndex;
      }
      flatIndex += groupedPhotos[key].length;
    }
    return flatIndex;
  };

  return (
    <>
      {Object.keys(groupedPhotos).map((groupKey) => (
        <div key={groupKey} className="my-6">
          <div className="flex items-center justify-between mb-4">
            <h2 className="text-xl font-bold text-left">{groupKey}</h2>
            <span className="text-sm text-gray-500">
              {groupedPhotos[groupKey].length} item
              {groupedPhotos[groupKey].length !== 1 ? "s" : ""}
            </span>
          </div>

          <div className={getViewModeClasses(viewMode)}>
            {groupedPhotos[groupKey].map((asset, assetIndex) => (
              <PhotosThumbnail
                key={asset.assetId}
                asset={asset}
                openCarousel={(assetId) => {
                  const flatIndex = getFlatIndex(groupKey, assetIndex);
                  openCarousel(assetId, flatIndex);
                }}
                isSelected={selectedAssetId === asset.assetId}
                viewMode={viewMode}
              />
            ))}
          </div>
        </div>
      ))}
    </>
  );
};

const getViewModeClasses = (viewMode: ViewModeType): string => {
  switch (viewMode) {
    case "grid":
      return "grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 xl:grid-cols-5 gap-4";
    case "masonry":
    default:
      return "columns-2 sm:columns-3 lg:columns-4 xl:columns-5 gap-4";
  }
};

export default PhotosMasonry;
