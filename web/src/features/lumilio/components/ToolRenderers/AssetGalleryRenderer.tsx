import React, { useCallback, useState, useMemo } from "react";
import { GalleryThumbnails, X, ExternalLink, Hammer } from "lucide-react";
import { useNavigate } from "react-router-dom";
import { useAssetsView } from "@/features/assets/hooks/useAssetsView";
import { SideChannelEvent } from "@/features/lumilio/schema";
import { AssetFilter } from "@/lib/assets/types";
import { AssetViewDefinition } from "@/features/assets/types/assets.type";
import JustifiedGallery from "@/features/assets/components/page/JustifiedGallery/JustifiedGallery";
import FullScreenCarousel from "@/features/assets/components/page/FullScreen/FullScreenCarousel/FullScreenCarousel";
import PageHeader from "@/components/PageHeader.tsx";
import { WorkerProvider } from "@/contexts/WorkerProvider.tsx";
import { AssetsProvider } from "@/features/assets/AssetsProvider";
import PhotosLoadingSkeleton from "@/features/assets/components/page/LoadingSkeleton";
import {
  findAssetIndex,
  flattenAssetGroups,
} from "@/features/assets/utils/assetGroups";
import { useI18n } from "@/lib/i18n.tsx";

interface AssetGalleryRendererProps {
  event: SideChannelEvent;
}

export const AssetGalleryRenderer: React.FC<AssetGalleryRendererProps> = ({
  event,
}) => {
  const { t } = useI18n();
  const navigate = useNavigate();
  const filterDTO = event.data?.payload as AssetFilter;
  const [isModalOpen, setIsModalOpen] = useState(false);

  const handleOpenMainView = useCallback(() => {
    if (!filterDTO) return;

    navigate("/assets", {
      state: {
        assetsInitialFilter: filterDTO,
      },
    });
    setIsModalOpen(false);
  }, [filterDTO, navigate]);

  if (!filterDTO) return null;

  return (
    <div>
      <button
        className="btn btn-sm btn-outline btn-primary"
        onClick={() => setIsModalOpen(true)}
      >
        <GalleryThumbnails className="h-4 w-4 mr-2" />
        {t("lumilio.tools.viewResults")}
      </button>

      {isModalOpen && (
        <dialog className="modal modal-open">
          <div className="modal-box w-11/12 max-w-7xl h-[90vh] p-5 overflow-hidden flex flex-col bg-base-100 shadow-2xl">
            {/* Header */}
            <PageHeader
              title={t("lumilio.tools.resultsTitle", { toolName: event.tool.name })}
              icon={<Hammer className="w-6 h-6 text-primary" />}
            >
              <div className="flex items-center gap-4">
                <button
                  className="btn btn-sm btn-soft btn-info"
                  onClick={handleOpenMainView}
                  title={t("lumilio.tools.openMainGallery")}
                >
                  <ExternalLink className="w-4 h-4 mr-1" />
                  {t("lumilio.tools.openFullView")}
                </button>
                <button className="btn btn-sm btn-soft btn-info" onClick={() => setIsModalOpen(false)}>
                  <X className="w-5 h-5" />
                </button>
              </div>
            </PageHeader>
            {/* Content */}
            <div className="flex-1 overflow-y-auto relative bg-base-100" id="agent-gallery-container">
              <WorkerProvider preload={["exif", "export"]}>
                <AssetsProvider
                  key={`lumilio-result:${event.tool.executionId}`}
                  scopeId={`lumilio-result:${event.tool.executionId}`}
                  persist={false}
                >
                  <AgentGallery filter={filterDTO} />
                </AssetsProvider>
              </WorkerProvider>
            </div>
          </div>
          <form method="dialog" className="modal-backdrop" onClick={() => setIsModalOpen(false)}>
            <button>{t("common.close")}</button>
          </form>
        </dialog>
      )}
    </div>
  );
};

const AgentGallery = ({ filter }: { filter: AssetFilter }) => {
  const { t } = useI18n();
  const [carouselAssetId, setCarouselAssetId] = useState<string | undefined>();

  const viewDefinition = useMemo<AssetViewDefinition>(() => ({
    filter: filter,
    // Default to all types to prevent "missing query parameters" error from backend
    // when filter is empty (which triggers listAssets instead of filterAssets)
    types: ["photos", "videos", "audios"],
    sortBy: "date_captured",
    pageSize: 50,
  }), [filter]);

  const {
    assets,
    groups,
    isLoading,
    isLoadingMore,
    hasMore,
    fetchMore,
    error,
  } = useAssetsView(viewDefinition, {
    withGroups: true,
    autoFetch: true
  });

  // Use flat assets from grouped to ensure order consistency with gallery
  const flatAssets = useMemo(() => {
    if (groups && groups.length > 0) {
      return flattenAssetGroups(groups);
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
          <span>{t("lumilio.tools.error", { error: String(error) })}</span>
        </div>
      )}

      {isInitialLoading ? (
        <PhotosLoadingSkeleton />
      ) : (
        <JustifiedGallery
          groups={groups || []}
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
        />
      )}
    </div>
  );
};
