import { useState, useEffect, useMemo } from "react";
import { useInView } from "react-intersection-observer";
import { useInfiniteQuery } from "@tanstack/react-query";
import FullScreenCarousel from "@/components/Photos/FullScreen/FullScreenCarousel";
import { ListAssetsParams } from "@/services/getAssetsService"; // Assuming you have this service

// --- 1. Type Definitions (based on your input) ---

// A helper type for our grouping functions
type GroupedAssets = Record<string, Asset[]>;

// The Asset types you provided
interface Asset {
  assetId?: string;
  uploadTime?: string;
  originalFilename?: string;
  fileSize?: number;
  tags?: AssetTag[];
  type?: "PHOTO" | "VIDEO" | "AUDIO" | "DOCUMENT";
  thumbnails?: AssetThumbnail[];
  description?: string; // Assuming description can be part of the asset
}

interface AssetTag {
  tagId?: number;
  tagName?: string;
}

interface AssetThumbnail {
  size?: "small" | "medium" | "large";
  storagePath?: string;
}

// --- 2. API Data Fetching Hook ---

// We create a custom hook to encapsulate the logic for fetching assets.
const useInfiniteAssets = (
  filters: Omit<ListAssetsParams, "limit" | "offset">,
) => {
  const queryKey = ["assets", "infinite", filters];

  const fetchAssets = async ({ pageParam = 0 }) => {
    const limit = 30; // Fetch 30 assets per page
    const params: ListAssetsParams = {
      ...filters,
      limit,
      offset: pageParam,
    };

    // In a real app, you would make the actual API call here.
    // const response = await AssetService.listAssets(params);
    // return response.data.data;

    // For demonstration, we simulate the API call with mock data.
    // Replace this with your actual API call.
    console.log("Fetching assets with params:", params);
    return new Promise<{ assets: Asset[]; offset: number; limit: number }>(
      (resolve) => {
        setTimeout(() => {
          const newAssets = Array.from({ length: limit }).map((_, index) => {
            const id = pageParam + index;
            return {
              assetId: `id_${id}`,
              uploadTime: new Date(Date.now() - id * 1000 * 3600 * 24)
                .toISOString()
                .split("T")[0],
              originalFilename: `photo_${id}.jpg`,
              fileSize: 100000 + Math.random() * 50000000,
              type: "PHOTO",
              thumbnails: [
                {
                  size: "medium",
                  storagePath: `https://picsum.photos/seed/${id}/400/600`,
                },
              ],
              description: `Sample photo ${id}`,
            } as Asset;
          });
          resolve({ assets: newAssets, offset: pageParam, limit });
        }, 1000); // Simulate network delay
      },
    );
  };

  return useInfiniteQuery({
    queryKey,
    queryFn: fetchAssets,
    initialPageParam: 0,
    getNextPageParam: (lastPage) => {
      if (!lastPage || lastPage.assets.length === 0) {
        return undefined; // No more pages
      }
      return lastPage.offset + lastPage.assets.length;
    },
  });
};

// --- 3. Grouping Logic (updated for the new Asset type) ---

const groupPhotosByDate = (assets: Asset[]): GroupedAssets => {
  return assets.reduce<GroupedAssets>((acc, asset) => {
    const date = asset.uploadTime
      ? new Date(asset.uploadTime).toISOString().split("T")[0]
      : "Unknown Date";
    if (!acc[date]) acc[date] = [];
    acc[date].push(asset);
    return acc;
  }, {});
};

const groupPhotosByType = (assets: Asset[]): GroupedAssets => {
  return assets.reduce<GroupedAssets>((acc, asset) => {
    const type = asset.type || "UNKNOWN";
    if (!acc[type]) acc[type] = [];
    acc[type].push(asset);
    return acc;
  }, {});
};

const groupPhotosBySizeRange = (assets: Asset[]): GroupedAssets => {
  const sizeGroups = [
    { name: ">100MB", condition: (mb: number) => mb > 100 },
    { name: "10MB - 100MB", condition: (mb: number) => mb >= 10 && mb < 100 },
    { name: "1MB - 10MB", condition: (mb: number) => mb >= 1 && mb < 10 },
    { name: "<1MB", condition: (mb: number) => mb < 1 },
  ];

  return assets.reduce<GroupedAssets>((acc, asset) => {
    const bytes = asset.fileSize;
    if (typeof bytes !== "number") return acc;
    const mb = bytes / (1024 * 1024);
    const group = sizeGroups.find((g) => g.condition(mb));
    const groupKey = group ? group.name : "Unknown Size";
    if (!acc[groupKey]) acc[groupKey] = [];
    acc[groupKey].push(asset);
    return acc;
  }, {});
};

