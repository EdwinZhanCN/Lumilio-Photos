import { useState, useEffect, useOptimistic, useTransition } from "react";
import { SquarePen, X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useSettingsContext } from "@/features/settings";
import { assetService } from "@/services/assetsService";
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
  const showMessage = useMessage();
  const { state: settings } = useSettingsContext();
  const [isPending, startTransition] = useTransition();
  const [locationName, setLocationName] = useState<string>("");
  const [isLoadingLocation, setIsLoadingLocation] = useState(false);

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

  // Type guard to ensure we have photo metadata
  const metadata = isPhotoMetadata(asset.type, optimisticMetadata)
    ? optimisticMetadata
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
  const isRaw = metadata.is_raw ? "RAW" : "";
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
      <div className="card bg-base-100 w-max shadow-sm">
        <div className="card-body">
          <div className="card-actions justify-end">
            <h1 className="font-sans font-bold">
              {t("assets.basicInfo.title")}
            </h1>
            <div className="badge badge-soft badge-success">{asset.type}</div>
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
              <p>{takenDisplay}</p>
              <div className="text-sm text-info">{mimeDisplay}</div>
              {isRaw && <div className="badge badge-xs badge-warning">RAW</div>}
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

          {/* Camera & Technical Info */}
          <div className="rounded bg-base-300">
            <div className="px-2 py-0.5">
              <p>{cameraModel}</p>
              <p>{lensModel}</p>
              <div className="flex gap-2">
                <span>{resolution}</span>
                <span>{dimensions}</span>
                <span>{sizeM}</span>
              </div>
            </div>
            <div className="flex flex-wrap gap-2 p-2">
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {"ISO " + iso}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {exposure + "s"}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {ev + "ev"}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {focal}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {fnumber}
              </div>
            </div>
          </div>

          {/* Species Prediction - TODO: Add species_prediction to backend schema */}
          {/* {metadata.species_prediction &&
            metadata.species_prediction.length > 0 && (
              <div className="rounded bg-base-200 p-2">
                <div className="text-xs text-base-content/70 mb-1">
                  {t("assets.basicInfo.aiSpeciesDetection")}
                </div>
                <div className="flex flex-wrap gap-1">
                  {metadata.species_prediction
                    .slice(0, 3)
                    .map((species: any, index: number) => (
                      <div key={index} className="badge badge-xs badge-info">
                        {species.label}{" "}
                        {species.score &&
                          `(${(species.score * 100).toFixed(0)}%)`}
                      </div>
                    ))}
                </div>
              </div>
            )} */}

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

          {/* GPS Coordinates and Location */}
          {(metadata.gps_latitude || metadata.gps_longitude) && (
            <div className="rounded bg-base-200 p-2">
              <div className="text-xs text-base-content/70 mb-1">
                {t("assets.basicInfo.location")}
              </div>

              {/* Location Name */}
              <div className="text-sm mb-2">
                {isLoadingLocation ? (
                  <div className="flex items-center gap-2">
                    <span className="loading loading-spinner loading-xs"></span>
                    <span className="text-base-content/50">
                      {t("assets.basicInfo.loadingLocation")}
                    </span>
                  </div>
                ) : (
                  locationName && (
                    <div className="font-medium">{locationName}</div>
                  )
                )}
              </div>

              {/* GPS Coordinates */}
              <div className="text-xs font-mono text-base-content/70">
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
            <div className="rounded bg-base-200 p-2">
              <div className="text-xs text-base-content/70 mb-2">
                {t("map.photoLocation", { defaultValue: "Photo Location" })}
              </div>
              <div className="h-64 rounded overflow-hidden">
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

          {/* EXIF Button */}
          <div className="card-actions justify-end font-sans">
            <button
              className="btn btn-sm btn-soft btn-primary"
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
