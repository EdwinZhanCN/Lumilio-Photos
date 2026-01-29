import { useState, useEffect } from "react";
import { X } from "lucide-react";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { assetUrls } from "@/lib/assets/assetUrls";
import { useI18n } from "@/lib/i18n.tsx";
import { ExifDataDisplay } from "@/features/studio/components/panels/ExifDataDisplay";
import type { Asset } from "@/lib/http-commons";
import {
  isPhotoMetadata,
  isVideoMetadata,
  isAudioMetadata,
} from "@/lib/http-commons";
import PhotoInfoView from "./PhotoInfoView";
import VideoInfoView from "./VideoInfoView";
import AudioInfoView from "./AudioInfoView";

interface FullScreenBasicInfoProps {
  asset?: Asset;
  onAssetUpdate?: (updatedAsset: Asset) => void;
}

export default function FullScreenBasicInfo({
  asset,
  onAssetUpdate,
}: FullScreenBasicInfoProps) {
  const [detailedExif, setDetailedExif] = useState<any>(null);
  const [isLoadingFile, setIsLoadingFile] = useState(false);
  const showMessage = useMessage();
  const { isExtracting, exifData, extractExifData } = useExtractExifdata();
  const { t } = useI18n();

  const closeInfo = () => {
    window.dispatchEvent(new CustomEvent("fullscreen:toggleInfo"));
  };

  const handleExtractExif = async () => {
    if (!asset?.asset_id) {
      showMessage("error", t("assets.basicInfo.errors.noAssetId"));
      return;
    }

    try {
      setIsLoadingFile(true);
      setDetailedExif(null);

      // Fetch the original file using the URL helper
      const url = assetUrls.getOriginalFileUrl(asset.asset_id);
      const response = await fetch(url);
      const blob = await response.blob();
      const file = new File([blob], asset.original_filename || "image", {
        type: asset.mime_type || "image/jpeg",
      });

      setIsLoadingFile(false);

      // Extract EXIF data
      await extractExifData([file]);

      // Show EXIF modal
      (
        document.getElementById("exif_modal") as HTMLDialogElement | null
      )?.showModal();
    } catch (error) {
      setIsLoadingFile(false);

      // Handle error - fetch doesn't throw on HTTP errors, so we just show the message
      const errorMessage = error instanceof Error ? error.message : "Unknown error";
      showMessage(
        "error",
        t("assets.basicInfo.errors.extractFailed", {
          message: errorMessage,
        }),
      );
    }
  };

  // Watch for exifData changes and update detailedExif
  useEffect(() => {
    if (exifData && Object.keys(exifData).length > 0) {
      setDetailedExif(exifData[0]);
    }
  }, [exifData]);

  // Return null if no asset
  if (!asset) {
    return null;
  }

  // Determine asset type and render appropriate view
  const assetType = asset.type;
  const metadata = asset.specific_metadata;
  const isLoadingExif = isLoadingFile || isExtracting;

  // Render PhotoInfoView for photos
  if (isPhotoMetadata(assetType, metadata)) {
    return (
      <>
        <PhotoInfoView
          asset={asset}
          onAssetUpdate={onAssetUpdate}
          onClose={closeInfo}
          onExtractExif={handleExtractExif}
          isLoadingExif={isLoadingExif}
        />
        <dialog id="exif_modal" className="modal">
          <div className="modal-box">
            <form method="dialog">
              <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
                <X />
              </button>
            </form>
            <ExifDataDisplay
              exifData={detailedExif}
              isLoading={isLoadingExif}
            />
          </div>
        </dialog>
      </>
    );
  }

  // Render VideoInfoView for videos
  if (isVideoMetadata(assetType, metadata)) {
    return (
      <VideoInfoView
        asset={asset}
        onAssetUpdate={onAssetUpdate}
        onClose={closeInfo}
      />
    );
  }

  // Render AudioInfoView for audio
  if (isAudioMetadata(assetType, metadata)) {
    return (
      <AudioInfoView
        asset={asset}
        onAssetUpdate={onAssetUpdate}
        onClose={closeInfo}
      />
    );
  }

  // Fallback: Generic view for unknown types
  return (
    <div className="absolute top-5 right-5 z-10 font-mono">
      <div className="card bg-base-100 w-max shadow-sm">
        <div className="card-body">
          <div className="card-actions justify-end">
            <h1 className="font-sans font-bold">
              {t("assets.basicInfo.title")}
            </h1>
            <div className="badge badge-soft badge-neutral">{t("assets.fullScreenBasicInfo.unknown_asset_type")}</div>
            <button className="btn btn-circle btn-xs" onClick={closeInfo}>
              <X className="w-4 h-4" />
            </button>
          </div>
          <div className="px-2">
            <p className="text-sm text-base-content/70">
              {asset.original_filename || t("assets.fullScreenBasicInfo.unknown_file")}
            </p>
            <p className="text-xs text-base-content/50 mt-2">
              {asset.mime_type || t("assets.fullScreenBasicInfo.unknown_type")}
            </p>
            {asset.file_size && (
              <p className="text-xs text-base-content/50">
                {(asset.file_size / 1024 / 1024).toFixed(1)}M
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
