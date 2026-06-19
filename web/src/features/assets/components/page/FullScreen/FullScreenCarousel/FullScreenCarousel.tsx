import {
  useMemo,
  useRef,
  useEffect,
  useState,
  useCallback,
  useOptimistic,
  useTransition,
} from "react";
import { createPortal } from "react-dom";
import { useNavigate } from "react-router-dom";
import { Swiper, SwiperSlide } from "swiper/react";
import { Virtual, Navigation, Pagination } from "swiper/modules";
import {
  Bird,
  ChevronUp,
  Ellipsis,
  ExternalLink,
  Info,
  ScanSearch,
  Share,
  Heart,
  Telescope,
  Trash2,
  X,
  ImageOff,
  Plus,
} from "lucide-react";
import { ErrorBoundary } from "react-error-boundary";
import ExportModal from "@/components/ExportModal";
import "swiper/css";
import "swiper/css/navigation";
import "swiper/css/pagination";
import "@/styles/custom-swiper.css";
import FullScreenBasicInfo from "../FullScreenInfo/FullScreenBasicInfo";
import { useI18n } from "@/lib/i18n.tsx";
import { useCarouselContextContributor } from "@/features/lumilio/contributors/useCarouselContextContributor";
import { useDockStore } from "@/features/lumilio/state/dockStore";
import { LumilioAvatar } from "@/features/lumilio/components/LumilioAvatar/LumilioAvatar";
import { useAssetActions } from "@/features/assets/hooks/useAssetActions";
import MediaViewer from "../../../shared/MediaViewer";
import type { Asset, components } from "@/lib/http-commons";
import { $api } from "@/lib/http-commons/queryClient";
import type { Album } from "@/lib/albums/types";
import {
  type ParsedSpeciesPrediction,
  type TaxonomyRank,
  formatSpeciesScore,
  getSpeciesScorePercent,
  normalizeSpeciesPredictions,
  parseSpeciesPrediction,
  TAXONOMY_RANKS,
} from "./fieldGuide";

