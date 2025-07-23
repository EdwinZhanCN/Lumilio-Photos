import PhotosThumbnail from "../PhotosThumbnail/PhotosThumbnail";
import { useState, useEffect } from "react";

interface PhotosMasonryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string) => void;
  isLoading?: boolean;
}

// Utility function to reorder photos for natural reading order in masonry layout
const reorderForReadingOrder = (photos: Asset[], columns: number): Asset[] => {
  if (photos.length === 0) return photos;

  const reordered: Asset[] = Array.from({ length: photos.length });
  const itemsPerColumn = Math.ceil(photos.length / columns);

  // Convert from reading order (row-by-row) to column order for CSS columns
  for (let i = 0; i < photos.length; i++) {
    const row = Math.floor(i / columns);
    const col = i % columns;
    const newIndex = col * itemsPerColumn + row;

    if (newIndex < photos.length) {
      reordered[newIndex] = photos[i];
    }
  }

  return reordered.filter(Boolean); // Remove undefined entries
};

// Get current column count based on viewport
const getColumnCount = (): number => {
  if (typeof window === "undefined") return 3; // SSR fallback

  const width = window.innerWidth;
  if (width >= 1280) return 5; // xl
  if (width >= 1024) return 4; // lg
  if (width >= 640) return 3; // sm
  return 2; // default
};

const PhotosMasonry = ({
  groupedPhotos,
  openCarousel,
  isLoading = false,
}: PhotosMasonryProps) => {
  const [columnCount, setColumnCount] = useState(getColumnCount());

  // Update column count on resize
  useEffect(() => {
    const handleResize = () => {
      setColumnCount(getColumnCount());
    };

    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);
  if (isLoading) {
    return (
      <div className="space-y-6">
        {[1, 2].map((groupIndex) => (
          <div key={groupIndex} className="my-6">
            <div className="skeleton h-6 w-32 mb-4"></div>
            <div className="columns-2 sm:columns-3 lg:columns-4 xl:columns-5 gap-2">
              {Array.from({ length: 8 }).map((_, index) => (
                <div
                  key={index}
                  className="break-inside-avoid mb-4 overflow-hidden rounded-lg shadow-md"
                >
                  <div
                    className="skeleton w-full"
                    style={{
                      height: `${Math.floor(Math.random() * 200) + 150}px`,
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

  return (
    <>
      {Object.keys(groupedPhotos).map((groupKey) => (
        <div key={groupKey} className="my-6">
          <div className="flex items-center justify-between mb-2">
            <h2 className="text-xl font-bold text-left">{groupKey}</h2>
            <span className="text-sm text-gray-500">
              {groupedPhotos[groupKey].length} item
              {groupedPhotos[groupKey].length !== 1 ? "s" : ""}
            </span>
          </div>

          <div className="columns-2 sm:columns-3 lg:columns-4 xl:columns-5 gap-2">
            {reorderForReadingOrder(groupedPhotos[groupKey], columnCount).map(
              (asset, index) => (
                <PhotosThumbnail
                  key={asset.asset_id || `asset-${index}`}
                  asset={asset}
                  openCarousel={openCarousel}
                />
              ),
            )}
          </div>
        </div>
      ))}
    </>
  );
};

export default PhotosMasonry;
