import { useMemo, useRef, useEffect, useState, useCallback } from "react";
import { Swiper, SwiperSlide } from "swiper/react";
import { Virtual, Navigation, Pagination } from "swiper/modules";
import { ImageOff, X } from "lucide-react";
import { ErrorBoundary } from "react-error-boundary";
import "swiper/css";
import "swiper/css/navigation";
import "swiper/css/pagination";
import "./asset-viewer.css";
import FullScreenBasicInfo from "./info/FullScreenBasicInfo";
import { useI18n } from "@/lib/i18n.tsx";
import { useViewerContextContributor } from "./useViewerContextContributor";
import MediaViewer from "./media/MediaViewer";
import type { Asset } from "@/lib/http-commons";
import { $api } from "@/lib/http-commons/queryClient";
import { normalizeSpeciesPredictions, parseSpeciesPrediction } from "./fieldGuide";
import { AssetFieldGuide } from "./AssetFieldGuide";
import { AssetViewerActions } from "./AssetViewerActions";

export interface AssetViewerProps {
  photos: Asset[];
  initialSlide: number;
  slideIndex?: number;
  onClose: () => void;
  onNavigate: (assetId: string) => void;
  onAssetUpdate?: (updatedAsset: Asset) => void;
  onAssetDelete?: (deletedAssetId: string) => void;
}

type AssetWithSpecies = Asset & {
  species_predictions?: unknown;
};

