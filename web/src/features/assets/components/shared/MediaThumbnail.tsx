import React from "react";
import { Play, Music, Video, Headphones, CheckCircle2, Circle } from "lucide-react";
import {
  isVideo,
  isAudio,
  formatDuration,
  getAssetAriaLabel,
} from "@/lib/utils/mediaTypes";
import { Asset } from "@/services";

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
  const videoAsset = isVideo(asset);
  const audioAsset = isAudio(asset);
  const duration = asset.duration;
  const ariaLabel = getAssetAriaLabel(asset);

  const selectionOverlay = isSelectionMode && (
    <div className="absolute top-2 right-2 z-10">
      {isSelected ? (
        <CheckCircle2 className="text-primary fill-base-100" size={24} />
      ) : (
        <Circle className="text-white/50" size={24} />
      )}
    </div>
  );

  const selectionClass = isSelected ? "ring-4 ring-primary ring-inset" : "";

  // Photo or thumbnail available - render image with potential overlays
  if (thumbnailUrl && !audioAsset) {
    return (
      <div
        className={`relative w-full h-full overflow-hidden ${selectionClass} ${className}`}
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
        <img
          src={thumbnailUrl}
          alt={asset.original_filename || "Asset"}
          className={`w-full h-full object-cover transition-transform duration-200 ${isSelected ? '' : 'hover:scale-105'}`}
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
        {videoAsset && !isSelectionMode && (
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
        
        {isSelected && <div className="absolute inset-0 bg-primary/10 pointer-events-none" />}
      </div>
    );
  }

  // Audio asset - render audio-specific visualization
  if (audioAsset) {
    return (
      <div
        className={`relative w-full h-full flex flex-col items-center justify-center gap-2 text-center text-white cursor-pointer bg-gradient-to-b from-black/10 via-black/5 to-transparent px-4 py-3 rounded overflow-hidden transition-colors ${selectionClass} ${className}`}
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
        {isSelected && <div className="absolute inset-0 bg-primary/10 pointer-events-none" />}
      </div>
    );
  }

  // Fallback for no preview available
  return (
    <div
      className={`relative w-full h-full bg-base-300 flex items-center justify-center text-base-content/50 cursor-pointer hover:bg-base-200 transition-colors ${selectionClass} ${className}`}
      onClick={onClick}
      role="button"
      tabIndex={0}
      aria-label={`Asset: ${asset.original_filename || "Unknown file"}`}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onClick?.(e as any);
        }
      }}
    >
      {selectionOverlay}
      <div className="text-center">
        <div className="text-xs">No Preview</div>
        <div className="text-xs opacity-60">
          {asset.mime_type || asset.type || "Unknown MIME"}
        </div>
      </div>
      {isSelected && <div className="absolute inset-0 bg-primary/10 pointer-events-none" />}
    </div>
  );
};

export default MediaThumbnail;
