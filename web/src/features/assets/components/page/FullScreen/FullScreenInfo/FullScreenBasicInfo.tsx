import { useState, useEffect } from "react";
import { X } from "lucide-react";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { assetService } from "@/services/assetsService";
import { AxiosError } from "axios";
import { useI18n } from "@/lib/i18n.tsx";
import { ExifDataDisplay } from "@/features/studio/components/panels/ExifDataDisplay";
import type { Asset } from "@/lib/http-commons/schema-extensions";
import {
  isPhotoMetadata,
  isVideoMetadata,
  isAudioMetadata,
} from "@/lib/http-commons/metadata-types";
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

      // Fetch the original file using the service method
      const response = await assetService.getOriginalFile(asset.asset_id);
      const blob = response.data;
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

      // Handle specific error cases
      if ((error as AxiosError)?.response?.status === 404) {
        showMessage("error", t("assets.basicInfo.errors.notFound"));
      } else if ((error as AxiosError)?.response?.status === 401) {
        showMessage("error", t("assets.basicInfo.errors.unauthorized"));
      } else if ((error as AxiosError)?.response?.status === 403) {
        showMessage("error", t("assets.basicInfo.errors.forbidden"));
      } else {
        showMessage(
          "error",
          t("assets.basicInfo.errors.extractFailed", {
            message: (error as Error).message,
          }),
        );
      }
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
            <div className="badge badge-soft badge-neutral">UNKNOWN</div>
            <button className="btn btn-circle btn-xs" onClick={closeInfo}>
              <X className="w-4 h-4" />
            </button>
          </div>
          <div className="px-2">
            <p className="text-sm text-base-content/70">
              {asset.original_filename || "Unknown file"}
            </p>
            <p className="text-xs text-base-content/50 mt-2">
              {asset.mime_type || "Unknown type"}
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
