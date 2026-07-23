import { useEffect, useMemo, useState, type ReactNode } from "react";
import { createPortal } from "react-dom";
import { Layers, X } from "lucide-react";
import type { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import { useStackCarouselAssets } from "../../../../api/useStackCarouselAssets";
import AssetViewer from "../../../viewer/AssetViewer";

interface StackCarouselOverlayProps {
  asset: Asset;
  focusAssetId?: string;
  open: boolean;
  onClose: () => void;
}

const overlayMessageClasses = "fixed inset-0 z-lightbox isolate flex items-center justify-center bg-black/90";

export default function StackCarouselOverlay({
  asset,
  focusAssetId,
  open,
  onClose,
}: StackCarouselOverlayProps) {
  const { t } = useI18n();
  const { assets, isLoading, error } = useStackCarouselAssets(asset, open);
  const [activeAssetId, setActiveAssetId] = useState<string | undefined>(
    focusAssetId ?? asset.asset_id,
  );

  useEffect(() => {
    if (!open) return;
    setActiveAssetId(focusAssetId ?? asset.asset_id);
  }, [asset.asset_id, focusAssetId, open]);

  const slideIndex = useMemo(() => {
    if (!activeAssetId) return -1;
    return assets.findIndex((item) => item.asset_id === activeAssetId);
  }, [activeAssetId, assets]);

  if (!open) {
    return null;
  }

  const renderOverlay = (content: ReactNode) => {
    if (typeof document === "undefined") return content;
    return createPortal(content, document.body);
  };

  if (isLoading) {
    return renderOverlay(
      <div className={overlayMessageClasses}>
        <button
          type="button"
          className="btn btn-ghost btn-sm absolute left-4 top-2 z-10 text-white"
          onClick={onClose}
          aria-label={t("assets.stackDetail.close", {
            defaultValue: "Close stack details",
          })}
        >
          <X className="size-6" />
        </button>
        <div className="rounded-3xl border border-white/10 bg-white/5 px-8 py-7 text-center text-white shadow-2xl backdrop-blur-sm">
          <div className="mb-4 flex justify-center">
            <span className="loading loading-spinner loading-lg"></span>
          </div>
          <p className="text-lg font-medium">
            {t("assets.stackCarousel.loading", {
              defaultValue: "Loading stack assets...",
            })}
          </p>
        </div>
      </div>,
    );
  }

  if (error || assets.length === 0) {
    return renderOverlay(
      <div className={overlayMessageClasses}>
        <button
          type="button"
          className="btn btn-ghost btn-sm absolute left-4 top-2 z-10 text-white"
          onClick={onClose}
          aria-label={t("assets.stackDetail.close", {
            defaultValue: "Close stack details",
          })}
        >
          <X className="size-6" />
        </button>
        <div className="mx-4 w-full max-w-md rounded-3xl border border-white/10 bg-white/5 p-8 text-center text-white shadow-2xl backdrop-blur-sm">
          <div className="mb-4 flex justify-center">
            <div className="flex size-14 items-center justify-center rounded-2xl border border-white/10 bg-white/10">
              <Layers className="size-6" />
            </div>
          </div>
          <p className="text-lg font-medium">
            {t("assets.stackCarousel.error", {
              defaultValue: "Stack assets are temporarily unavailable.",
            })}
          </p>
          <p className="mt-2 text-sm text-white/70">
            {t("assets.stackCarousel.errorHint", {
              defaultValue: "Try again in a moment.",
            })}
          </p>
          <div className="mt-6">
            <button type="button" className="btn btn-outline btn-sm" onClick={onClose}>
              {t("common.close", {
                defaultValue: "Close",
              })}
            </button>
          </div>
        </div>
      </div>,
    );
  }

  return renderOverlay(
    <AssetViewer
      photos={assets}
      initialSlide={slideIndex >= 0 ? slideIndex : 0}
      slideIndex={slideIndex >= 0 ? slideIndex : undefined}
      onClose={onClose}
      onNavigate={setActiveAssetId}
    />,
  );
}
