import { useOptimistic, useTransition } from "react";
import { SquarePen, X } from "lucide-react";
import { useI18n } from "@/lib/i18n.tsx";
import { useMessage } from "@/hooks/util-hooks/useMessage";
import { useAssetActions } from "@/features/assets/hooks/useAssetActions";
import RatingComponent from "@/components/ui/RatingComponent";
import InlineTextEditor from "@/components/ui/InlineTextEditor";
import type { Asset, AudioSpecificMetadata } from "@/lib/http-commons";
import { isAudioMetadata } from "@/lib/http-commons";

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

  // Type guard to ensure we have audio metadata
  const metadata = isAudioMetadata(asset.type, asset.specific_metadata)
    ? (asset.specific_metadata as AudioSpecificMetadata)
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
              <div className="badge badge-soft badge-warning">AUDIO</div>
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
                <p className="text-sm">{uploadDisplay}</p>
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

            {/* Music Metadata */}
            {(title !== "-" || artist !== "-" || album !== "-") && (
              <div className="rounded bg-base-300 overflow-hidden">
                <div className="px-3 py-2 space-y-1">
                  {title !== "-" && <p className="text-sm font-bold leading-tight">{title}</p>}
                  {artist !== "-" && <p className="text-xs font-medium">{artist}</p>}
                  {album !== "-" && <p className="text-xs opacity-70">{album}</p>}
                  <div className="flex flex-wrap gap-x-3 gap-y-1 text-[10px] opacity-60 uppercase tracking-wider">
                    {genre !== "-" && <span>{genre}</span>}
                    {year !== "-" && <span>{year}</span>}
                  </div>
                </div>
              </div>
            )}

            {/* Audio Technical Info */}
            <div className="rounded bg-base-300 overflow-hidden">
              <div className="px-3 py-2 space-y-1">
                <p className="text-xs opacity-70">Codec: {codec}</p>
                <div className="flex flex-wrap gap-x-3 gap-y-1 text-xs opacity-60">
                  <span>{duration}</span>
                  <span>{sizeM}</span>
                </div>
              </div>
              <div className="flex flex-wrap gap-2 p-3 pt-0">
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {bitrate}
                </div>
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {sampleRate}
                </div>
                <div className="px-2 py-0.5 rounded-full bg-base-100 text-[10px] font-bold">
                  {channels}
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
          </div>
        </div>
      </div>
    </div>
  );
}