interface FullScreenCarouselProps {
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

type SpeciesReference =
  components["schemas"]["dto.SpeciesReferenceResponseDTO"];

type SpeciesReferenceTriggerProps = {
  prediction: ParsedSpeciesPrediction;
};

const SpeciesReferenceTrigger = ({
  prediction,
}: SpeciesReferenceTriggerProps) => {
  const [isOpen, setIsOpen] = useState(false);
  const triggerRef = useRef<HTMLSpanElement>(null);
  const [position, setPosition] = useState({ top: 0, left: 0 });
  const closeTimeoutRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const { t, i18n } = useI18n();
  const inaturalistLocale = useMemo(() => {
    const language = i18n.resolvedLanguage || i18n.language || "en";
    return language.toLowerCase().startsWith("zh") ? "zh" : "en";
  }, [i18n.language, i18n.resolvedLanguage]);
  const queryName =
    prediction.scientificName ??
    prediction.commonName ??
    prediction.displayName;
  const referenceQuery = $api.useQuery(
    "get",
    "/api/v1/species/reference",
    {
      params: {
        query: {
          scientific_name: prediction.scientificName,
          common_name: prediction.commonName ?? prediction.displayName,
          locale: inaturalistLocale,
        },
      },
    },
    {
      enabled: isOpen && Boolean(queryName),
      staleTime: 24 * 60 * 60 * 1000,
      retry: 1,
    },
  );
  const reference = referenceQuery.data as SpeciesReference | undefined;

  const handleOpen = useCallback(() => {
    if (closeTimeoutRef.current) clearTimeout(closeTimeoutRef.current);
    if (triggerRef.current) {
      const rect = triggerRef.current.getBoundingClientRect();
      setPosition({ top: rect.top, left: rect.right + 8 });
    }
    setIsOpen(true);
  }, []);

  const handleClose = useCallback(() => {
    closeTimeoutRef.current = setTimeout(() => setIsOpen(false), 150);
  }, []);

  useEffect(() => {
    return () => {
      if (closeTimeoutRef.current) clearTimeout(closeTimeoutRef.current);
    };
  }, []);

  const tooltip = isOpen && (
    <div
      style={{
        position: "fixed",
        left: position.left,
        top: position.top,
        zIndex: 9999,
      }}
      className="w-[min(520px,calc(100vw-96px))] rounded-xl border border-white/12 bg-zinc-950/95 p-3 text-left text-white shadow-2xl shadow-black/40 backdrop-blur-xl"
      role="tooltip"
      onMouseEnter={handleOpen}
      onMouseLeave={handleClose}
    >
      <div className="mb-3 flex items-start justify-between gap-3">
        <div className="min-w-0">
          <p className="text-xs font-semibold text-white/86">
            {t("assets.photos.fullscreen.fieldGuide.reference")}
          </p>
          <p className="text-[11px] text-white/42">
            {t("assets.photos.fullscreen.fieldGuide.fromINaturalist")}
          </p>
        </div>
        {reference?.reference_url && (
          <a
            href={reference.reference_url}
            target="_blank"
            rel="noreferrer"
            className="grid size-7 shrink-0 place-items-center rounded-full bg-white/8 text-white/58 hover:bg-white/12 hover:text-white"
            aria-label={t(
              "assets.photos.fullscreen.fieldGuide.openINaturalist",
            )}
            title={t("assets.photos.fullscreen.fieldGuide.openINaturalist")}
          >
            <ExternalLink className="size-3.5" />
          </a>
        )}
      </div>

      {referenceQuery.isLoading ? (
        <div className="grid grid-cols-[72px_1fr] gap-3">
          <div className="h-20 rounded-lg bg-white/10" />
          <div className="space-y-2.5">
            <div className="h-3.5 w-32 rounded-full bg-white/14" />
            <div className="h-3 w-full rounded-full bg-white/10" />
            <div className="h-3 w-5/6 rounded-full bg-white/10" />
            <div className="h-3 w-2/3 rounded-full bg-white/10" />
          </div>
        </div>
      ) : reference ? (
        <div className="space-y-3">
          <div className="grid grid-cols-[72px_1fr] gap-3">
            {reference.image_url ? (
              <img
                src={reference.image_url}
                alt={
                  reference.common_name ??
                  reference.scientific_name ??
                  prediction.displayName
                }
                className="h-20 w-18 rounded-lg object-cover"
                loading="lazy"
              />
            ) : (
              <div className="grid h-20 w-18 place-items-center rounded-lg bg-white/8 text-white/35">
                <ImageOff className="size-5" />
              </div>
            )}
            <div className="min-w-0">
              <h4 className="truncate text-sm font-semibold text-white/90">
                {reference.common_name ?? prediction.displayName}
              </h4>
              {reference.scientific_name && (
                <p className="truncate text-xs italic text-white/50">
                  {reference.scientific_name}
                </p>
              )}
              {reference.wikipedia_summary && (
                <p className="mt-1.5 line-clamp-3 text-xs leading-5 text-white/62">
                  {reference.wikipedia_summary}
                </p>
              )}
            </div>
          </div>

          <div className="flex flex-wrap items-center gap-2 text-[11px] text-white/45">
            {reference.image_license && (
              <span className="rounded-full bg-white/8 px-2 py-1 uppercase text-white/55">
                {reference.image_license}
              </span>
            )}
            {reference.wikipedia_url && (
              <a
                href={reference.wikipedia_url}
                target="_blank"
                rel="noreferrer"
                className="inline-flex items-center gap-1 rounded-full bg-white/8 px-2 py-1 text-white/68 hover:bg-white/12 hover:text-white"
              >
                {t("assets.photos.fullscreen.fieldGuide.openWikipedia")}
                <ExternalLink className="size-3" />
              </a>
            )}
            {reference.image_attribution && (
              <span className="min-w-0 truncate">
                {reference.image_attribution}
              </span>
            )}
          </div>
        </div>
      ) : (
        <p className="text-xs leading-5 text-white/52">
          {t("assets.photos.fullscreen.fieldGuide.referenceError")}
        </p>
      )}
    </div>
  );

  return (
    <>
      <span
        ref={triggerRef}
        className="inline-flex shrink-0"
        onMouseEnter={handleOpen}
        onMouseLeave={handleClose}
        onFocus={() => setIsOpen(true)}
        onBlur={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget)) {
            setIsOpen(false);
          }
        }}
      >
        <button
          type="button"
          className="btn btn-soft btn-info btn-sm btn-circle"
          aria-label={t("assets.photos.fullscreen.fieldGuide.reference")}
          title={t("assets.photos.fullscreen.fieldGuide.reference")}
        >
          <Telescope className="size-3.5" />
        </button>
      </span>
      {isOpen && createPortal(tooltip, document.body)}
    </>
  );
};

