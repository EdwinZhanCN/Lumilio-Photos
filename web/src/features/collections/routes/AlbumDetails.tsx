import { useParams } from "react-router-dom";
import { useState, useEffect, useMemo, useCallback, useRef } from "react";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import { useGroupBy, useIsCarouselOpen, useUIActions } from "@/features/assets/selectors";
import { useAssetsNavigation } from "@/features/assets/hooks/useAssetsNavigation";
import { useAssetsView } from "@/features/assets/hooks/useAssetsView";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import {findAssetIndex, getFlatAssetsFromGrouped, groupAssets} from "@/lib/utils/assetGrouping";
import { WorkerProvider } from "@/contexts/WorkerProvider";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import {AssetViewDefinition, JustifiedGallery} from "@/features/assets";
import { FolderIcon } from "lucide-react";
import { $api } from "@/lib/http-commons/queryClient";
import type { Album, ApiResult } from "@/lib/albums/types";

const AlbumAssetsContent = () => {
  const { albumId, assetId } = useParams<{ albumId: string, assetId: string }>();
  const groupBy = useGroupBy();
  const isCarouselOpen = useIsCarouselOpen();
  const { setGroupBy } = useUIActions();
  const { openCarousel, closeCarousel } = useAssetsNavigation();
  const [isScrolled, setIsScrolled] = useState(false);
  const scrollContainerRef = useRef<HTMLDivElement>(null);
  const albumIdNumber = albumId ? Number(albumId) : 0;

  // Fetch album metadata
  const albumQuery = $api.useQuery(
    "get",
    "/api/v1/albums/{id}",
    {
      params: { path: { id: albumIdNumber } },
    },
    {
      enabled: !!albumId,
    },
  );
  const albumResponse = albumQuery.data as ApiResult<Album> | undefined;
  const album = albumResponse?.data;
  const isAlbumLoading = albumQuery.isLoading;

  // Memoize view definition to prevent unnecessary re-renders/fetches
  const viewDefinition: AssetViewDefinition = useMemo(() => ({
    filter: {
      album_id: parseInt(albumId!)
    },
    groupBy,
    pageSize: 50,
  }), [albumId, groupBy]);

  // Fetch album assets using the assets view hook
  const {
    assets,
    groups,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
    error,
  } = useAssetsView(viewDefinition, { withGroups: true });

  // Handle flat grouping - ensure we have a proper groups object
  const groupedPhotos = useMemo(() => {
    if (groups && Object.keys(groups).length > 0) {
      return groups;
    }
    return groupAssets(assets, groupBy);
  }, [groups, assets, groupBy]);

  // Use flat assets from grouped to ensure order consistency with gallery
  const flatAssets = useMemo(() => {
    if (groupedPhotos && Object.keys(groupedPhotos).length > 0) {
      return getFlatAssetsFromGrouped(groupedPhotos);
    }
    return assets;
  }, [groupedPhotos, assets]);

  // Calculate slide index from URL assetId
  const slideIndex = useMemo(() => {
    if (assetId && flatAssets.length > 0) {
      return findAssetIndex(flatAssets, assetId);
    }
    return -1;
  }, [assetId, flatAssets]);

  // Handle auto-fetching if asset is not in current page
  const [isLocatingAsset, setIsLocatingAsset] = useState(false);

  useEffect(() => {
    if (isCarouselOpen && assetId && flatAssets.length > 0) {
      const index = findAssetIndex(flatAssets, assetId);
      if (index < 0) {
        if (hasMore && !isLoading && !isLoadingMore) {
          setIsLocatingAsset(true);
          fetchMore();
        }
      } else {
        setIsLocatingAsset(false);
      }
    }
  }, [assetId, flatAssets, isCarouselOpen, hasMore, isLoading, isLoadingMore, fetchMore]);

  const handleLoadMore = useCallback(() => {
    if (hasMore && !isLoadingMore) {
      fetchMore();
    }
  }, [hasMore, isLoadingMore, fetchMore]);

  const handleScroll = useCallback((e: React.UIEvent<HTMLDivElement>) => {
    setIsScrolled(e.currentTarget.scrollTop > 60);
  }, []);

  if (error) return <div className="p-8 text-error">Error loading album: {error}</div>;

  // Initial loading state: loading assets and we have none yet
  const isInitialLoading = isLoading && assets.length === 0;

  return (
    <div className="flex flex-col h-full relative">
      <div className="sticky top-0 z-30 bg-base-100/80 backdrop-blur-md border-b border-base-200/30">
        <AssetsPageHeader
          groupBy={groupBy}
          onGroupByChange={setGroupBy}
          title={album?.album_name || "Album"}
          icon={<FolderIcon className="w-6 h-6 text-primary" />}
        />

        <div className={`px-4 transition-all duration-500 ease-in-out overflow-hidden ${isScrolled ? "py-1.5" : "py-4"}`}>
          {/* Title and Badge - Collapses when scrolled */}
          <div className={`flex items-baseline gap-4 transition-all duration-500 ease-in-out ${isScrolled ? "max-h-0 opacity-0 -translate-y-2" : "max-h-20 opacity-100 translate-y-0"}`}>
            {isAlbumLoading && !album ? (
              <div className="h-10 w-64 bg-base-300 animate-pulse rounded-lg"></div>
            ) : (
              <>
                <h1 className="text-4xl font-black tracking-tight text-primary">{album?.album_name || "Untitled Album"}</h1>
                <span className="badge badge-ghost font-mono text-xs opacity-50">ALBUM #{albumId}</span>
              </>
            )}
          </div>

          {/* Description - Collapses or becomes a single line preview */}
          <div className={`transition-all duration-500 ease-in-out ${isScrolled ? "max-h-0 opacity-0" : "max-h-40 opacity-100 mt-3"}`}>
            {isAlbumLoading && !album ? (
              <div className="h-4 w-full max-w-xl bg-base-300 animate-pulse rounded"></div>
            ) : (
              album?.description && <p className="text-base-content/70 max-w-3xl leading-relaxed line-clamp-2">{album.description}</p>
            )}
          </div>

          {/* Stats Row - Always visible but changes style */}
          <div className={`flex items-center gap-6 transition-all duration-500 ease-in-out ${isScrolled ? "mt-0 text-[10px] opacity-60" : "mt-6 text-xs opacity-40"}`}>
            <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
              <span className="text-primary text-[8px]">●</span>
              {isAlbumLoading && !album ? (
                <div className="h-3 w-16 bg-base-300 animate-pulse rounded"></div>
              ) : (
                <span>{album?.asset_count || 0} items</span>
              )}
            </div>
            <div className="flex items-center gap-2 font-bold uppercase tracking-widest">
              <span className="text-primary text-[8px]">●</span>
              {isAlbumLoading && !album ? (
                <div className="h-3 w-24 bg-base-300 animate-pulse rounded"></div>
              ) : (
                <span>Created {album?.created_at ? new Date(album.created_at).toLocaleDateString() : ""}</span>
              )}
            </div>

            {/* Inline description preview when scrolled */}
            <div className={`flex items-center gap-2 ml-auto max-w-[50%] transition-all duration-500 ${isScrolled ? "opacity-70 translate-x-0" : "opacity-0 translate-x-4 pointer-events-none"}`}>
              <span className="truncate italic font-normal normal-case">{album?.description}</span>
            </div>
          </div>
        </div>
      </div>

      <div
        ref={scrollContainerRef}
        className="flex-1 overflow-y-auto"
        onScroll={handleScroll}
      >
        <div>
          {isInitialLoading ? (
            <PhotosLoadingSkeleton />
          ) : (
            <JustifiedGallery
              groupedPhotos={groupedPhotos}
              openCarousel={openCarousel}
              onLoadMore={handleLoadMore}
              hasMore={hasMore}
              isLoadingMore={isLoadingMore}
            />
          )}
        </div>
      </div>

      {isCarouselOpen && flatAssets.length > 0 && (
        <>
          <FullScreenCarousel
            photos={flatAssets}
            initialSlide={slideIndex >= 0 ? slideIndex : 0}
            slideIndex={slideIndex >= 0 ? slideIndex : undefined}
            onClose={closeCarousel}
            onNavigate={openCarousel}
          />
          {isLocatingAsset && (
            <div className="fixed inset-0 bg-black/70 z-[60] flex items-center justify-center">
              <div className="text-white text-center bg-black/50 backdrop-blur-sm rounded-2xl p-8 max-w-md">
                <div className="loading loading-spinner loading-lg mb-4"></div>
                <p className="text-lg font-medium mb-2">Locating asset...</p>
                <p className="text-sm text-gray-300">Loading more data to find the asset...</p>
              </div>
            </div>
          )}
        </>
      )}
    </div>
  );
};

const AlbumDetails = () => {
  const { albumId } = useParams<{ albumId: string }>();

  return (
    <WorkerProvider preload={["exif", "export"]}>
      <AssetsProvider persist={false} basePath={`/collections/${albumId}`}>
        <AlbumAssetsContent />
      </AssetsProvider>
    </WorkerProvider>
  );
};

export default AlbumDetails;
