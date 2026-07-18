import React, { useEffect, useId, useMemo, useState } from "react";
import { MediaPlayer, MediaProvider } from "@vidstack/react";

import { assetUrls } from "@/lib/assets/assetUrls";
import { isVideo } from "../../../model/mediaTypes";
import { Asset } from "@/lib/assets/types";
import "@vidstack/react/player/styles/base.css";
import "@vidstack/react/player/styles/default/theme.css";
import "@vidstack/react/player/styles/default/layouts/video.css";
import "@vidstack/react/player/styles/default/layouts/audio.css";

import { defaultLayoutIcons, DefaultVideoLayout } from "@vidstack/react/player/layouts/default";
import { useI18n } from "@/lib/i18n";
import { useAssetMediaItem } from "../../../api/useAssetMediaItem";
import { useLivePhotoPlayback } from "../useLivePhotoPlayback";
import { LivePhotos } from "./LivePhotos";

interface MediaViewerProps {
  asset: Asset;
  className?: string;
  /** Whether this slide is the currently active one in the carousel. */
  isActive?: boolean;
  selectedAssetId?: string;
  onSelectedAssetChange?: (assetId: string) => void;
}

/**
 * MediaViewer renders the appropriate viewer for an asset.
 *
 * For Live Photo media items the still image is shown by default. A Live Photo
 * icon button appears in the top-left corner; hovering (or pressing on touch
 * devices) plays the motion video as a translucent layer over the still image,
 * replicating the native Apple Live Photo experience without any visible
 * player controls.
 */
const MediaViewer: React.FC<MediaViewerProps> = ({
  asset,
  className = "",
  isActive = true,
  selectedAssetId: controlledSelectedAssetId,
  onSelectedAssetChange,
}) => {
  const { t } = useI18n();
  const videoAsset = isVideo(asset);
  const mediaItemQuery = useAssetMediaItem(asset.asset_id, isActive);
  const mediaItem = mediaItemQuery.data?.media_item;
  const components = useMemo(() => mediaItem?.components ?? [], [mediaItem?.components]);
  const isLivePhoto = mediaItem?.media_kind === "live_photo";
  const motionComponent = components.find((component) => component.relation === "live_photo_video");
  const livePhotoVideoUrl = motionComponent?.asset_id
    ? assetUrls.getWebVideoUrl(motionComponent.asset_id)
    : undefined;
  const visualComponents = components.filter(
    (component) => component.relation === "raw_original" || component.relation === "jpeg_original",
  );
  const [internalSelectedAssetId, setInternalSelectedAssetId] = useState(asset.asset_id);
  const selectedAssetId = controlledSelectedAssetId ?? internalSelectedAssetId;
  const componentTabGroupName = useId();

  useEffect(() => {
    if (controlledSelectedAssetId !== undefined) return;
    setInternalSelectedAssetId(mediaItem?.primary_asset_id ?? asset.asset_id);
  }, [asset.asset_id, controlledSelectedAssetId, mediaItem?.primary_asset_id]);

  const selectAsset = (assetId?: string) => {
    if (!assetId) return;
    if (controlledSelectedAssetId === undefined) {
      setInternalSelectedAssetId(assetId);
    }
    onSelectedAssetChange?.(assetId);
  };

  const { videoRef, isPlaying, handlePlay, handleStop, handleEnded } = useLivePhotoPlayback();

  // Get media source URL
  const webVideoUrl =
    videoAsset && asset.asset_id ? assetUrls.getWebVideoUrl(asset.asset_id) : undefined;

  // For photos, get large thumbnail as fallback to original
  const imageUrl =
    !videoAsset && selectedAssetId
      ? assetUrls.getThumbnailUrl(selectedAssetId, "large")
      : undefined;

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

          {visualComponents.length > 1 && (
            <div
              role="tablist"
              aria-label={t("assets.mediaViewer.componentTabs", {
                defaultValue: "Media components",
              })}
              className="tabs tabs-box absolute top-2.5 right-2.5 z-10 shadow-lg"
            >
              {visualComponents.map((component) => {
                const label =
                  component.relation === "raw_original"
                    ? t("assets.mediaViewer.componentRaw", { defaultValue: "RAW" })
                    : t("assets.mediaViewer.componentJpeg", { defaultValue: "JPEG" });
                return (
                  <input
                    key={component.asset_id}
                    type="radio"
                    name={componentTabGroupName}
                    className="tab"
                    aria-label={label}
                    checked={selectedAssetId === component.asset_id}
                    onChange={() => selectAsset(component.asset_id)}
                  />
                );
              })}
            </div>
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
