import React from "react";
import { MediaPlayer, MediaProvider } from "@vidstack/react";

import { assetService } from "@/services/assetsService";
import { isVideo, isAudio } from "@/lib/utils/mediaTypes";
import { Asset } from "@/services";
import "@vidstack/react/player/styles/base.css";
import "@vidstack/react/player/styles/default/theme.css";
import "@vidstack/react/player/styles/default/layouts/video.css";
import "@vidstack/react/player/styles/default/layouts/audio.css";

import {
  defaultLayoutIcons,
  DefaultVideoLayout,
  DefaultAudioLayout,
} from "@vidstack/react/player/layouts/default";
interface MediaViewerProps {
  asset: Asset;
  className?: string;
}

/**
 * MediaViewer component that renders appropriate viewer based on asset type
 */
const MediaViewer: React.FC<MediaViewerProps> = ({ asset, className = "" }) => {
  const videoAsset = isVideo(asset);
  const audioAsset = isAudio(asset);

  // Get media source URL
  const webVideoUrl = asset.asset_id
    ? assetService.getWebVideoUrl(asset.asset_id)
    : undefined;
  const webAudioUrl = asset.asset_id
    ? assetService.getWebAudioUrl(asset.asset_id)
    : undefined;

  // For photos, get large thumbnail as fallback to original
  const imageUrl =
    !videoAsset && !audioAsset && asset.asset_id
      ? assetService.getThumbnailUrl(asset.asset_id, "large")
      : undefined;

  // Video player
  if (videoAsset && webVideoUrl) {
    return (
      <div
        className={`h-screen w-screen flex items-center justify-center ${className}`}
      >
        <div className="w-full max-w-6xl h-auto max-h-[90vh]">
          <MediaPlayer
            title={asset.original_filename || "Video"}
            src={webVideoUrl}
            load="visible"
            crossOrigin
            playsInline
            onError={(error) => console.error("Video player error:", error)}
          >
            <MediaProvider />
            <DefaultVideoLayout icons={defaultLayoutIcons} />
          </MediaPlayer>
        </div>
      </div>
    );
  }

  // Audio player - using Vidstack's audio layout
  if (audioAsset && webAudioUrl) {
    // Try to get a poster image for audio (album art or thumbnail)
    const posterUrl = asset.asset_id
      ? assetService.getThumbnailUrl(asset.asset_id, "medium")
      : undefined;

    return (
      <div
        className={`h-screen w-screen flex items-center justify-center bg-gradient-to-br from-purple-900 to-pink-900 ${className}`}
      >
        <div className="w-full max-w-md h-auto">
          <MediaPlayer
            title={asset.original_filename || "Audio"}
            src={webAudioUrl}
            poster={posterUrl}
            load="visible"
            viewType="audio"
            streamType="on-demand"
            crossOrigin = "use-credentials"
            onError={(error) => console.error("Audio player error:", error)}
          >
            <MediaProvider />
            <DefaultAudioLayout icons={defaultLayoutIcons} />
          </MediaPlayer>
        </div>
      </div>
    );
  }

  // Photo display (fallback to existing behavior)
  if (imageUrl) {
    return (
      <div
        className={`h-screen w-screen flex items-center justify-center p-4 ${className}`}
      >
        <img
          src={imageUrl}
          alt={asset.original_filename || "Asset"}
          className="max-h-full max-w-full object-contain select-none"
        />
      </div>
    );
  }

  // Fallback for unsupported or missing media
  return (
    <div
      className={`w-full h-full flex items-center justify-center text-white ${className}`}
    >
      <div className="text-center">
        <div className="text-xl mb-2">Media not available</div>
        <div className="text-sm opacity-70">
          {asset.original_filename || "Unknown file"}
        </div>
        <div className="text-xs opacity-50 mt-1">
          {asset.mime_type || asset.type || "Unknown type"}
        </div>
      </div>
    </div>
  );
};

export default MediaViewer;
