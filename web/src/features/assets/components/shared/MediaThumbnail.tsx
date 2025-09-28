import React from "react";
import { Play, Music, Video, Headphones } from "lucide-react";
import {
  isVideo,
  isAudio,
  formatDuration,
  getAssetAriaLabel,
} from "@/lib/utils/mediaTypes";

interface MediaThumbnailProps {
  asset: Asset;
  thumbnailUrl?: string;
  className?: string;
  onClick?: () => void;
}

/**
 * MediaThumbnail component that renders appropriate thumbnail based on asset type
 */
const MediaThumbnail: React.FC<MediaThumbnailProps> = ({
  asset,
  thumbnailUrl,
  className = "",
  onClick,
}) => {
  const videoAsset = isVideo(asset);
  const audioAsset = isAudio(asset);
  const duration = asset.duration;
  const ariaLabel = getAssetAriaLabel(asset);

  // Photo or thumbnail available - render image with potential overlays
  if (thumbnailUrl && !audioAsset) {
    return (
      <div
        className={`relative w-full h-full ${className}`}
        onClick={onClick}
        role="button"
        tabIndex={0}
        aria-label={ariaLabel}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onClick?.();
          }
        }}
      >
        <img
          src={thumbnailUrl}
          alt={asset.original_filename || "Asset"}
          className="w-full h-full object-cover transition-transform duration-200 hover:scale-105"
          loading="lazy"
        />

        {/* Media type indicator badge */}
        {videoAsset && (
          <div className="absolute top-2 left-2">
            <div className="bg-black/70 text-white text-xs px-2 py-1 rounded flex items-center gap-1">
              <Video className="w-3 h-3" />
              <span className="sr-only">Video</span>
            </div>
          </div>
        )}

        {/* Video play overlay */}
        {videoAsset && (
          <div className="absolute inset-0 flex items-center justify-center">
            <div
              className="bg-black/60 rounded-full p-3 hover:bg-black/80 transition-colors"
              aria-hidden="true"
            >
              <Play className="w-8 h-8 text-white fill-white ml-1" />
            </div>
          </div>
        )}

        {/* Duration badge for videos */}
        {videoAsset && duration && (
          <div className="absolute bottom-2 right-2 bg-black/80 text-white text-xs px-2 py-1 rounded">
            {formatDuration(duration)}
          </div>
        )}
      </div>
    );
  }

  // Audio asset - render audio-specific visualization
  if (audioAsset) {
    return (
      <div
        className={`w-full h-full bg-gradient-to-br from-purple-500 to-pink-500 flex flex-col items-center justify-center text-white cursor-pointer hover:from-purple-600 hover:to-pink-600 transition-colors ${className}`}
        onClick={onClick}
        role="button"
        tabIndex={0}
        aria-label={ariaLabel}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            onClick?.();
          }
        }}
      >
        {/* Audio type indicator */}
        <div className="absolute top-2 left-2">
          <div className="bg-black/30 text-white text-xs px-2 py-1 rounded flex items-center gap-1">
            <Headphones className="w-3 h-3" />
            <span className="sr-only">Audio</span>
          </div>
        </div>

        <Music className="w-12 h-12 mb-2" aria-hidden="true" />
        <div className="text-center px-2">
          <div className="text-sm font-medium truncate max-w-full">
            {asset.original_filename?.replace(/\.[^/.]+$/, "") || "Audio File"}
          </div>
          {duration && (
            <div className="text-xs opacity-80 mt-1">
              {formatDuration(duration)}
            </div>
          )}
        </div>
      </div>
    );
  }

  // Fallback for no preview available
  return (
    <div
      className={`w-full h-full bg-base-300 flex items-center justify-center text-base-content/50 cursor-pointer hover:bg-base-200 transition-colors ${className}`}
      onClick={onClick}
      role="button"
      tabIndex={0}
      aria-label={`Asset: ${asset.original_filename || "Unknown file"}`}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick?.();
        }
      }}
    >
      <div className="text-center">
        <div className="text-xs">No Preview</div>
        <div className="text-xs opacity-60">
          {asset.mime_type || asset.type || "Unknown MIME"}
        </div>
      </div>
    </div>
  );
};

export default MediaThumbnail;
