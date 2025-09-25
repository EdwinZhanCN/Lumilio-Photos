import PhotosThumbnail from "../PhotosThumbnail/PhotosThumbnail";
import { useState, useEffect, useRef, useCallback } from "react";

interface PhotosMasonryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string) => void;
  isLoading?: boolean;
  onLoadMore?: () => void;
  hasMore?: boolean;
  isLoadingMore?: boolean;
}

// Utility function to reorder photos for natural reading order in masonry layout
const reorderForReadingOrder = (photos: Asset[], columns: number): Asset[] => {
  if (photos.length === 0) return photos;

  const reordered: Asset[] = Array.from({ length: photos.length });
  const itemsPerColumn = Math.ceil(photos.length / columns);

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
  if (width >= 1280) return 5;
  if (width >= 1024) return 4;
  if (width >= 640) return 3;
  return 2;
};

// The Main Virtualized Component
const PhotosMasonry = ({
  groupedPhotos,
  openCarousel,
  isLoading = false,
  onLoadMore,
  hasMore = false,
  isLoadingMore = false,
}: PhotosMasonryProps) => {
  const [columnCount, setColumnCount] = useState(getColumnCount());
  const loadMoreRef = useRef<HTMLDivElement>(null);

  // Update column count on resize
  useEffect(() => {
    const handleResize = () => {
      setColumnCount(getColumnCount());
    };
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  // Intersection observer for infinite scroll
  const handleLoadMore = useCallback(() => {
    if (hasMore && !isLoadingMore && onLoadMore) {
      onLoadMore();
    }
  }, [hasMore, isLoadingMore, onLoadMore]);

  useEffect(() => {
    const observer = new IntersectionObserver(
      (entries) => {
        const target = entries[0];
        if (target.isIntersecting) {
          handleLoadMore();
        }
      },
      {
        threshold: 0.1,
        rootMargin: "100px",
      },
    );

    const currentRef = loadMoreRef.current;
    if (currentRef && hasMore) {
      observer.observe(currentRef);
    }

    return () => {
      if (currentRef) {
        observer.unobserve(currentRef);
      }
    };
  }, [handleLoadMore, hasMore]);

  // We no longer use virtualization here — render groups directly while preserving reading order.
  // Keeping the reading order reordering utility (reorderForReadingOrder) above.

  if (isLoading) {
    // Your existing loading skeleton is fine, no changes needed
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
    // Your existing empty state is fine
    return (
      <div className="text-center py-12">
        <div className="text-gray-400 text-lg mb-2">No photos found</div>
        <div className="text-gray-500 text-sm">
          Try adjusting your filters or search query
        </div>
      </div>
    );
  }

  // 3. Render the (non-virtualized) list grouped by header while retaining reading order
  return (
    <div>
      {Object.keys(groupedPhotos).map((groupKey) => {
        const photos = groupedPhotos[groupKey] || [];
        return (
          <div key={groupKey} className="my-6">
            <div className="my-6 mx-2">
              <div className="flex items-center justify-between mb-2">
                <h2 className="text-xl font-bold text-left">{groupKey}</h2>
                <span className="text-sm text-gray-500">
                  {photos.length} item{photos.length !== 1 ? "s" : ""}
                </span>
              </div>
            </div>

            <div className="columns-2 sm:columns-3 lg:columns-4 xl:columns-5 gap-1 select-none px-2">
              {reorderForReadingOrder(photos, columnCount).map(
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
        );
      })}

      {/* Load more trigger */}
      {hasMore && (
        <div ref={loadMoreRef} className="flex justify-center py-8">
          {isLoadingMore ? (
            <div className="flex items-center space-x-2">
              <div className="animate-spin rounded-full h-6 w-6 border-b-2 border-blue-500"></div>
              <span className="text-gray-500">Loading more photos...</span>
            </div>
          ) : (
            <div className="text-gray-400">Loading more...</div>
          )}
        </div>
      )}
    </div>
  );
};

export default PhotosMasonry;