// --- 4. Main Component (Refactored for Real Data) ---

const Photos = () => {
  const [currentGrouping, setCurrentGrouping] = useState<
    "date" | "type" | "size"
  >("date");
  const [isCarouselOpen, setIsCarouselOpen] = useState(false);
  const [currentPhotoIndex, setCurrentPhotoIndex] = useState(0);

  // Use our new data fetching hook
  const {
    data,
    error,
    fetchNextPage,
    hasNextPage,
    isFetching,
    isFetchingNextPage,
  } = useInfiniteAssets({ type: "PHOTO" }); // Example filter

  // `react-intersection-observer` hook for infinite scroll
  const { ref, inView } = useInView({ threshold: 0, rootMargin: "400px" });

  useEffect(() => {
    if (inView && hasNextPage && !isFetchingNextPage) {
      fetchNextPage();
    }
  }, [inView, hasNextPage, isFetchingNextPage, fetchNextPage]);

  // Flatten the pages of assets into a single array for rendering and grouping
  const allAssets = useMemo(
    () => data?.pages.flatMap((page) => page.assets) ?? [],
    [data],
  );

  const groupingFunctions = {
    date: groupPhotosByDate,
    type: groupPhotosByType,
    size: groupPhotosBySizeRange,
  };

  const groupedPhotos = useMemo(() => {
    return groupingFunctions[currentGrouping](allAssets);
  }, [currentGrouping, allAssets]);

  const openCarousel = (assetId: string) => {
    const currentIndex = allAssets.findIndex((a) => a.assetId === assetId);
    if (currentIndex !== -1) {
      setCurrentPhotoIndex(currentIndex);
      setIsCarouselOpen(true);
    }
  };

  // Utility to get a thumbnail URL from an asset
  const getThumbnailUrl = (asset: Asset) => {
    return (
      asset.thumbnails?.find((t) => t.size === "medium")?.storagePath ||
      "https://placehold.co/400x400/222/fff?text=No+Preview"
    );
  };

  return (
    <div className="p-4 w-full max-w-screen-lg mx-auto">
      <div className="flex gap-2 items-center mb-4">
        <h1 className="text-2xl font-bold">Photos</h1>
        {/* Dropdown for grouping */}
        <div className="dropdown">
          <div tabIndex={0} role="button" className="btn btn-ghost m-1">
            <svg
              xmlns="http://www.w3.org/2000/svg"
              fill="none"
              viewBox="0 0 24 24"
              strokeWidth={1.5}
              stroke="currentColor"
              className="size-5"
            >
              <path
                strokeLinecap="round"
                strokeLinejoin="round"
                d="M3.75 6.75h16.5M3.75 12h16.5m-16.5 5.25h16.5"
              />
            </svg>
          </div>
          <ul
            tabIndex={0}
            className="dropdown-content menu bg-base-200 rounded-box z-[1] w-52 p-2 shadow"
          >
            <li>
              <a onClick={() => setCurrentGrouping("date")}>Group by Date</a>
            </li>
            <li>
              <a onClick={() => setCurrentGrouping("size")}>Group by Size</a>
            </li>
            <li>
              <a onClick={() => setCurrentGrouping("type")}>Group by Type</a>
            </li>
          </ul>
        </div>
      </div>

      {isFetching && !isFetchingNextPage && <p>Loading...</p>}
      {error && <p>Error: {error.message}</p>}

      {Object.keys(groupedPhotos).map((groupKey) => (
        <div key={groupKey} className="my-6">
          <h2 className="text-xl font-bold mb-4 text-left">{groupKey}</h2>
          <div className="columns-2 sm:columns-3 lg:columns-4 xl:columns-5 gap-4">
            {groupedPhotos[groupKey].map((asset) => (
              <div
                key={asset.assetId}
                className="break-inside-avoid mb-4 cursor-pointer overflow-hidden rounded-lg shadow-md hover:shadow-xl transition-shadow"
                onClick={() => openCarousel(asset.assetId!)}
              >
                <img
                  src={getThumbnailUrl(asset)}
                  alt={asset.originalFilename || "Asset"}
                  className="w-full h-auto object-cover"
                  loading="lazy"
                />
              </div>
            ))}
          </div>
        </div>
      ))}

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

      {isCarouselOpen && <h1>Hello</h1>}
    </div>
  );
};

export default Photos;
