import { useParams } from "react-router-dom";
import { useState, useEffect } from "react";
import { useQuery } from "@tanstack/react-query";
import { albumService } from "@/services/albumService";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import { useAssetsContext, useAssetsNavigation } from "@/features/assets/hooks/useAssetsContext";
import { useAssetsView } from "@/features/assets/hooks/useAssetsView";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { findAssetIndex } from "@/lib/utils/assetGrouping";
import { WorkerProvider } from "@/contexts/WorkerProvider";

const AlbumAssetsContent = () => {
  const { albumId, assetId } = useParams<{ albumId: string, assetId: string }>();
  const { state, dispatch } = useAssetsContext();
  const { openCarousel, closeCarousel } = useAssetsNavigation();

  const { groupBy, isCarouselOpen } = state.ui;

  // Fetch album metadata
  const { data: albumData } = useQuery({
    queryKey: ["album", albumId],
    queryFn: () => albumService.getAlbumById(parseInt(albumId!)),
    enabled: !!albumId
  });

  const album = albumData?.data?.data;

  // Fetch album assets using the assets view hook
  const {
    assets,
    groups,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
    error
  } = useAssetsView({
    filter: { 
      // @ts-ignore - backend now supports album_id in filter
      album_id: parseInt(albumId!) 
    },
    groupBy,
    pageSize: 50,
  }, { withGroups: true });

  const [slideIndex, setSlideIndex] = useState<number>(-1);

  useEffect(() => {
    if (isCarouselOpen && assetId && assets.length > 0) {
      const index = findAssetIndex(assets, assetId);
      if (index >= 0) setSlideIndex(index);
    }
  }, [assetId, assets, isCarouselOpen]);

  if (error) return <div className="p-8 text-error">Error loading album: {error}</div>;

  return (
    <div className="flex flex-col h-full">
      <AssetsPageHeader
        groupBy={groupBy}
        onGroupByChange={(v) => dispatch({ type: "SET_GROUP_BY", payload: v })}
      />
      
      <div className="px-8 py-6 bg-base-100/50 backdrop-blur-md border-b border-base-300">
        <div className="flex items-baseline gap-4">
          <h1 className="text-4xl font-black tracking-tight text-primary">{album?.album_name || "Loading..."}</h1>
          <span className="badge badge-ghost font-mono text-xs opacity-50">ALBUM #{albumId}</span>
        </div>
        {album?.description && <p className="mt-3 text-base-content/70 max-w-3xl leading-relaxed">{album.description}</p>}
        <div className="mt-6 flex items-center gap-6 text-xs font-bold uppercase tracking-widest opacity-40">
          <div className="flex items-center gap-2">
            <span className="text-primary">●</span>
            <span>{album?.asset_count || 0} items</span>
          </div>
          <div className="flex items-center gap-2">
            <span className="text-primary">●</span>
            <span>Created {album?.created_at ? new Date(album.created_at).toLocaleDateString() : ""}</span>
          </div>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        <JustifiedGallery
          groupedPhotos={groups || {}}
          openCarousel={openCarousel}
          onLoadMore={fetchMore}
          hasMore={hasMore}
          isLoadingMore={isLoadingMore}
          isLoading={isLoading && assets.length === 0}
        />
      </div>

      {isCarouselOpen && assets.length > 0 && (
        <FullScreenCarousel
          photos={assets}
          initialSlide={slideIndex >= 0 ? slideIndex : 0}
          onClose={closeCarousel}
          onNavigate={openCarousel}
        />
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
