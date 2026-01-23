import React, { useCallback, useState, useMemo } from "react";
import {GalleryThumbnails, X, ExternalLink, Hammer} from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useAssetsContext } from "@/features/assets/hooks/useAssetsContext";
import { useAssetsView } from "@/features/assets/hooks/useAssetsView";
import { SideChannelEvent } from "@/features/lumilio/schema";
import { AssetFilter } from "@/services/assetsService";
import { AssetViewDefinition } from "@/features/assets/assets.types.ts";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import { findAssetIndex, getFlatAssetsFromGrouped } from "@/lib/utils/assetGrouping";
import PageHeader from "@/components/PageHeader.tsx";
import {WorkerProvider} from "@/contexts/WorkerProvider.tsx";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";

interface AssetGalleryRendererProps {
  event: SideChannelEvent;
}

export const AssetGalleryRenderer: React.FC<AssetGalleryRendererProps> = ({
  event,
}) => {
  const navigate = useNavigate();
  const { dispatch: globalDispatch } = useAssetsContext();
  const filterDTO = event.data?.payload as AssetFilter;
  const [isModalOpen, setIsModalOpen] = useState(false);

  const handleOpenMainView = useCallback(() => {
    if (!filterDTO) return;
    
    // Reset and apply filters to global context
    globalDispatch({ type: "RESET_FILTERS" });
    
    const payload: any = { enabled: true };
    if (filterDTO.raw !== undefined) payload.raw = filterDTO.raw;
    if (filterDTO.rating !== undefined) payload.rating = filterDTO.rating;
    if (filterDTO.liked !== undefined) payload.liked = filterDTO.liked;
    if (filterDTO.filename) {
      payload.filename = {
        mode: filterDTO.filename.mode, 
        value: filterDTO.filename.value
      };
    }
    if (filterDTO.date) {
      payload.date = { from: filterDTO.date.from, to: filterDTO.date.to };
    }
    if (filterDTO.camera_make) payload.camera_make = filterDTO.camera_make;
    if (filterDTO.lens) payload.lens = filterDTO.lens;

    globalDispatch({ type: "BATCH_UPDATE_FILTERS", payload });
    navigate("/assets/photos");
    setIsModalOpen(false);
  }, [globalDispatch, filterDTO, navigate]);

  if (!filterDTO) return null;

  return (
    <div>
      <button
        className="btn btn-sm btn-outline btn-primary"
        onClick={() => setIsModalOpen(true)}
      >
        <GalleryThumbnails className="h-4 w-4 mr-2" />
        View Results
      </button>
      
      {isModalOpen && (
        <dialog className="modal modal-open">
          <div className="modal-box w-11/12 max-w-7xl h-[90vh] p-5 overflow-hidden flex flex-col bg-base-100 shadow-2xl">
            {/* Header */}
            <PageHeader title={`${event.tool.name} Results`} icon={<Hammer className="w-6 h-6 text-primary" />}>
              <div className="flex items-center gap-4">
                <button
                  className="btn btn-sm btn-soft btn-info"
                  onClick={handleOpenMainView}
                  title="Open in main gallery view"
                >
                  <ExternalLink className="w-4 h-4 mr-1" />
                  Open Full View
                </button>
                <button className="btn btn-sm btn-soft btn-info" onClick={() => setIsModalOpen(false)}>
                  <X className="w-5 h-5" />
                </button>
              </div>
            </PageHeader>
            {/* Content */}
            <div className="flex-1 overflow-y-auto relative bg-base-100" id="agent-gallery-container">
              <WorkerProvider preload={["exif","export"]}>
                <AssetsProvider persist={false}>
                  <AgentGallery filter={filterDTO} />
                </AssetsProvider>
              </WorkerProvider>
            </div>
          </div>
          <form method="dialog" className="modal-backdrop" onClick={() => setIsModalOpen(false)}>
            <button>close</button>
          </form>
        </dialog>
      )}
    </div>
  );
};

const AgentGallery = ({ filter }: { filter: AssetFilter }) => {
  const { dispatch } = useAssetsContext();
  const [carouselAssetId, setCarouselAssetId] = useState<string | undefined>();
  
  const viewDefinition = useMemo<AssetViewDefinition>(() => ({
    filter: filter,
    // Default to all types to prevent "missing query parameters" error from backend
    // when filter is empty (which triggers listAssets instead of filterAssets)
    types: ["photos", "videos", "audios"],
    groupBy: "date",
    pageSize: 50,
    inheritGlobalFilter: false,
  }), [filter]);

  const { 
    assets, 
    groups, 
    isLoading, 
    isLoadingMore, 
    hasMore, 
    fetchMore,
    error
  } = useAssetsView(viewDefinition, {
    withGroups: true,
    autoFetch: true
  });

  // Use flat assets from grouped to ensure order consistency with gallery
  const flatAssets = useMemo(() => {
    if (groups && Object.keys(groups).length > 0) {
      return getFlatAssetsFromGrouped(groups);
    }
    return assets;
  }, [groups, assets]);

  // Carousel logic
  const slideIndex = useMemo(() => {
    if (!carouselAssetId) return -1;
    return findAssetIndex(flatAssets, carouselAssetId);
  }, [flatAssets, carouselAssetId]);

  const handleLoadMore = useCallback(() => {
    if (hasMore && !isLoadingMore) {
      fetchMore();
    }
  }, [hasMore, isLoadingMore, fetchMore]);

  const isInitialLoading = isLoading && assets.length === 0;

  return (
    <div className="min-h-full p-4">
          {error && (
            <div className="alert alert-error mb-4">
              <span>Error: {error}</span>
            </div>
          )}

          {isInitialLoading ? (
            <PhotosLoadingSkeleton />
          ) : (
            <JustifiedGallery
                groupedPhotos={groups || {}}
                openCarousel={setCarouselAssetId}
                isLoading={isLoading}
                isLoadingMore={isLoadingMore}
                hasMore={hasMore}
                onLoadMore={handleLoadMore}
            />
          )}

          {carouselAssetId && flatAssets.length > 0 && (
              <FullScreenCarousel
                  photos={flatAssets}
                  initialSlide={slideIndex >= 0 ? slideIndex : 0}
                  slideIndex={slideIndex >= 0 ? slideIndex : undefined}
                  onClose={() => setCarouselAssetId(undefined)}
                  onNavigate={setCarouselAssetId}
                  onAssetUpdate={(updatedAsset) => dispatch({
                    type: "UPDATE_ENTITY",
                    payload: { assetId: updatedAsset.asset_id ?? "", updates: updatedAsset }
                  })}
                  onAssetDelete={(assetId) => dispatch({
                    type: "DELETE_ENTITY",
                    payload: { assetId }
                  })}
              />
          )}
    </div>
  );
};
