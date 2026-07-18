import { X } from "lucide-react";
import type { UseQueryResult } from "@tanstack/react-query";
import { useMessage } from "@/features/notifications";
import { useI18n } from "@/lib/i18n.tsx";
import { $api } from "@/lib/http-commons/queryClient";
import type { components } from "@/lib/http-commons/schema.d.ts";
import { ExifDataDisplay } from "./ExifDataDisplay";
import type { Asset } from "@/lib/http-commons";
import { isPhotoMetadata, isVideoMetadata, isAudioMetadata } from "@/lib/http-commons";
import PhotoInfoView from "./PhotoInfoView";
import VideoInfoView from "./VideoInfoView";
import AudioInfoView from "./AudioInfoView";

type Schemas = components["schemas"];
type AssetExifResponse = Schemas["dto.AssetExifResponseDTO"];

interface FullScreenBasicInfoProps {
  asset?: Asset;
  onAssetUpdate?: (updatedAsset: Asset) => void;
}

export default function FullScreenBasicInfo({ asset, onAssetUpdate }: FullScreenBasicInfoProps) {
  const showMessage = useMessage();
  const { t } = useI18n();

  const exifQuery = $api.useQuery(
    "get",
    "/api/v1/assets/{id}/exif",
    {
      params: { path: { id: asset?.asset_id ?? "" } },
    },
    {
      enabled: false,
      retry: 1,
    },
  ) as UseQueryResult<AssetExifResponse, unknown>;

  const rawExif = (exifQuery.data?.exif_raw as Record<string, unknown> | undefined) ?? null;
  const rawExifForDisplay =
    rawExif && Object.keys(rawExif).length > 0 ? (rawExif as Record<string, any>) : null;

  const closeInfo = () => {
    window.dispatchEvent(new CustomEvent("fullscreen:toggleInfo"));
  };

  const handleViewExif = async () => {
    if (!asset?.asset_id) {
      showMessage("error", t("assets.basicInfo.errors.noAssetId"));
      return;
    }

    (document.getElementById("exif_modal") as HTMLDialogElement | null)?.showModal();

    const result = await exifQuery.refetch();

    if (result.isError) {
      const message =
        result.error instanceof Error
          ? result.error.message
          : t("assets.basicInfo.errors.extractFailed", {
              message: String(result.error),
            });
      showMessage("error", message);
      return;
    }

    const payload = result.data;
    const exifRaw = payload?.exif_raw as Record<string, unknown> | undefined;
    if (!exifRaw || Object.keys(exifRaw).length === 0) {
      showMessage("info", t("assets.basicInfo.exifNotAvailable"));
    }
  };

  const isLoadingExif = exifQuery.isFetching && !rawExifForDisplay;

  // Return null if no asset
  if (!asset) {
    return null;
  }

  // Determine asset type and render appropriate view
  const assetType = asset.type;
  const metadata = asset.specific_metadata;

  // Render PhotoInfoView for photos
  if (isPhotoMetadata(assetType, metadata)) {
    return (
      <>
        <PhotoInfoView
          asset={asset}
          onAssetUpdate={onAssetUpdate}
          onClose={closeInfo}
          onExtractExif={handleViewExif}
          isLoadingExif={isLoadingExif}
        />
        <dialog id="exif_modal" className="modal">
          <div className="modal-box">
            <form method="dialog">
              <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
                <X />
              </button>
            </form>
            <ExifDataDisplay exifData={rawExifForDisplay} isLoading={isLoadingExif} />
          </div>
        </dialog>
      </>
    );
  }

  // Render VideoInfoView for videos
  if (isVideoMetadata(assetType, metadata)) {
    return <VideoInfoView asset={asset} onAssetUpdate={onAssetUpdate} onClose={closeInfo} />;
  }

  // Render AudioInfoView for audio
  if (isAudioMetadata(assetType, metadata)) {
    return <AudioInfoView asset={asset} onAssetUpdate={onAssetUpdate} onClose={closeInfo} />;
  }

  // Fallback: Generic view for unknown types
  return (
    <div className="absolute top-5 right-5 z-10 max-w-[calc(100vw-2.5rem)] font-mono">
      <div className="card bg-base-100 w-max max-w-full shadow-sm">
        <div className="card-body">
          <div className="card-actions justify-end">
            <h1 className="font-sans font-bold">{t("assets.basicInfo.title")}</h1>
            <div className="badge badge-soft badge-neutral">
              {t("assets.fullScreenBasicInfo.unknown_asset_type")}
            </div>
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
