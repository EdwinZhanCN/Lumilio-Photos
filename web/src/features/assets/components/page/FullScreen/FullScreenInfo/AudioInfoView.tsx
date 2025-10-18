import { useOptimistic, useTransition } from "react";
import { SquarePen, X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { assetService } from "@/services/assetsService";
import RatingComponent from "@/components/ui/RatingComponent";
import InlineTextEditor from "@/components/ui/InlineTextEditor";
import type { Asset } from "@/lib/http-commons/schema-extensions";
import type { AudioSpecificMetadata } from "@/lib/http-commons/metadata-types";
import { isAudioMetadata } from "@/lib/http-commons/metadata-types";

interface AudioInfoViewProps {
  asset: Asset;
  onAssetUpdate?: (updatedAsset: Asset) => void;
  onClose: () => void;
}

export default function AudioInfoView({
  asset,
  onAssetUpdate,
  onClose,
}: AudioInfoViewProps) {
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

  // Type guard to ensure we have audio metadata
  const metadata = isAudioMetadata(asset.type, optimisticMetadata)
    ? optimisticMetadata
    : ({} as AudioSpecificMetadata);

  const fmt = (v: any, fallback = "-") =>
    v === undefined || v === null || v === "" ? fallback : v;

  // Basic info
  const uploadTime = asset?.upload_time
    ? new Date(asset.upload_time).toLocaleString()
    : undefined;
  const uploadDisplay = fmt(uploadTime);
  const mimeDisplay = fmt(asset?.mime_type);
  const filename = fmt(asset?.original_filename);

  // Audio file info
  const duration = asset?.duration
    ? `${Math.floor(asset.duration / 60)}:${String(Math.floor(asset.duration % 60)).padStart(2, "0")}`
    : "-";
  const sizeM = asset?.file_size
    ? `${(asset.file_size / 1024 / 1024).toFixed(1)}M`
    : "-";

  // Audio technical info
  const codec = fmt(metadata.codec);
  const bitrate = metadata.bitrate
    ? `${(metadata.bitrate / 1000).toFixed(0)} kbps`
    : "-";
  const sampleRate = metadata.sample_rate
    ? `${(metadata.sample_rate / 1000).toFixed(1)} kHz`
    : "-";
  const channels = metadata.channels
    ? metadata.channels === 1
      ? "Mono"
      : metadata.channels === 2
        ? "Stereo"
        : `${metadata.channels} channels`
    : "-";

  // Music metadata
  const title = fmt(metadata.title);
  const artist = fmt(metadata.artist);
  const album = fmt(metadata.album);
  const genre = fmt(metadata.genre);
  const year = fmt(metadata.year);

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
            <div className="badge badge-soft badge-warning">AUDIO</div>
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
              <p>{uploadDisplay}</p>
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

          {/* Music Metadata */}
          {(title !== "-" || artist !== "-" || album !== "-") && (
            <div className="rounded bg-base-300">
              <div className="px-2 py-0.5">
                {title !== "-" && <p className="font-semibold">{title}</p>}
                {artist !== "-" && <p>{artist}</p>}
                {album !== "-" && <p className="text-sm">{album}</p>}
                <div className="flex gap-2 text-xs text-base-content/70">
                  {genre !== "-" && <span>{genre}</span>}
                  {year !== "-" && <span>{year}</span>}
                </div>
              </div>
            </div>
          )}

          {/* Audio Technical Info */}
          <div className="rounded bg-base-300">
            <div className="px-2 py-0.5">
              <p>Codec: {codec}</p>
              <div className="flex gap-2">
                <span>{duration}</span>
                <span>{sizeM}</span>
              </div>
            </div>
            <div className="flex flex-wrap gap-2 p-2">
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {bitrate}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {sampleRate}
              </div>
              <div className="px-2 py-0.5 rounded-full bg-base-100 text-xs">
                {channels}
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
        </div>
      </div>
    </div>
  );
}