const AssetViewer = ({
  photos,
  initialSlide,
  slideIndex,
  onClose,
  onNavigate,
  onAssetUpdate,
  onAssetDelete,
}: AssetViewerProps) => {
  const swiperRef = useRef<any>(null),
    closingRef = useRef(false);
  const [showInfo, setShowInfo] = useState(false);
  const [showFieldGuide, setShowFieldGuide] = useState(false);
  const [currentAsset, setCurrentAsset] = useState(() => {
    const index = slideIndex !== undefined ? slideIndex : initialSlide;
    return photos[index] || photos[0] || null;
  });
  const [activeComponentId, setActiveComponentId] = useState<string | undefined>(
    currentAsset?.asset_id,
  );
  const activeComponentQuery = $api.useQuery(
    "get",
    "/api/v1/assets/{id}",
    {
      params: { path: { id: activeComponentId ?? "" } },
    },
    {
      enabled: Boolean(
        activeComponentId && currentAsset?.asset_id && activeComponentId !== currentAsset.asset_id,
      ),
      staleTime: 60_000,
    },
  );
  const activeAsset =
    activeComponentId === currentAsset?.asset_id
      ? currentAsset
      : (activeComponentQuery.data ?? null);
  useViewerContextContributor(activeAsset?.asset_id ?? currentAsset?.asset_id);
  const { t } = useI18n();
  const fieldGuideAssetQuery = $api.useQuery(
    "get",
    "/api/v1/assets/{id}",
    {
      params: {
        path: { id: activeAsset?.asset_id ?? "" },
        query: {
          include_thumbnails: false,
          include_tags: false,
          include_albums: false,
          include_species: true,
          include_ocr: false,
          include_faces: false,
        },
      },
    },
    {
      enabled: Boolean(activeAsset?.asset_id),
      staleTime: 60_000,
    },
  );
  const fieldGuideAsset = fieldGuideAssetQuery.data;

  const slides = useMemo(() => {
    return photos.map((photo) => ({
      asset: photo,
      assetId: photo.asset_id!,
    }));
  }, [photos]);
  const speciesPredictions = useMemo(
    () =>
      normalizeSpeciesPredictions(
        fieldGuideAsset?.species_predictions ??
          (activeAsset as AssetWithSpecies | null)?.species_predictions,
      ),
    [activeAsset, fieldGuideAsset?.species_predictions],
  );
  const parsedSpeciesPredictions = useMemo(
    () =>
      speciesPredictions
        .map(parseSpeciesPrediction)
        .filter((item): item is NonNullable<typeof item> => Boolean(item))
        .slice(0, 3),
    [speciesPredictions],
  );
  const hasFieldGuide = parsedSpeciesPredictions.length > 0;
  const isFieldGuideLoading =
    fieldGuideAssetQuery.isLoading && parsedSpeciesPredictions.length === 0;

  const onSlideChange = (swiper: any) => {
    if (closingRef.current) return;
    const idx = swiper.activeIndex;
    const assetId = slides[idx]?.assetId;
    if (assetId && assetId !== currentAsset?.asset_id) onNavigate(assetId);
    if (photos[idx]) setCurrentAsset(photos[idx]);
  };

  const toggleInfo = useCallback(() => {
    setShowInfo((visible) => !visible);
  }, []);
  const toggleFieldGuide = useCallback(() => {
    if (!hasFieldGuide) return;
    setShowFieldGuide((visible) => !visible);
  }, [hasFieldGuide]);
  const handleClose = useCallback(() => {
    closingRef.current = true;
    onClose();
  }, [onClose]);

  // Sync swiper to external slideIndex when it changes
  useEffect(() => {
    if (slideIndex !== undefined && swiperRef.current) {
      const swiper = swiperRef.current.swiper;
      if (swiper && swiper.activeIndex !== slideIndex) {
        swiper.slideTo(slideIndex);
      }
    }
  }, [slideIndex]);

  // Update currentAsset when photos or slideIndex/initialSlide changes
  useEffect(() => {
    const index = slideIndex !== undefined ? slideIndex : initialSlide;
    if (photos[index] && photos[index].asset_id !== currentAsset?.asset_id) {
      setCurrentAsset(photos[index]);
    }
  }, [photos, slideIndex, initialSlide, currentAsset?.asset_id]);

  useEffect(() => {
    setActiveComponentId(currentAsset?.asset_id);
  }, [currentAsset?.asset_id]);

  useEffect(() => {
    const handler = () => setShowInfo((s) => !s);
    window.addEventListener("fullscreen:toggleInfo", handler);
    return () => window.removeEventListener("fullscreen:toggleInfo", handler);
  }, []);

  useEffect(() => {
    if (
      showFieldGuide &&
      !hasFieldGuide &&
      !isFieldGuideLoading &&
      !fieldGuideAssetQuery.isFetching
    ) {
      setShowFieldGuide(false);
    }
  }, [fieldGuideAssetQuery.isFetching, hasFieldGuide, isFieldGuideLoading, showFieldGuide]);

  // Add keyboard event handler for Escape key
  useEffect(() => {
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        handleClose();
      }
    };

    document.addEventListener("keydown", handleKeyDown);
    return () => document.removeEventListener("keydown", handleKeyDown);
  }, [handleClose]);

  const handleAssetUpdate = (updatedAsset: Asset) => {
    if (updatedAsset.asset_id === currentAsset?.asset_id) {
      setCurrentAsset(updatedAsset);
    }
    if (onAssetUpdate) {
      onAssetUpdate(updatedAsset);
    }
  };

  const handleAssetDelete = useCallback(
    (assetId: string) => {
      onAssetDelete?.(assetId);
      handleClose();
    },
    [handleClose, onAssetDelete],
  );

  return (
    <div className="fixed inset-0 bg-black/90 z-9999 flex items-center justify-center animate-fade-in">
      <button
        type="button"
        onClick={handleClose}
        className="btn btn-ghost btn-sm absolute top-2 left-4 text-white z-20"
        aria-label={t("common.close")}
      >
        <X className="w-6 h-6" />
      </button>
      <Swiper
        ref={swiperRef}
        modules={[Virtual, Navigation, Pagination]}
        spaceBetween={50}
        slidesPerView={1}
        virtual
        navigation
        pagination={{ clickable: true }}
        onSlideChange={onSlideChange}
        initialSlide={slideIndex !== undefined ? slideIndex : initialSlide}
        className="fullscreen-swiper"
      >
        {slides.map((slide, index) => (
          <SwiperSlide key={slide.assetId} virtualIndex={index}>
            {({ isActive }) => (
              <ErrorBoundary
                fallback={
                  <div className="h-screen w-screen flex items-center justify-center">
                    <div className="flex flex-col items-center gap-3 text-white/60">
                      <ImageOff className="size-10" />
                      <p className="text-sm">{t("assets.mediaViewer.media_not_available")}</p>
                    </div>
                  </div>
                }
              >
                <MediaViewer
                  asset={slide.asset}
                  isActive={isActive}
                  selectedAssetId={isActive ? activeComponentId : slide.asset.asset_id}
                  onSelectedAssetChange={isActive ? setActiveComponentId : undefined}
                />
              </ErrorBoundary>
            )}
          </SwiperSlide>
        ))}
      </Swiper>

      {/* Info panel */}
      {showInfo && activeAsset && (
        <FullScreenBasicInfo asset={activeAsset} onAssetUpdate={handleAssetUpdate} />
      )}

      <AssetFieldGuide
        open={showFieldGuide}
        loading={isFieldGuideLoading}
        error={fieldGuideAssetQuery.isError}
        predictions={parsedSpeciesPredictions}
        onToggle={toggleFieldGuide}
      />

      <AssetViewerActions
        asset={activeAsset}
        deleteTarget={currentAsset}
        showInfo={showInfo}
        onToggleInfo={toggleInfo}
        onAssetUpdate={handleAssetUpdate}
        onAssetDelete={handleAssetDelete}
      />
    </div>
  );
};

export default AssetViewer;
