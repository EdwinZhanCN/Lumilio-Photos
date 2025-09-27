import React from "react";
import { Play, Music, FileText } from "lucide-react";

interface MediaThumbnailProps {
  asset: Asset;
  thumbnailUrl?: string;
  className?: string;
  onClick?: () => void;
}

/**
 * Determines if an asset is a video based on MIME type or legacy type
 */
const isVideo = (asset: Asset): boolean => {
  if (asset.mime_type) {
    return asset.mime_type.startsWith("video/");
  }
  return asset.type === "VIDEO";
};

/**
 * Determines if an asset is audio based on MIME type or legacy type
 */
const isAudio = (asset: Asset): boolean => {
  if (asset.mime_type) {
    return asset.mime_type.startsWith("audio/");
  }
  return asset.type === "AUDIO";
};

/**
 * Formats duration in seconds to MM:SS format
 */
const formatDuration = (duration?: number): string => {
  if (!duration) return "";
  const minutes = Math.floor(duration / 60);
  const seconds = Math.floor(duration % 60);
  return `${minutes}:${seconds.toString().padStart(2, "0")}`;
};

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
  const duration = asset.duration || asset.specific_metadata?.duration;

  // Photo or thumbnail available - render image with potential overlays
  if (thumbnailUrl && !audioAsset) {
    return (
      <div className={`relative w-full h-full ${className}`} onClick={onClick}>
        <img
          src={thumbnailUrl}
          alt={asset.original_filename || "Asset"}
          className="w-full h-full object-cover transition-transform duration-200 hover:scale-105"
          loading="lazy"
        />
        
        {/* Video play overlay */}
        {videoAsset && (
          <div className="absolute inset-0 flex items-center justify-center">
            <div className="bg-black/60 rounded-full p-3 hover:bg-black/80 transition-colors">
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
        className={`w-full h-full bg-gradient-to-br from-purple-500 to-pink-500 flex flex-col items-center justify-center text-white ${className}`}
        onClick={onClick}
      >
        <Music className="w-12 h-12 mb-2" />
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

  // Document or other file types
  if (asset.type === "DOCUMENT" || asset.mime_type?.startsWith("application/")) {
    return (
      <div 
        className={`w-full h-full bg-base-300 flex flex-col items-center justify-center text-base-content/70 ${className}`}
        onClick={onClick}
      >
        <FileText className="w-12 h-12 mb-2" />
        <div className="text-center px-2">
          <div className="text-sm font-medium truncate max-w-full">
            {asset.original_filename || "Document"}
          </div>
          <div className="text-xs opacity-60 mt-1">
            {asset.mime_type || asset.type || "Unknown"}
          </div>
        </div>
      </div>
    );
  }

  // Fallback for no preview available
  return (
    <div 
      className={`w-full h-full bg-base-300 flex items-center justify-center text-base-content/50 ${className}`}
      onClick={onClick}
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