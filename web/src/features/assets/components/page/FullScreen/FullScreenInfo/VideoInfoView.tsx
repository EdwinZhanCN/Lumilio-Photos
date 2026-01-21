import { useOptimistic, useTransition } from "react";
import { SquarePen, X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useAssetActions } from "@/features/assets/hooks/useAssetActions";
import RatingComponent from "@/components/ui/RatingComponent";
import InlineTextEditor from "@/components/ui/InlineTextEditor";
import type { Asset, VideoSpecificMetadata } from "@/lib/http-commons";
import { isVideoMetadata } from "@/lib/http-commons";

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
  const [isPending, startTransition] = useTransition();
  const { updateRating, updateDescription } = useAssetActions();

  // Use React 19's useOptimistic for better UX
  const [optimisticRating, setOptimisticRating] = useOptimistic(
    asset?.rating || 0,
    (_, v: number) => v,
  );

  const [optimisticDescription, setOptimisticDescription] = useOptimistic(
    asset?.specific_metadata?.description || "",
    (_, v: string) => v,
  );

  // Type guard to ensure we have video metadata
  const metadata = isVideoMetadata(asset.type, asset.specific_metadata)
    ? (asset.specific_metadata as VideoSpecificMetadata)
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

  const currentRating = optimisticRating;

  const handleRatingChange = (newRating: number) => {
    if (!asset?.asset_id || isPending) return;

    startTransition(async () => {
      setOptimisticRating(newRating);

      try {
        await updateRating(asset.asset_id!, newRating);

        if (onAssetUpdate && asset) {
          const updatedAsset = {
            ...asset,
            rating: newRating,
          } as Asset;
          onAssetUpdate(updatedAsset);
        }
      } catch (error) {
        console.error("Failed to update rating:", error);
      }
    });
  };

  const handleDescriptionChange = (newDescription: string) => {
    if (!asset?.asset_id || isPending) return;

    startTransition(async () => {
      setOptimisticDescription(newDescription);

      try {
        await updateDescription(asset.asset_id!, newDescription);

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
      } catch (error) {
        console.error("Failed to update description:", error);
      }
    });
  };

  return (
    <div className="absolute top-5 right-5 z-10 font-mono">
      <div className="card bg-base-100 w-[380px] max-h-[calc(100vh-40px)] shadow-sm overflow-hidden flex flex-col">
        <div className="card-body p-0 flex flex-col overflow-hidden">
          {/* Header - Fixed */}
          <div className="p-4 pb-2 flex items-center justify-between border-b border-base-200">
            <div className="flex items-center gap-2">
              <h1 className="font-sans font-bold">
                {t("assets.basicInfo.title")}
              </h1>
              <div className="badge badge-soft badge-info">VIDEO</div>
            </div>
            <div className="flex gap-1">
              <button className="btn btn-circle btn-xs" disabled>
                <SquarePen className="w-4 h-4" />
              </button>
              <button className="btn btn-circle btn-xs" onClick={onClose}>
                <X className="w-4 h-4" />
              </button>
            </div>
          </div>

          {/* Scrollable Content */}
          <div className="flex-1 overflow-y-auto p-4 space-y-4 custom-scrollbar">
            {/* Basic Information */}
            <div className="px-2">
              <div className="flex flex-wrap gap-2 items-center">
                <p className="text-sm">{recordedDisplay}</p>
                <div className="text-xs text-info">{mimeDisplay}</div>
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
              <div className="mt-2">
                <p className="text-sm break-all font-medium">{filename}</p>
              </div>
            </div>

            {/* Video Technical Info */}
            <div className="rounded bg-base-300 overflow-hidden">
              <div className="px-3 py-2 space-y-1">
                {cameraModel !== "-" && <p className="text-sm font-medium">{cameraModel}</p>}
                <p className="text-xs opacity-70">Codec: {codec}</p>
                <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs opacity-60">
                  <span>{resolution}</span>
                  <span>{duration}</span>
                  <span>{sizeM}</span>
                </div>
              </div>
              <div className="flex flex-wrap gap-2 p-3 pt-0">
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {bitrate}
                </div>
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {frameRate}
                </div>
              </div>
            </div>

            {/* Description */}
            <div className="rounded bg-base-200 p-3">
              <div className="text-[10px] uppercase tracking-wider font-bold text-base-content/50 mb-2">
                {t("assets.basicInfo.description")}
              </div>
              <InlineTextEditor
                value={optimisticDescription || ""}
                onSave={handleDescriptionChange}
                placeholder={t("assets.basicInfo.descriptionPlaceholder")}
                emptyStateText={t("assets.basicInfo.noDescription")}
                editHint={t("assets.basicInfo.clickToEdit")}
                disabled={isPending}
                saving={isPending}
                multiline={true}
                maxLength={500}
                className="text-sm min-h-[1.5rem]"
              />
            </div>

            {/* GPS Location (if available) */}
            {hasGPS && (
              <div className="rounded bg-base-200 p-3">
                <div className="text-[10px] uppercase tracking-wider font-bold text-base-content/50 mb-2">
                  Recording Location
                </div>
                <div className="text-[10px] font-mono text-base-content/50">
                  {gpsDisplay}
                </div>
              </div>
            )}
          </div>
        </div>
      </div>
    </div>
  );
}
