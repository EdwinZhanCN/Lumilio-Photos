import React from "react";
import { Play, Music, Video, Headphones, Check } from "lucide-react";
import {
  isVideo,
  isAudio,
  formatDuration,
  getAssetAriaLabel,
} from "@/lib/utils/mediaTypes";
import { Asset } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";

interface MediaThumbnailProps {
  asset: Asset;
  thumbnailUrl?: string;
  className?: string;
  onClick?: (e: React.MouseEvent) => void;
  isSelected?: boolean;
  isSelectionMode?: boolean;
}

/**
 * MediaThumbnail component that renders appropriate thumbnail based on asset type
 */
const MediaThumbnail: React.FC<MediaThumbnailProps> = ({
  asset,
  thumbnailUrl,
  className = "",
  onClick,
  isSelected = false,
  isSelectionMode = false,
}) => {
  const { t } = useI18n();
  const videoAsset = isVideo(asset);
  const audioAsset = isAudio(asset);
  const duration = asset.duration;
  const ariaLabel = getAssetAriaLabel(asset);

  const selectionTint = isSelectionMode ? (
    <div
      className={`pointer-events-none absolute inset-0 z-10 transition-colors duration-200 ${
        isSelected
          ? "bg-primary/14"
          : "bg-gradient-to-b from-black/20 via-transparent to-black/10"
      }`}
    />
  ) : null;

  const selectionOverlay = isSelectionMode && (
    <div className="absolute right-3 top-3 z-20">
      <div
        className={`flex size-8 items-center justify-center rounded-full border backdrop-blur-md transition-all duration-200 ${
          isSelected
            ? "border-primary/70 bg-primary text-primary-content shadow-lg shadow-primary/25"
            : "border-white/30 bg-black/35 text-white/75 shadow-lg shadow-black/20"
        }`}
      >
        {isSelected ? (
          <Check className="size-4" strokeWidth={3} />
        ) : (
          <div className="size-3 rounded-full border border-current/80" />
        )}
      </div>
    </div>
  );

  const selectionClass = isSelected
    ? "ring-2 ring-primary/80 ring-inset shadow-[0_24px_50px_-28px_rgba(59,130,246,0.55)]"
    : isSelectionMode
      ? "ring-1 ring-black/10 ring-inset shadow-[0_16px_40px_-30px_rgba(15,23,42,0.5)]"
      : "shadow-[0_16px_40px_-30px_rgba(15,23,42,0.45)]";

  // Photo or thumbnail available - render image with potential overlays
  if (thumbnailUrl && !audioAsset) {
    return (
      <div
        className={`group relative h-full w-full cursor-pointer overflow-hidden border border-base-100/10 bg-base-200/40 transition-all duration-200 ease-out ${selectionClass} ${className}`}
        onClick={onClick}
        role="button"
        tabIndex={0}
        aria-label={ariaLabel}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onClick?.(e as any);
          }
        }}
      >
        {selectionOverlay}
        {selectionTint}
        <img
          src={thumbnailUrl}
          alt={
            asset.original_filename || t("assets.mediaThumbnail.asset_alt_text")
          }
          className={`h-full w-full object-cover transition-transform duration-300 ease-out ${
            isSelectionMode || isSelected ? "" : "group-hover:scale-[1.03]"
          }`}
          loading="lazy"
        />

        {/* Media type indicator badge */}
        {videoAsset && (
          <div className="absolute left-3 top-3 z-20">
            <div className="flex items-center gap-1 rounded-full border border-white/15 bg-black/55 px-2.5 py-1 text-xs text-white backdrop-blur-sm">
              <Video className="w-3 h-3" />
              <span className="sr-only">
                {t("assets.mediaThumbnail.video_sr_only")}
              </span>
            </div>
          </div>
        )}

        {/* Video play overlay */}
        {videoAsset && !isSelectionMode && (
          <div className="absolute inset-0 z-10 flex items-center justify-center">
            <div
              className="rounded-full border border-white/10 bg-black/55 p-3 shadow-xl backdrop-blur-sm transition-colors group-hover:bg-black/65"
              aria-hidden="true"
            >
              <Play className="w-8 h-8 text-white fill-white ml-1" />
            </div>
          </div>
        )}

        {/* Duration badge for videos */}
        {videoAsset && duration && (
          <div className="absolute bottom-3 right-3 z-20 rounded-full border border-white/10 bg-black/65 px-2.5 py-1 text-xs text-white shadow-lg backdrop-blur-sm">
            {formatDuration(duration)}
          </div>
        )}

        {isSelected && (
          <div className="pointer-events-none absolute inset-0 z-10 bg-gradient-to-br from-primary/14 via-primary/8 to-transparent" />
        )}
      </div>
    );
  }

  // Audio asset - render audio-specific visualization
  if (audioAsset) {
    return (
      <div
        className={`relative flex h-full w-full cursor-pointer flex-col items-center justify-center gap-2 overflow-hidden border border-base-100/10 bg-gradient-to-br from-slate-900 via-slate-800 to-slate-700 px-4 py-3 text-center text-white transition-all duration-200 ease-out ${selectionClass} ${className}`}
        onClick={onClick}
        role="button"
        tabIndex={0}
        aria-label={ariaLabel}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onClick?.(e as any);
          }
        }}
      >
        {selectionOverlay}
        {selectionTint}
        {/* Audio type indicator */}
        <div className="absolute left-3 top-3 z-20">
          <div className="flex items-center gap-1 rounded-full border border-white/15 bg-black/25 px-2.5 py-1 text-xs text-white/90 backdrop-blur-sm">
            <Headphones className="w-3 h-3" />
            <span className="sr-only">
              {t("assets.mediaThumbnail.audio_sr_only")}
            </span>
          </div>
        </div>

        <div className="rounded-full border border-white/10 bg-white/10 p-4 shadow-lg backdrop-blur-sm">
          <Music className="h-10 w-10" aria-hidden="true" />
        </div>
        <div className="text-center px-2">
          <div className="text-sm font-medium truncate max-w-full">
            {asset.original_filename?.replace(/\.[^/.]+$/, "") ||
              t("assets.mediaThumbnail.audio_file_fallback")}
          </div>
          {duration && (
            <div className="text-xs opacity-80 mt-1">
              {formatDuration(duration)}
            </div>
          )}
        </div>
        {isSelected && (
          <div className="pointer-events-none absolute inset-0 z-10 bg-gradient-to-br from-primary/16 via-primary/8 to-transparent" />
        )}
      </div>
    );
  }

  // Fallback for no preview available
  return (
    <div
      className={`relative flex h-full w-full cursor-pointer items-center justify-center border border-base-300/80 bg-base-200 text-base-content/50 transition-all duration-200 ease-out hover:bg-base-100 ${selectionClass} ${className}`}
      onClick={onClick}
      role="button"
      tabIndex={0}
      aria-label={`${t("assets.mediaThumbnail.asset_aria_label_prefix")}: ${asset.original_filename || t("assets.mediaThumbnail.unknown_file_fallback")}`}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick?.(e as any);
        }
      }}
    >
      {selectionOverlay}
      {selectionTint}
      <div className="text-center">
        <div className="text-xs">{t("assets.mediaThumbnail.no_preview")}</div>
        <div className="text-xs opacity-60">
          {asset.mime_type ||
            asset.type ||
            t("assets.mediaThumbnail.unknown_mime_fallback")}
        </div>
      </div>
      {isSelected && (
        <div className="pointer-events-none absolute inset-0 z-10 bg-primary/10" />
      )}
    </div>
  );
};

export default MediaThumbnail;