function getRankLabel(t: (key: string) => string, rank: TaxonomyRank) {
  switch (rank) {
    case "kingdom":
      return t("assets.photos.fullscreen.fieldGuide.ranks.kingdom");
    case "phylum":
      return t("assets.photos.fullscreen.fieldGuide.ranks.phylum");
    case "class":
      return t("assets.photos.fullscreen.fieldGuide.ranks.class");
    case "order":
      return t("assets.photos.fullscreen.fieldGuide.ranks.order");
    case "family":
      return t("assets.photos.fullscreen.fieldGuide.ranks.family");
    case "genus":
      return t("assets.photos.fullscreen.fieldGuide.ranks.genus");
    case "species":
      return t("assets.photos.fullscreen.fieldGuide.ranks.species");
  }
}

const FullScreenCarousel = ({
  photos,
  initialSlide,
  slideIndex,
  onClose,
  onNavigate,
  onAssetUpdate,
  onAssetDelete,
}: FullScreenCarouselProps) => {
  const swiperRef = useRef<any>(null),
    closingRef = useRef(false);
  const [showInfo, setShowInfo] = useState(false);
  const [showFieldGuide, setShowFieldGuide] = useState(false);
  const [currentAsset, setCurrentAsset] = useState(() => {
    const index = slideIndex !== undefined ? slideIndex : initialSlide;
    return photos[index] || photos[0] || null;
  });
  useCarouselContextContributor(currentAsset?.asset_id);
  const openAgentDock = useDockStore((s) => s.setCollapsed);
  const [agentFabHovered, setAgentFabHovered] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const { t } = useI18n();
  const { toggleLike, deleteAsset } = useAssetActions();
  const navigate = useNavigate();
  const [albums, setAlbums] = useState<Album[]>([]);
  const [isLoadingAlbums, setIsLoadingAlbums] = useState(false);
  const [isAddingToAlbum, setIsAddingToAlbum] = useState(false);
  const listAlbumsMutation = $api.useMutation("get", "/api/v1/albums");
  const addToAlbumMutation = $api.useMutation(
    "post",
    "/api/v1/albums/{id}/assets/{assetId}",
  );

  const handleOpenStudio = useCallback(
    (asset: Asset) => {
      navigate(`/studio?assetId=${asset.asset_id}`);
    },
    [navigate],
  );

  const handleAddToAlbum = useCallback(
    async (_asset: Asset) => {
      setIsLoadingAlbums(true);
      try {
        const response = await listAlbumsMutation.mutateAsync({
          params: { query: { limit: 50 } },
        });
        if (response?.albums) {
          setAlbums(response.albums);
        }
        const modal = document.getElementById(
          "album_picker_modal",
        ) as HTMLDialogElement | null;
        modal?.showModal();
      } catch {
        setAlbums([]);
        const modal = document.getElementById(
          "album_picker_modal",
        ) as HTMLDialogElement | null;
        modal?.showModal();
      } finally {
        setIsLoadingAlbums(false);
      }
    },
    [listAlbumsMutation],
  );

  const handleSelectAlbum = useCallback(
    async (albumId: number) => {
      if (!currentAsset?.asset_id) return;
      setIsAddingToAlbum(true);
      try {
        await addToAlbumMutation.mutateAsync({
          params: {
            path: { id: albumId, assetId: currentAsset.asset_id },
          },
          body: {},
        });
        const modal = document.getElementById(
          "album_picker_modal",
        ) as HTMLDialogElement | null;
        modal?.close();
      } catch {
        // silently fail — album picker stays open for retry
      } finally {
        setIsAddingToAlbum(false);
      }
    },
    [currentAsset, addToAlbumMutation],
  );

  const fieldGuideAssetQuery = $api.useQuery(
    "get",
    "/api/v1/assets/{id}",
    {
      params: {
        path: { id: currentAsset?.asset_id ?? "" },
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
      enabled: Boolean(currentAsset?.asset_id),
      staleTime: 60_000,
    },
  );
  const fieldGuideAsset = fieldGuideAssetQuery.data as
    | AssetWithSpecies
    | undefined;

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
          (currentAsset as AssetWithSpecies | null)?.species_predictions,
      ),
    [currentAsset, fieldGuideAsset?.species_predictions],
  );
  const parsedSpeciesPredictions = useMemo(
    () =>
      speciesPredictions
        .map(parseSpeciesPrediction)
        .filter((item): item is NonNullable<typeof item> => Boolean(item))
        .slice(0, 3),
    [speciesPredictions],
  );
  const primarySpeciesPrediction = parsedSpeciesPredictions[0];
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

  const toggleInfo = () => {
    setShowInfo(!showInfo);
  };
  const toggleFieldGuide = () => {
    if (!hasFieldGuide) return;
    setShowFieldGuide((visible) => !visible);
  };
  const handleClose = () => {
    closingRef.current = true;
    onClose();
  };
  //

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
  }, [
    fieldGuideAssetQuery.isFetching,
    hasFieldGuide,
    isFieldGuideLoading,
    showFieldGuide,
  ]);

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
    setCurrentAsset(updatedAsset);
    if (onAssetUpdate) {
      onAssetUpdate(updatedAsset);
    }
  };

  const [, startTransition] = useTransition();
  const [optimisticLiked, setOptimisticLiked] = useOptimistic(
    currentAsset?.liked ?? false,
    (_state, newLiked: boolean) => newLiked,
  );

  const handleLikeToggle = () => {
    if (!currentAsset?.asset_id) return;
    const newLiked = !optimisticLiked;
    const assetId = currentAsset.asset_id;

    startTransition(async () => {
      setOptimisticLiked(newLiked);
      try {
        await toggleLike(assetId, newLiked);
        handleAssetUpdate({
          ...currentAsset,
          liked: newLiked,
        });
      } catch (error) {
        console.error("Failed to update like status:", error);
      }
    });
  };

  const handleDeleteAsset = async () => {
    if (!currentAsset?.asset_id || isDeleting) return;

    setIsDeleting(true);
    try {
      if (currentAsset.asset_id) {
        await deleteAsset(currentAsset.asset_id);

        // Notify parent about deletion
        if (onAssetDelete) {
          onAssetDelete(currentAsset.asset_id);
        }

        // Close the carousel
        handleClose();
      }
    } catch (error) {
      console.error("Failed to delete asset:", error);
    } finally {
      setIsDeleting(false);
      // Close the modal
      const modal = document.getElementById(
        "delete_confirm_modal",
      ) as HTMLDialogElement;
      modal?.close();
    }
  };

  return (
    <div className="fixed inset-0 bg-black/90 z-9999 flex items-center justify-center animate-fade-in">
      <button
        onClick={handleClose}
        className="btn btn-ghost btn-sm absolute top-2 left-4 text-white z-20"
      >
        <X className="w-6 h-6" />
      </button>
      <button
        type="button"
        onClick={() => openAgentDock(false)}
        onMouseEnter={() => setAgentFabHovered(true)}
        onMouseLeave={() => setAgentFabHovered(false)}
        title={t("lumilio.dock.title", "Lumilio Agent")}
        aria-label={t("lumilio.dock.title", "Lumilio Agent")}
        className="absolute right-4 top-4 z-20 transition-transform hover:scale-110"
      >
        <LumilioAvatar start={agentFabHovered} size={0.2} />
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
                      <p className="text-sm">
                        {t("assets.mediaViewer.media_not_available")}
                      </p>
                    </div>
                  </div>
                }
              >
                <MediaViewer asset={slide.asset} isActive={isActive} />
              </ErrorBoundary>
            )}
          </SwiperSlide>
        ))}
      </Swiper>

      {/* Info panel */}
      {showInfo && currentAsset && (
        <FullScreenBasicInfo
          asset={currentAsset}
          onAssetUpdate={handleAssetUpdate}
        />
      )}

      {showFieldGuide && (
        <aside className="absolute left-6 bottom-28 z-20 max-h-[calc(100vh-9rem)] w-[calc(100vw-48px)] max-w-[420px] overflow-y-auto rounded-2xl border border-white/10 bg-zinc-950/78 text-white shadow-2xl shadow-emerald-950/30 backdrop-blur-2xl">
          <div className="p-5">
            <div className="mb-5 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="grid size-9 place-items-center rounded-full bg-emerald-400/15 text-emerald-200 ring-1 ring-emerald-300/20">
                  <ScanSearch className="size-5" />
                </div>
                <div>
                  <h2 className="text-sm font-semibold tracking-wide">
                    {t("assets.photos.fullscreen.fieldGuide.topLabels")}
                  </h2>
                  <p className="text-xs text-white/45">
                    {isFieldGuideLoading
                      ? t("assets.photos.fullscreen.fieldGuide.loading")
                      : fieldGuideAssetQuery.isError
                        ? t("assets.photos.fullscreen.fieldGuide.loadError")
                        : parsedSpeciesPredictions.length > 0
                          ? t(
                              "assets.photos.fullscreen.fieldGuide.predictionsCount",
                              { count: parsedSpeciesPredictions.length },
                            )
                          : t("assets.photos.fullscreen.fieldGuide.noResults")}
                  </p>
                </div>
              </div>
              <button
                type="button"
                className="btn btn-circle btn-ghost btn-xs text-white/55 hover:bg-white/10 hover:text-white"
                onClick={toggleFieldGuide}
                aria-label={t("common.close")}
              >
                <X className="size-4" />
              </button>
            </div>

            <div className="space-y-4">
              {isFieldGuideLoading ? (
                [0, 1, 2].map((item) => (
                  <div key={item} className="grid grid-cols-[28px_1fr] gap-3">
                    <div className="grid size-6 place-items-center rounded-full bg-white/10 text-xs font-semibold text-white/70">
                      {item + 1}
                    </div>
                    <div className="min-w-0">
                      <div className="mb-2 flex items-center justify-between gap-4">
                        <div className="h-4 w-40 rounded-full bg-white/14" />
                        <div className="h-4 w-10 rounded-full bg-white/18" />
                      </div>
                      <div className="mb-3 h-3 w-28 rounded-full bg-white/10" />
                      <div className="h-1.5 rounded-full bg-white/12">
                        <div
                          className="h-full rounded-full bg-gradient-to-r from-emerald-300 via-lime-300 to-white/85"
                          style={{ width: `${88 - item * 12}%` }}
                        />
                      </div>
                    </div>
                  </div>
                ))
              ) : parsedSpeciesPredictions.length > 0 ? (
                parsedSpeciesPredictions.map((prediction, index) => (
                  <div
                    key={`${prediction.label}-${index}`}
                    className="grid grid-cols-[28px_1fr] gap-3"
                  >
                    <div className="grid size-6 place-items-center rounded-full bg-white/10 text-xs font-semibold text-white/70">
                      {index + 1}
                    </div>
                    <div className="min-w-0">
                      <div className="mb-1.5 flex items-start justify-between gap-4">
                        <div className="min-w-0">
                          <div className="flex min-w-0 items-center gap-1.5">
                            <h3 className="truncate text-sm font-semibold leading-5">
                              {prediction.displayName}
                            </h3>
                            <SpeciesReferenceTrigger prediction={prediction} />
                          </div>
                          {prediction.scientificName &&
                            prediction.scientificName !==
                              prediction.displayName && (
                              <p className="truncate text-xs italic text-white/50">
                                {prediction.scientificName}
                              </p>
                            )}
                        </div>
                        <span className="shrink-0 text-sm font-semibold text-white/88">
                          {formatSpeciesScore(prediction.score)}
                        </span>
                      </div>
                      <div className="h-1.5 rounded-full bg-white/12">
                        <div
                          className="h-full rounded-full bg-gradient-to-r from-emerald-300 via-lime-300 to-white/85"
                          style={{
                            width: `${getSpeciesScorePercent(prediction.score)}%`,
                          }}
                        />
                      </div>
                    </div>
                  </div>
                ))
              ) : (
                <div className="rounded-xl border border-white/10 bg-white/5 px-4 py-5 text-sm text-white/58">
                  {t("assets.photos.fullscreen.fieldGuide.noResults")}
                </div>
              )}
            </div>
          </div>

          <div className="border-t border-white/10 p-5">
            <div className="mb-4 flex items-center justify-between">
              <div className="flex items-center gap-3">
                <Bird className="size-5 text-emerald-200" />
                <h3 className="text-sm font-semibold">
                  {t("assets.photos.fullscreen.fieldGuide.taxonomy")}
                </h3>
              </div>
              <ChevronUp className="size-4 text-white/55" />
            </div>
            <div className="space-y-3">
              {TAXONOMY_RANKS.map((rank) => {
                const value = primarySpeciesPrediction?.taxonomy[rank];
                return (
                  <div
                    key={rank}
                    className="grid grid-cols-[104px_1fr] items-center border-b border-white/10 pb-2 text-sm last:border-b-0 last:pb-0"
                  >
                    <span className="text-white/48">
                      {getRankLabel(t, rank)}
                    </span>
                    <span
                      className={
                        value ? "truncate text-white/88" : "text-white/28"
                      }
                    >
                      {value ?? "-"}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>
        </aside>
      )}

      {hasFieldGuide && (
        <button
          type="button"
          className={`group absolute bottom-6 left-6 z-30 flex h-16 min-w-16 items-center gap-3 rounded-full border px-5 text-white shadow-2xl backdrop-blur-xl transition duration-200 ${
            showFieldGuide
              ? "border-emerald-200/55 bg-emerald-400/22 shadow-emerald-500/25"
              : "border-white/15 bg-zinc-950/64 shadow-black/40 hover:border-emerald-200/45 hover:bg-emerald-400/18 hover:shadow-emerald-500/20"
          }`}
          onClick={toggleFieldGuide}
          aria-label={t("assets.photos.fullscreen.fieldGuide.open")}
          title={t("assets.photos.fullscreen.fieldGuide.open")}
        >
          <span className="grid size-9 place-items-center rounded-full bg-emerald-300 text-zinc-950 ring-4 ring-emerald-300/18 transition group-hover:scale-105">
            <Bird className="size-5" />
          </span>
          <span className="hidden pr-1 text-sm font-semibold tracking-wide sm:inline">
            {t("assets.photos.fullscreen.fieldGuide.button")}
          </span>
        </button>
      )}

      {/* Export modal */}
      <ExportModal
        asset={currentAsset}
        onOpenStudio={handleOpenStudio}
        onAddToAlbum={handleAddToAlbum}
      />

      {/* Delete confirmation modal */}
      <dialog id="delete_confirm_modal" className="modal">
        <div className="modal-box">
          <h3 className="font-bold text-lg text-error">
            {t("delete.confirmTitle")}
          </h3>
          <p className="py-4">
            {t("delete.confirmMessage", {
              filename:
                currentAsset?.original_filename || t("delete.thisAsset"),
            })}
          </p>
          <p className="text-sm text-base-content/70 mb-4">
            {t("delete.softDeleteNote")}
          </p>
          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost mr-2" disabled={isDeleting}>
                {t("common.cancel")}
              </button>
              <button
                type="button"
                className={`btn btn-error ${isDeleting ? "loading" : ""}`}
                onClick={handleDeleteAsset}
                disabled={isDeleting}
              >
                {isDeleting ? "" : <Trash2 className="w-4 h-4 mr-2" />}
                {isDeleting ? t("delete.deleting") : t("delete.confirm")}
              </button>
            </form>
          </div>
        </div>
      </dialog>

      {/* Add to Album picker */}
      <dialog id="album_picker_modal" className="modal">
        <div className="modal-box">
          <form method="dialog">
            <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
              <X />
            </button>
          </form>
          <h3 className="font-bold text-lg mb-4">
            {t("assets.assetsPageHeader.addToAlbumModal.title", {
              defaultValue: "Add to Album",
            })}
          </h3>

          {isLoadingAlbums ? (
            <div className="flex justify-center py-12">
              <span className="loading loading-spinner loading-md text-primary" />
            </div>
          ) : albums.length > 0 ? (
            <ul className="menu bg-base-200/50 rounded-box">
              {albums.map((album) => (
                <li key={album.album_id}>
                  <button
                    className="flex items-center gap-3"
                    onClick={() => handleSelectAlbum(album.album_id!)}
                    disabled={isAddingToAlbum}
                  >
                    <div className="size-10 rounded-box overflow-hidden bg-base-300 flex-shrink-0 flex items-center justify-center opacity-40">
                      <Plus size={18} />
                    </div>
                    <div className="flex-1 min-w-0 text-left">
                      <div className="font-medium text-sm truncate">
                        {album.album_name}
                      </div>
                    </div>
                  </button>
                </li>
              ))}
            </ul>
          ) : (
            <div className="text-center py-12 opacity-50">
              <p>
                {t("assets.assetsPageHeader.addToAlbumModal.noAlbumsFound", {
                  defaultValue: "No albums found",
                })}
              </p>
            </div>
          )}

          <div className="modal-action">
            <form method="dialog">
              <button className="btn btn-ghost btn-sm">
                {t("common.cancel")}
              </button>
            </form>
          </div>
        </div>
      </dialog>

      {/* FAB flower */}
      <div className="fab fab-flower">
        {/* Main FAB button */}
        <div tabIndex={0} role="button" className="btn btn-circle btn-lg">
          <Ellipsis />
        </div>
        <div className="fab-close">
          <span className="btn btn-circle btn-lg btn-error">✕</span>
        </div>

        {/* Info toggle */}
        <button
          className={`btn btn-circle btn-lg ${showInfo ? "btn-primary" : ""}`}
          onClick={toggleInfo}
        >
          <Info />
        </button>

        {/* Like / Favorite */}
        <button
          className={`btn btn-circle btn-lg ${
            optimisticLiked ? "text-red-500" : ""
          }`}
          onClick={handleLikeToggle}
        >
          <Heart className={`${optimisticLiked ? "fill-red-500" : ""}`} />
        </button>

        {/* Share / Export */}
        <button
          className="btn btn-circle btn-lg"
          onClick={() =>
            (
              document.getElementById(
                "export_modal",
              ) as HTMLDialogElement | null
            )?.showModal()
          }
        >
          <Share />
        </button>

        {/* Delete */}
        <button
          className="btn btn-circle btn-lg text-error"
          onClick={() => {
            const modal = document.getElementById(
              "delete_confirm_modal",
            ) as HTMLDialogElement;
            modal?.showModal();
          }}
          disabled={isDeleting}
        >
          <Trash2 />
        </button>
      </div>
    </div>
  );
};

export default FullScreenCarousel;
