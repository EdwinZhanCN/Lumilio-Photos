import { useMemo, useRef, useEffect, useState } from "react";
import { Swiper, SwiperSlide } from "swiper/react";
import { Virtual, Navigation, Pagination } from "swiper/modules";
import { XMarkIcon } from "@heroicons/react/24/outline";
import { Ellipsis, Info, Share, Heart, Trash2 } from "lucide-react";
import ExportModal from "@/components/ExportModal";
import "@/styles/custom-swiper.css";
import "swiper/css";
import "swiper/css/navigation";
import "swiper/css/pagination";
import { assetService } from "@/services/assetsService";
import FullScreenBasicInfo from "../FullScreenInfo/FullScreenBasicInfo";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useI18n } from "@/lib/i18n.tsx";
import MediaViewer from "../../../shared/MediaViewer";
import type { Asset } from "@/lib/http-commons/schema-extensions";

interface FullScreenCarouselProps {
  photos: Asset[];
  initialSlide: number;
  onClose: () => void;
  onNavigate: (assetId: string) => void;
  onAssetUpdate?: (updatedAsset: Asset) => void;
  onAssetDelete?: (deletedAssetId: string) => void;
}

const FullScreenCarousel = ({
  photos,
  initialSlide,
  onClose,
  onNavigate,
  onAssetUpdate,
  onAssetDelete,
}: FullScreenCarouselProps) => {
  const swiperRef = useRef<any>(null),
    closingRef = useRef(false);
  const [showInfo, setShowInfo] = useState(false);
  const [currentAsset, setCurrentAsset] = useState(photos[initialSlide]);
  const [isDeleting, setIsDeleting] = useState(false);
  const showMessage = useMessage();
  const { t } = useI18n();

  const slides = useMemo(() => {
    return photos.map((photo) => ({
      asset: photo,
      assetId: photo.asset_id!,
    }));
  }, [photos]);

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
  const handleClose = () => {
    closingRef.current = true;
    onClose();
  };
  //

  useEffect(() => {
    const handler = () => setShowInfo((s) => !s);
    window.addEventListener("fullscreen:toggleInfo", handler);
    return () => window.removeEventListener("fullscreen:toggleInfo", handler);
  }, []);

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

  const handleLikeToggle = async () => {
    if (!currentAsset?.asset_id) return;

    const currentLiked = currentAsset.liked || false;
    const newLiked = !currentLiked;

    try {
      // Call API to persist the change
      await assetService.updateAssetLike(currentAsset.asset_id, newLiked);

      // Update the actual asset state after successful API call
      const updatedAsset = {
        ...currentAsset,
        liked: newLiked,
        specific_metadata: {
          ...currentAsset.specific_metadata,
        },
      };

      handleAssetUpdate(updatedAsset);
      showMessage("success", t("rating.updateSuccess"));
    } catch (error) {
      console.error("Failed to update like status:", error);
      showMessage("error", t("rating.updateError"));
    }
  };

  const handleDeleteAsset = async () => {
    if (!currentAsset?.asset_id || isDeleting) return;

    setIsDeleting(true);
    try {
      await assetService.deleteAsset(currentAsset.asset_id);

      showMessage("success", t("delete.success"));

      // Notify parent about deletion
      if (onAssetDelete) {
        onAssetDelete(currentAsset.asset_id);
      }

      // Close the carousel
      handleClose();
    } catch (error) {
      console.error("Failed to delete asset:", error);
      showMessage("error", t("delete.error"));
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
    <div className="fixed inset-0 bg-black/90 z-50 flex items-center justify-center animate-fade-in">
      <button
        onClick={handleClose}
        className="btn btn-ghost btn-sm absolute top-2 left-4 text-white z-20"
      >
        <XMarkIcon className="w-6 h-6" />
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
        initialSlide={initialSlide}
        className="fullscreen-swiper"
      >
        {slides.map((slide, index) => (
          <SwiperSlide key={slide.assetId} virtualIndex={index}>
            <MediaViewer asset={slide.asset} />
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

      {/* Export modal */}
      <ExportModal asset={currentAsset} />

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
            currentAsset?.liked ? "text-red-500" : ""
          }`}
          onClick={handleLikeToggle}
        >
          <Heart className={`${currentAsset?.liked ? "fill-red-500" : ""}`} />
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
