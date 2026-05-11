import React from "react";
import { MediaPlayer, MediaProvider } from "@vidstack/react";

import { assetUrls } from "@/lib/assets/assetUrls";
import { isVideo } from "@/lib/utils/mediaTypes";
import { Asset } from "@/lib/assets/types";
import "@vidstack/react/player/styles/base.css";
import "@vidstack/react/player/styles/default/theme.css";
import "@vidstack/react/player/styles/default/layouts/video.css";
import "@vidstack/react/player/styles/default/layouts/audio.css";

import {
  defaultLayoutIcons,
  DefaultVideoLayout,
} from "@vidstack/react/player/layouts/default";
import { useI18n } from "@/lib/i18n";

interface MediaViewerProps {
  asset: Asset;
  className?: string;
}

/**
 * MediaViewer component that renders appropriate viewer based on asset type
 */
const MediaViewer: React.FC<MediaViewerProps> = ({ asset, className = "" }) => {
  const { t } = useI18n();
  const videoAsset = isVideo(asset);

  // Get media source URL
  const webVideoUrl = asset.asset_id
    ? assetUrls.getWebVideoUrl(asset.asset_id)
    : undefined;

  // For photos, get large thumbnail as fallback to original
  const imageUrl =
    !videoAsset && asset.asset_id
      ? assetUrls.getThumbnailUrl(asset.asset_id, "large")
      : undefined;

  // Video player
  if (videoAsset && webVideoUrl) {
    return (
      <div
        className={`h-screen w-screen flex items-center justify-center ${className}`}
      >
        <div className="w-full max-w-6xl h-auto max-h-[90vh]">
          <MediaPlayer
            title={
              asset.original_filename || t("assets.mediaViewer.video_title")
            }
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

  // Photo display (fallback to existing behavior)
  if (imageUrl) {
    return (
      <div
        className={`h-screen w-screen flex items-center justify-center p-4 ${className}`}
      >
        <img
          src={imageUrl}
          alt={
            asset.original_filename || t("assets.mediaViewer.asset_alt_text")
          }
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
        <div className="text-xl mb-2">
          {t("assets.mediaViewer.media_not_available")}
        </div>
        <div className="text-sm opacity-70">
          {asset.original_filename || t("assets.mediaViewer.unknown_file")}
        </div>
        <div className="text-xs opacity-50 mt-1">
          {asset.mime_type ||
            asset.type ||
            t("assets.mediaViewer.unknown_type")}
        </div>
      </div>
    </div>
  );
};

export default MediaViewer;
