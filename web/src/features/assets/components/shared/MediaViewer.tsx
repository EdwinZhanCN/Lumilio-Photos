import React from "react";
import { MediaPlayer, MediaProvider } from "@vidstack/react";

import { assetUrls } from "@/lib/assets/assetUrls";
import { isVideo } from "@/lib/utils/mediaTypes";
import { Asset } from "@/lib/assets/types";
import "@vidstack/react/player/styles/base.css";
import "@vidstack/react/player/styles/default/theme.css";
import "@vidstack/react/player/styles/default/layouts/video.css";
import "@vidstack/react/player/styles/default/layouts/audio.css";

import { defaultLayoutIcons, DefaultVideoLayout } from "@vidstack/react/player/layouts/default";
import { useI18n } from "@/lib/i18n";
import { useStackCarouselAssets } from "@/features/assets/hooks/useStackCarouselAssets";
import { useLivePhotoPlayback } from "@/features/assets/hooks/useLivePhotoPlayback";
import { LivePhotos } from "@/components/icons/LivePhotos";

interface MediaViewerProps {
  asset: Asset;
  className?: string;
  /** Whether this slide is the currently active one in the carousel. */
  isActive?: boolean;
}

/**
 * MediaViewer renders the appropriate viewer for an asset.
 *
 * For Live Photo stacks the still image is shown by default.  A Live Photo
 * icon button appears in the top-left corner; hovering (or pressing on touch
 * devices) plays the motion video as a translucent layer over the still image,
 * replicating the native Apple Live Photo experience without any visible
 * player controls.
 */
const MediaViewer: React.FC<MediaViewerProps> = ({ asset, className = "", isActive = true }) => {
  const { t } = useI18n();
  const videoAsset = isVideo(asset);
  const isLivePhoto = asset.stack?.stack_kind === "live_photo";

  // Fetch stack members only for Live Photo assets.
  const { assets: stackAssets } = useStackCarouselAssets(asset, isLivePhoto && isActive);

  const motionAsset = isLivePhoto ? stackAssets.find((a) => isVideo(a)) : undefined;
  const livePhotoVideoUrl = motionAsset?.asset_id
    ? assetUrls.getWebVideoUrl(motionAsset.asset_id)
    : undefined;

  const { videoRef, isPlaying, handlePlay, handleStop, handleEnded } = useLivePhotoPlayback();

  // Get media source URL
  const webVideoUrl =
    videoAsset && asset.asset_id ? assetUrls.getWebVideoUrl(asset.asset_id) : undefined;

  // For photos, get large thumbnail as fallback to original
  const imageUrl =
    !videoAsset && asset.asset_id ? assetUrls.getThumbnailUrl(asset.asset_id, "large") : undefined;

  // ── Regular video player ──────────────────────────────────────────────────
  if (videoAsset && webVideoUrl) {
    return (
      <div className={`h-screen w-screen flex items-center justify-center ${className}`}>
        <div className="w-full max-w-6xl h-auto max-h-[90vh]">
          <MediaPlayer
            title={asset.original_filename || t("assets.mediaViewer.video_title")}
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

  // ── Photo display (with optional Live Photo overlay) ──────────────────────
  if (imageUrl) {
    return (
      <div className={`h-screen w-screen flex items-center justify-center p-4 ${className}`}>
        {/*
          Inner wrapper sizes itself to the rendered image, not the viewport.
          This ensures the Live Photo button is anchored to the image corner,
          not the top-left of the full screen (which would overlap the close btn).
        */}
        <div
          className="relative"
          style={{
            maxHeight: `calc(100vh - 2rem)`,
            maxWidth: `calc(100vw - 2rem)`,
            // Let the div shrink-wrap to the img's intrinsic size.
            // `inline-flex` collapses to content width/height.
            display: "inline-flex",
          }}
        >
          {/* Still image */}
          <img
            src={imageUrl}
            alt={asset.original_filename || t("assets.mediaViewer.asset_alt_text")}
            style={{
              maxHeight: `calc(100vh - 2rem)`,
              maxWidth: `calc(100vw - 2rem)`,
            }}
            className="block object-contain select-none"
          />

          {/* Live Photo motion video — fades in on hover, hidden otherwise */}
          {isLivePhoto && livePhotoVideoUrl && (
            <video
              ref={videoRef}
              src={livePhotoVideoUrl}
              muted
              playsInline
              preload={isActive ? "auto" : "metadata"}
              onEnded={handleEnded}
              style={{
                maxHeight: `calc(100vh - 2rem)`,
                maxWidth: `calc(100vw - 2rem)`,
              }}
              className={[
                "absolute inset-0 w-full h-full object-contain",
                "select-none pointer-events-none",
                "transition-opacity duration-150 ease-in",
                isPlaying ? "opacity-100" : "opacity-0",
              ].join(" ")}
            />
          )}

          {/* Live Photo icon button — anchored to the image's top-left corner */}
          {isLivePhoto && (
            <button
              type="button"
              onPointerEnter={handlePlay}
              onPointerLeave={handleStop}
              onPointerDown={handlePlay}
              onPointerUp={handleStop}
              aria-label={t("assets.livePhoto.playButton", {
                defaultValue: "Play Live Photo",
              })}
              title={t("assets.livePhoto.playButton", {
                defaultValue: "Play Live Photo",
              })}
              className={[
                "absolute top-2.5 left-2.5 z-10",
                "flex items-center justify-center",
                "rounded-full border border-white/25 bg-black/55 p-2",
                "backdrop-blur-md shadow-lg",
                "transition-colors duration-200",
                "hover:bg-black/75 hover:border-white/45",
                "active:scale-95 select-none",
                isPlaying ? "text-white" : "text-white/75",
              ].join(" ")}
            >
              <LivePhotos className="size-4" />
            </button>
          )}
        </div>
      </div>
    );
  }

  // ── Fallback for unsupported or missing media ─────────────────────────────
  return (
    <div className={`w-full h-full flex items-center justify-center text-white ${className}`}>
      <div className="text-center">
        <div className="text-xl mb-2">{t("assets.mediaViewer.media_not_available")}</div>
        <div className="text-sm opacity-70">
          {asset.original_filename || t("assets.mediaViewer.unknown_file")}
        </div>
        <div className="text-xs opacity-50 mt-1">
          {asset.mime_type || asset.type || t("assets.mediaViewer.unknown_type")}
        </div>
      </div>
    </div>
  );
};

export default MediaViewer;
