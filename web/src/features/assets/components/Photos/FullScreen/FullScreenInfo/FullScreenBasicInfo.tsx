import { ExifDataDisplay } from "@/features/studio/components/panels/ExifDataDisplay";
import { SquarePen, X } from "lucide-react";
import { useState, useEffect, useOptimistic, useTransition } from "react";
import { useExtractExifdata } from "@/hooks/util-hooks/useExtractExifdata";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { assetService } from "@/services/assetsService";
import { AxiosError } from "axios";
import { useI18n } from "@/lib/i18n.tsx";
import MapComponent from "@/components/MapComponent";
import { assetToPhotoLocation } from "@/lib/utils/mapUtils";
import RatingComponent from "@/components/ui/RatingComponent";
import InlineTextEditor from "@/components/ui/InlineTextEditor";

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
  const [isPending, startTransition] = useTransition();
  const showMessage = useMessage();
  const { isExtracting, exifData, extractExifData } = useExtractExifdata();
  const { t } = useI18n();

  /**
   * React 19 Optimistic Updates Implementation
   *
   * This provides instant UI feedback while maintaining data integrity:
   * 1. User interaction triggers optimistic update (immediate visual feedback)
   * 2. Async API call persists change to database
   * 3. On success: optimistic state becomes real state
   * 4. On failure: React automatically reverts to original database state
   *
   * Benefits:
   * - Instant UI feedback for better UX
   * - Automatic error handling and state reversion
   * - No manual rollback logic needed
   * - Single source of truth maintained
   */
  // Use React 19's useOptimistic for better UX with data integrity
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

  const md = optimisticMetadata;
  const fmt = (v: any, fallback = "-") =>
    v === undefined || v === null || v === "" ? fallback : v;

  // Basic info
  const taken = md.taken_time
    ? new Date(md.taken_time).toLocaleString()
    : asset?.upload_time
      ? new Date(asset.upload_time).toLocaleString()
      : undefined;
  const takenDisplay = fmt(taken);
  const mimeDisplay = fmt(asset?.mime_type);
  const filename = fmt(asset?.original_filename);

  // Dimensions and file info
  const width = asset?.width;
  const height = asset?.height;
  const resolution =
    md.resolution || (width && height ? `${width}✕${height}` : "-");
  const dimensions = md.dimensions || resolution;
  const sizeM = asset?.file_size
    ? `${(asset.file_size / 1024 / 1024).toFixed(1)}M`
    : "-";

  // Camera and lens info
  const cameraModel = fmt(md.camera_model);
  const lensModel = fmt(md.lens_model);

  // Exposure settings
  const iso = fmt(md.iso_speed);
  const exposure = fmt(md.exposure_time);
  const ev = fmt(md.exposure);
  const focal = fmt(md.focal_length ? `${md.focal_length}mm` : md.focal_length);
  const fnumber = fmt(md.f_number ? `f/${md.f_number}` : md.f_number);

  // Additional metadata
  const isRaw = md.is_raw ? "RAW" : "";
  const currentRating = md.rating || 0;
  const closeInfo = () => {
    window.dispatchEvent(new CustomEvent("fullscreen:toggleInfo"));
  };

  const handleRatingChange = (newRating: number) => {
    if (!asset?.asset_id || isPending) return;

    startTransition(async () => {
      // Step 1: Optimistic update for immediate UI feedback
      // User sees the change instantly, even before API call completes
      setOptimisticMetadata({ rating: newRating });

      try {
        // Step 2: Persist the change to database via API
        await assetService.updateAssetRating(asset.asset_id!, newRating);

        // Step 3: Update the source of truth after successful API response
        // This ensures parent components also reflect the database state
        if (onAssetUpdate && asset) {
          const updatedAsset = {
            ...asset,
            specific_metadata: {
              ...asset.specific_metadata,
              rating: newRating,
            },
          };
          onAssetUpdate(updatedAsset);
        }

        showMessage("success", t("rating.updateSuccess"));
      } catch (error) {
        console.error("Failed to update rating:", error);
        showMessage("error", t("rating.updateError"));
        // Step 4: React 19 automatically reverts optimistic update on error
        // No manual rollback needed - UI shows real database state
        // This maintains data integrity without additional code
      }
    });
  };

  const handleDescriptionChange = (newDescription: string) => {
    if (!asset?.asset_id || isPending) return;

    startTransition(async () => {
      // Step 1: Optimistic update for immediate UI feedback
      // Description updates instantly for responsive UX
      setOptimisticMetadata({ description: newDescription });

      try {
        // Step 2: Persist the change to database via API
        await assetService.updateAssetDescription(
          asset.asset_id!,
          newDescription,
        );

        // Step 3: Update the source of truth after successful API response
        // This ensures consistency across all components displaying this asset
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
        // Step 4: React 19 automatically reverts optimistic update on error
        // Description returns to original state, maintaining data integrity
      }
    });
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

  return (
    <div className="absolute top-5 right-5 z-10 font-mono">
      <div className="card bg-base-100 w-max shadow-sm">
        <div className="card-body">
          <div className="card-actions justify-end">
            <h1 className="font-sans font-bold">
              {t("assets.basicInfo.title")}
            </h1>
            <div className="badge badge-soft badge-success">OK</div>
            {/* TODO: Edit Basic Info Functionality, Now disable*/}
            <button className="btn btn-circle btn-xs" disabled>
              <SquarePen className="w-4 h-4" />
            </button>
            <button className="btn btn-circle btn-xs" onClick={closeInfo}>
              <X className="w-4 h-4" />
            </button>
          </div>
          <div className="px-2">
            <div className="flex gap-2 items-center">
              <p>{takenDisplay}</p>
              <div className="text-sm text-info">{mimeDisplay}</div>
              {isRaw && <div className="badge badge-xs badge-warning">RAW</div>}
            </div>
            <div className="flex gap-2 items-center mt-2">
              {/*
                Rating Component with React 19 Optimistic Updates:
                - Shows immediate visual feedback on click
                - Displays optimistic state during API call
                - Automatically reverts on error
                - Disabled during pending state for data integrity
              */}
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
                {ev ? `EV ${ev}` : ev}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {focal}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {fnumber}
              </div>
            </div>
          </div>

          {/* GPS Coordinates */}
          {(md.gps_latitude || md.gps_longitude) && (
            <div className="rounded bg-base-200 p-2">
              <div className="text-xs text-base-content/70 mb-1">
                {t("assets.basicInfo.location")}
              </div>
              <div className="text-xs font-mono">
                {md.gps_latitude &&
                  `${t("assets.basicInfo.latitude")}: ${md.gps_latitude.toFixed(6)}`}
                {md.gps_latitude && md.gps_longitude && " • "}
                {md.gps_longitude &&
                  `${t("assets.basicInfo.longitude")}: ${md.gps_longitude.toFixed(6)}`}
              </div>
            </div>
          )}

          {/* Species Prediction */}
          {md.species_prediction && md.species_prediction.length > 0 && (
            <div className="rounded bg-base-200 p-2">
              <div className="text-xs text-base-content/70 mb-1">
                {t("assets.basicInfo.aiSpeciesDetection")}
              </div>
              <div className="flex flex-wrap gap-1">
                {md.species_prediction.slice(0, 3).map((species, index) => (
                  <div key={index} className="badge badge-xs badge-info">
                    {species.species}{" "}
                    {species.confidence &&
                      `(${(species.confidence * 100).toFixed(0)}%)`}
                  </div>
                ))}
              </div>
            </div>
          )}

          {/* Description - Inline editable with React 19 optimistic updates */}
          <div className="rounded bg-base-200 p-2">
            <div className="text-xs text-base-content/70 mb-1">
              {t("assets.basicInfo.description")}
            </div>
            <InlineTextEditor
              value={md.description || ""}
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

          {/* Map Display */}
          {(md.gps_latitude || md.gps_longitude) && asset && (
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
          <div className="card-actions justify-end font-sans">
            <button
              className="btn btn-sm btn-soft btn-primary"
              onClick={() => {
                handleExtractExif();
                (
                  document.getElementById(
                    "exif_modal",
                  ) as HTMLDialogElement | null
                )?.showModal();
              }}
              disabled={isExtracting || isLoadingFile}
            >
              {isLoadingFile ? (
                <>
                  <span className="loading loading-spinner loading-xs"></span>
                  {t("assets.basicInfo.loadingFile")}
                </>
              ) : isExtracting ? (
                <>
                  <span className="loading loading-spinner loading-xs"></span>
                  {t("assets.basicInfo.extracting")}
                </>
              ) : (
                t("assets.basicInfo.viewExif")
              )}
            </button>
          </div>
          <dialog id="exif_modal" className="modal">
            <div className="modal-box">
              <form method="dialog">
                {/* if there is a button in form, it will close the modal */}
                <button className="btn btn-sm btn-circle btn-ghost absolute right-2 top-2">
                  <X />
                </button>
              </form>
              <ExifDataDisplay
                exifData={detailedExif}
                isLoading={isExtracting || isLoadingFile}
              />
            </div>
          </dialog>
        </div>
      </div>
    </div>
  );
}
