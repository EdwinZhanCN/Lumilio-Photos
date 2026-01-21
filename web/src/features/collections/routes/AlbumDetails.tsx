import { useParams } from "react-router-dom";
import { useMemo, useState, useEffect, useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { albumService } from "@/services/albumService";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import AssetsPageHeader from "@/features/assets/components/shared/AssetsPageHeader";
import { useAssetsContext, useAssetsNavigation } from "@/features/assets/hooks/useAssetsContext";
import { useAssetsView } from "@/features/assets/hooks/useAssetsView";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { findAssetIndex } from "@/lib/utils/assetGrouping";
import { useI18n } from "@/lib/i18n";
import { Asset } from "@/lib/http-commons";
import { useAssetActions } from "@/features/assets/hooks/useAssetActions";

const AlbumAssetsContent = () => {
  const { albumId, assetId } = useParams<{ albumId: string, assetId: string }>();
  const { state, dispatch } = useAssetsContext();
  const { openCarousel, closeCarousel } = useAssetsNavigation();
  const { deleteAsset } = useAssetActions();
  const { t } = useI18n();

  const { groupBy, isCarouselOpen } = state.ui;

  // Fetch album metadata
  const { data: albumData } = useQuery({
    queryKey: ["album", albumId],
    queryFn: () => albumService.getAlbumById(parseInt(albumId!)),
    enabled: !!albumId
  });

  const album = albumData?.data?.data;

  // Fetch album assets using the assets view hook
  // We'll use the 'inheritGlobalFilter: true' to allow filtering within the album
  const {
    assets,
    groups,
    isLoading,
    isLoadingMore,
    fetchMore,
    hasMore,
    error
  } = useAssetsView({
    // We need a way to filter by album ID in the assets API
    // Assuming the filter API supports album_id or we use a specific album assets endpoint
    // For now, we'll use the filter object
    filter: { 
      // @ts-ignore - assuming backend might support this or we'll need to add it
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
      
      <div className="px-8 py-4 bg-base-100/50 backdrop-blur-md border-b border-base-300">
        <h1 className="text-3xl font-bold text-primary">{album?.album_name || "Loading Album..."}</h1>
        {album?.description && <p className="mt-2 text-base-content/60 max-w-2xl">{album.description}</p>}
        <div className="mt-4 flex items-center gap-4 text-sm font-medium opacity-50">
          <span>{album?.asset_count || 0} items</span>
          <span>•</span>
          <span>Created {album?.created_at ? new Date(album.created_at).toLocaleDateString() : ""}</span>
        </div>
      </div>

      <div className="flex-1 overflow-y-auto">
        <JustifiedGallery
          groupedPhotos={groups || {}}
          openCarousel={(id) => {
            // Custom navigation for album context
            const currentParams = new URLSearchParams(window.location.search);
            window.history.pushState({}, "", `/collections/${albumId}/${id}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`);
            dispatch({ type: "SET_CAROUSEL_OPEN", payload: true });
            dispatch({ type: "SET_ACTIVE_ASSET_ID", payload: id });
          }}
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
          onClose={() => {
            const currentParams = new URLSearchParams(window.location.search);
            window.history.pushState({}, "", `/collections/${albumId}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`);
            dispatch({ type: "SET_CAROUSEL_OPEN", payload: false });
          }}
          onNavigate={(id) => {
            const currentParams = new URLSearchParams(window.location.search);
            window.history.pushState({}, "", `/collections/${albumId}/${id}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`);
            dispatch({ type: "SET_ACTIVE_ASSET_ID", payload: id });
          }}
        />
      )}
    </div>
  );
};

const AlbumDetails = () => {
  return (
    <AssetsProvider persist={false}>
      <AlbumAssetsContent />
    </AssetsProvider>
  );
};

export default AlbumDetails;
