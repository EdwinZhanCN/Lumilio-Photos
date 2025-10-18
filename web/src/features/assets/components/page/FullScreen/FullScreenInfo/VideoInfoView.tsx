import { useOptimistic, useTransition } from "react";
import { SquarePen, X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { assetService } from "@/services/assetsService";
import RatingComponent from "@/components/ui/RatingComponent";
import InlineTextEditor from "@/components/ui/InlineTextEditor";
import type { Asset } from "@/lib/http-commons/schema-extensions";
import type { VideoSpecificMetadata } from "@/lib/http-commons/metadata-types";
import { isVideoMetadata } from "@/lib/http-commons/metadata-types";

interface VideoInfoViewProps {
  asset: Asset;
  onAssetUpdate?: (updatedAsset: Asset) => void;
  onClose: () => void;
}

export default function VideoInfoView({
  asset,
  onAssetUpdate,
  onClose,
}: VideoInfoViewProps) {
  const { t } = useI18n();
  const showMessage = useMessage();
  const [isPending, startTransition] = useTransition();

  // Use React 19's useOptimistic for better UX
  const [optimisticMetadata, setOptimisticMetadata] = useOptimistic(
    asset?.specific_metadata || {},
    (
      currentMetadata,
      optimisticValue: {
        rating?: number;
        description?: string;
      },
    ) => ({
      ...currentMetadata,
      ...optimisticValue,
    }),
  );

  // Type guard to ensure we have video metadata
  const metadata = isVideoMetadata(asset.type, optimisticMetadata)
    ? optimisticMetadata
    : ({} as VideoSpecificMetadata);

  const fmt = (v: any, fallback = "-") =>
    v === undefined || v === null || v === "" ? fallback : v;

  // Basic info
  const recordedTime = metadata.recorded_time
    ? new Date(metadata.recorded_time).toLocaleString()
    : asset?.upload_time
      ? new Date(asset.upload_time).toLocaleString()
      : undefined;
  const recordedDisplay = fmt(recordedTime);
  const mimeDisplay = fmt(asset?.mime_type);
  const filename = fmt(asset?.original_filename);

  // Video dimensions and file info
  const width = asset?.width;
  const height = asset?.height;
  const resolution = width && height ? `${width}✕${height}` : "-";
  const duration = asset?.duration ? `${asset.duration.toFixed(1)}s` : "-";
  const sizeM = asset?.file_size
    ? `${(asset.file_size / 1024 / 1024).toFixed(1)}M`
    : "-";

  // Video technical info
  const codec = fmt(metadata.codec);
  const bitrate = metadata.bitrate
    ? `${(metadata.bitrate / 1000000).toFixed(1)} Mbps`
    : "-";
  const frameRate = metadata.frame_rate
    ? `${metadata.frame_rate.toFixed(0)} fps`
    : "-";
  const cameraModel = fmt(metadata.camera_model);

  // GPS info
  const hasGPS = metadata.gps_latitude && metadata.gps_longitude;
  const gpsDisplay = hasGPS
    ? `${metadata.gps_latitude!.toFixed(4)}, ${metadata.gps_longitude!.toFixed(4)}`
    : null;

  const currentRating = (asset as any).rating || 0;

  const handleRatingChange = (newRating: number) => {
    if (!asset?.asset_id || isPending) return;

    startTransition(async () => {
      setOptimisticMetadata({ rating: newRating });

      try {
        await assetService.updateAssetRating(asset.asset_id!, newRating);

        if (onAssetUpdate && asset) {
          const updatedAsset = {
            ...asset,
            rating: newRating,
          } as Asset;
          onAssetUpdate(updatedAsset);
        }

        showMessage("success", t("rating.updateSuccess"));
      } catch (error) {
        console.error("Failed to update rating:", error);
        showMessage("error", t("rating.updateError"));
      }
    });
  };

  const handleDescriptionChange = (newDescription: string) => {
    if (!asset?.asset_id || isPending) return;

    startTransition(async () => {
      setOptimisticMetadata({ description: newDescription });

      try {
        await assetService.updateAssetDescription(
          asset.asset_id!,
          newDescription,
        );

        if (onAssetUpdate && asset) {
          const updatedAsset = {
            ...asset,
            specific_metadata: {
              ...asset.specific_metadata,
              description: newDescription,
            },
          };
          onAssetUpdate(updatedAsset);
        }

        showMessage("success", t("assets.basicInfo.descriptionUpdated"));
      } catch (error) {
        console.error("Failed to update description:", error);
        showMessage("error", t("assets.basicInfo.descriptionUpdateError"));
      }
    });
  };

  return (
    <div className="absolute top-5 right-5 z-10 font-mono">
      <div className="card bg-base-100 w-max shadow-sm">
        <div className="card-body">
          <div className="card-actions justify-end">
            <h1 className="font-sans font-bold">
              {t("assets.basicInfo.title")}
            </h1>
            <div className="badge badge-soft badge-info">VIDEO</div>
            <button className="btn btn-circle btn-xs" disabled>
              <SquarePen className="w-4 h-4" />
            </button>
            <button className="btn btn-circle btn-xs" onClick={onClose}>
              <X className="w-4 h-4" />
            </button>
          </div>

          {/* Basic Information */}
          <div className="px-2">
            <div className="flex gap-2 items-center">
              <p>{recordedDisplay}</p>
              <div className="text-sm text-info">{mimeDisplay}</div>
            </div>
            <div className="flex gap-2 items-center mt-2">
              <RatingComponent
                rating={currentRating}
                onRatingChange={handleRatingChange}
                disabled={isPending}
                size="sm"
                showUnratedButton={true}
              />
            </div>
            <div className="flex">
              <p>{filename}</p>
            </div>
          </div>

          {/* Video Technical Info */}
          <div className="rounded bg-base-300">
            <div className="px-2 py-0.5">
              {cameraModel !== "-" && <p>{cameraModel}</p>}
              <p>Codec: {codec}</p>
              <div className="flex gap-2">
                <span>{resolution}</span>
                <span>{duration}</span>
                <span>{sizeM}</span>
              </div>
            </div>
            <div className="flex flex-wrap gap-2 p-2">
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {bitrate}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {frameRate}
              </div>
            </div>
          </div>

          {/* Description */}
          <div className="rounded bg-base-200 p-2">
            <div className="text-xs text-base-content/70 mb-1">
              {t("assets.basicInfo.description")}
            </div>
            <InlineTextEditor
              value={metadata.description || ""}
              onSave={handleDescriptionChange}
              placeholder={t("assets.basicInfo.descriptionPlaceholder")}
              emptyStateText={t("assets.basicInfo.noDescription")}
              editHint={t("assets.basicInfo.clickToEdit")}
              disabled={isPending}
              saving={isPending}
              multiline={true}
              maxLength={500}
              className="min-h-[1.5rem]"
            />
          </div>

          {/* GPS Location (if available) */}
          {hasGPS && (
            <div className="rounded bg-base-200 p-2">
              <div className="text-xs text-base-content/70 mb-1">
                Recording Location
              </div>
              <div className="text-xs font-mono text-base-content/70">
                {gpsDisplay}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
