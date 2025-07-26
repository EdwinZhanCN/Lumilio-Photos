import PhotosThumbnail from "../PhotosThumbnail/PhotosThumbnail";
import { useState, useEffect, useMemo, useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";

interface PhotosMasonryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string) => void;
  isLoading?: boolean;
}

// A simple utility to chunk an array
const chunk = <T,>(arr: T[], size: number): T[][] => {
  if (!arr || arr.length === 0 || size <= 0) return [];
  const result: T[][] = [];
  for (let i = 0; i < arr.length; i += size) {
    result.push(arr.slice(i, i + size));
  }
  return result;
};

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
}: PhotosMasonryProps) => {
  const [columnCount, setColumnCount] = useState(getColumnCount());
  const parentRef = useRef<HTMLDivElement>(null);

  // Update column count on resize
  useEffect(() => {
    const handleResize = () => {
      setColumnCount(getColumnCount());
    };
    window.addEventListener("resize", handleResize);
    return () => window.removeEventListener("resize", handleResize);
  }, []);

  // 1. Flatten the data structure for the virtualizer
  //    Each item will be a header or a chunk of photos
  const flatItems = useMemo(() => {
    const items: (
      | { type: "header"; key: string; title: string; count: number }
      | { type: "photos"; key: string; photos: Asset[] }
    )[] = [];
    const photosPerChunk = columnCount * 2; // Render 4 rows of photos per chunk

    Object.keys(groupedPhotos).forEach((groupKey) => {
      // Add the header row
      items.push({
        type: "header",
        key: groupKey,
        title: groupKey,
        count: groupedPhotos[groupKey].length,
      });

      // Chunk the photos and add them as photo rows
      const photoChunks = chunk(groupedPhotos[groupKey], photosPerChunk);
      photoChunks.forEach((photoChunk, index) => {
        items.push({
          type: "photos",
          key: `${groupKey}-chunk-${index}`,
          photos: photoChunk,
        });
      });
    });
    return items;
  }, [groupedPhotos, columnCount]);

  // 2. Setup the virtualizer
  const rowVirtualizer = useVirtualizer({
    count: flatItems.length,
    getScrollElement: () => parentRef.current,
    estimateSize: (index) => (flatItems[index].type === "header" ? 48 : 800), // Estimate header height and photo chunk height
    overscan: 5,
  });

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

  // 3. Render the virtualized list
  return (
    <div ref={parentRef}>
      <div
        style={{
          height: `${rowVirtualizer.getTotalSize()}px`,
          position: "relative",
        }}
      >
        {rowVirtualizer.getVirtualItems().map((virtualItem) => {
          const item = flatItems[virtualItem.index];
          if (!item) return null;

          return (
            <div
              key={item.key}
              ref={rowVirtualizer.measureElement}
              data-index={virtualItem.index}
              style={{
                position: "absolute",
                top: 0,
                left: 0,
                width: "100%",
                transform: `translateY(${virtualItem.start}px)`,
              }}
              className="no-scrollbar"
            >
              {item.type === "header" ? (
                <div className="my-6 mx-2">
                  <div className="flex items-center justify-between mb-2">
                    <h2 className="text-xl font-bold text-left">
                      {item.title}
                    </h2>
                    <span className="text-sm text-gray-500">
                      {item.count} item{item.count !== 1 ? "s" : ""}
                    </span>
                  </div>
                </div>
              ) : (
                <div className="columns-2 sm:columns-3 lg:columns-4 xl:columns-5 gap-1 select-none px-2">
                  {reorderForReadingOrder(item.photos, columnCount).map(
                    (asset, index) => (
                      <PhotosThumbnail
                        key={asset.asset_id || `asset-${index}`}
                        asset={asset}
                        openCarousel={openCarousel}
                      />
                    ),
                  )}
                </div>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};

export default PhotosMasonry;
