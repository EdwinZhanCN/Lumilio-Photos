import { useState, useEffect, useOptimistic, useTransition } from "react";
import { SquarePen, X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useSettingsContext } from "@/features/settings";
import { useAssetActions } from "@/features/assets/hooks/useAssetActions";
import { geoService } from "@/services/geoService";
import RatingComponent from "@/components/ui/RatingComponent";
import InlineTextEditor from "@/components/ui/InlineTextEditor";
import MapComponent from "@/components/MapComponent";
import { assetToPhotoLocation } from "@/lib/utils/mapUtils";
import type { Asset, PhotoSpecificMetadata } from "@/lib/http-commons";
import { isPhotoMetadata } from "@/lib/http-commons";

interface PhotoInfoViewProps {
  asset: Asset;
  onAssetUpdate?: (updatedAsset: Asset) => void;
  onClose: () => void;
  onExtractExif: () => void;
  isLoadingExif: boolean;
}

export default function PhotoInfoView({
  asset,
  onAssetUpdate,
  onClose,
  onExtractExif,
  isLoadingExif,
}: PhotoInfoViewProps) {
  const { t } = useI18n();
  const { state: settings } = useSettingsContext();
  const [isPending, startTransition] = useTransition();
  const [locationName, setLocationName] = useState<string>("");
  const [isLoadingLocation, setIsLoadingLocation] = useState(false);
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

  // Type guard to ensure we have photo metadata
  const metadata = isPhotoMetadata(asset.type, asset.specific_metadata)
    ? (asset.specific_metadata as PhotoSpecificMetadata)
    : ({} as PhotoSpecificMetadata);

  const fmt = (v: any, fallback = "-") =>
    v === undefined || v === null || v === "" ? fallback : v;

  // Basic info
  const takenTime = metadata.taken_time
    ? new Date(metadata.taken_time).toLocaleString()
    : asset?.upload_time
      ? new Date(asset.upload_time).toLocaleString()
      : undefined;
  const takenDisplay = fmt(takenTime);
  const mimeDisplay = fmt(asset?.mime_type);
  const filename = fmt(asset?.original_filename);

  // Dimensions and file info
  const width = asset?.width;
  const height = asset?.height;
  const resolution =
    metadata.resolution || (width && height ? `${width}✕${height}` : "-");
  const dimensions = metadata.dimensions || resolution;
  const sizeM = asset?.file_size
    ? `${(asset.file_size / 1024 / 1024).toFixed(1)}M`
    : "-";

  // Camera and lens info
  const cameraModel = fmt(metadata.camera_model);
  const lensModel = fmt(metadata.lens_model);

  // Exposure settings
  const iso = fmt(metadata.iso_speed);
  const exposure = fmt(metadata.exposure_time);
  const ev = fmt(metadata.exposure);
  const focal = fmt(
    metadata.focal_length
      ? `${metadata.focal_length}mm`
      : metadata.focal_length,
  );
  const fnumber = fmt(
    metadata.f_number ? `f/${metadata.f_number}` : metadata.f_number,
  );

  // Additional metadata
  const isRaw = metadata.is_raw ? t("assets.photoInfoView.raw_badge") : "";
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

  // Fetch location name when GPS coordinates are available
  useEffect(() => {
    if (metadata.gps_latitude && metadata.gps_longitude) {
      setIsLoadingLocation(true);
      const region = settings.ui.region || "other";
      const language = settings.ui.language || "en";

      geoService
        .reverseGeocode(
          metadata.gps_latitude,
          metadata.gps_longitude,
          region,
          language,
        )
        .then((name) => {
          setLocationName(name);
        })
        .catch((error) => {
          console.error("Failed to get location name:", error);
          setLocationName(
            `${metadata.gps_latitude!.toFixed(4)}, ${metadata.gps_longitude!.toFixed(4)}`,
          );
        })
        .finally(() => {
          setIsLoadingLocation(false);
        });
    }
  }, [
    metadata.gps_latitude,
    metadata.gps_longitude,
    settings.ui.region,
    settings.ui.language,
  ]);

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
              <div className="badge badge-soft badge-success">{asset.type}</div>
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
                <p className="text-sm">{takenDisplay}</p>
                <div className="text-xs text-info">{mimeDisplay}</div>
                {isRaw && <div className="badge badge-xs badge-warning">{isRaw}</div>}
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

            {/* Camera & Technical Info */}
            <div className="rounded bg-base-300 overflow-hidden">
              <div className="px-3 py-2 space-y-1">
                <p className="text-sm font-medium">{cameraModel}</p>
                <p className="text-xs opacity-70">{lensModel}</p>
                <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs opacity-60">
                  <span>{resolution}</span>
                  <span>{dimensions}</span>
                  <span>{sizeM}</span>
                </div>
              </div>
              <div className="flex flex-wrap gap-2 p-3 pt-0">
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {t("assets.photoInfoView.iso_prefix") + iso}
                </div>
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {exposure + t("assets.photoInfoView.exposure_suffix")}
                </div>
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {ev + t("assets.photoInfoView.ev_suffix")}
                </div>
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {focal}
                </div>
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {fnumber}
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

            {/* GPS Coordinates and Location */}
            {(metadata.gps_latitude || metadata.gps_longitude) && (
              <div className="rounded bg-base-200 p-3">
                <div className="text-[10px] uppercase tracking-wider font-bold text-base-content/50 mb-2">
                  {t("assets.basicInfo.location")}
                </div>

                {/* Location Name */}
                <div className="text-sm mb-2">
                  {isLoadingLocation ? (
                    <div className="flex items-center gap-2">
                      <span className="loading loading-spinner loading-xs"></span>
                      <span className="text-base-content/50 text-xs">
                        {t("assets.basicInfo.loadingLocation")}
                      </span>
                    </div>
                  ) : (
                    locationName && (
                      <div className="font-medium leading-tight">{locationName}</div>
                    )
                  )}
                </div>

                {/* GPS Coordinates */}
                <div className="text-[10px] font-mono text-base-content/50">
                  {metadata.gps_latitude &&
                    `${t("assets.basicInfo.latitude")}: ${metadata.gps_latitude.toFixed(6)}`}
                  {metadata.gps_latitude && metadata.gps_longitude && " • "}
                  {metadata.gps_longitude &&
                    `${t("assets.basicInfo.longitude")}: ${metadata.gps_longitude.toFixed(6)}`}
                </div>
              </div>
            )}

            {/* Map Display */}
            {(metadata.gps_latitude || metadata.gps_longitude) && asset && (
              <div className="rounded bg-base-200 p-1">
                <div className="h-48 rounded overflow-hidden">
                  {(() => {
                    const photoLocation = assetToPhotoLocation(asset);
                    return photoLocation ? (
                      <MapComponent
                        photoLocations={[photoLocation]}
                        showSinglePhoto={true}
                        height="100%"
                        zoom={15}
                      />
                    ) : null;
                  })()}
                </div>
              </div>
            )}
          </div>

          {/* Footer - Fixed */}
          <div className="p-4 border-t border-base-200 bg-base-100/50 backdrop-blur-sm">
            <button
              className="btn btn-sm btn-block btn-soft btn-primary font-sans"
              onClick={onExtractExif}
              disabled={isLoadingExif || isPending}
            >
              {isLoadingExif ? (
                <>
                  <span className="loading loading-spinner loading-xs"></span>
                  {t("assets.basicInfo.extracting")}
                </>
              ) : (
                t("assets.basicInfo.viewExif")
              )}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
