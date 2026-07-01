import { useState } from "react";
import { Layers } from "lucide-react";
import { Asset, StackPreview } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import MediaThumbnail from "./MediaThumbnail";
import StackCarouselOverlay from "./StackCarouselOverlay";
import { LivePhotos } from "@/components/icons/LivePhotos";
import { resolveStackFocusAssetId } from "@/features/assets/utils/browseItems";
import type { BrowseStackItem } from "@/features/assets/types/assets.type";

interface StackedThumbnailProps {
  asset: Asset;
  thumbnailUrl?: string;
  stackInfo: StackPreview;
  browseStack?: BrowseStackItem;
  className?: string;
  onClick?: (e: React.MouseEvent) => void;
  isSelected?: boolean;
  isSelectionMode?: boolean;
}

/**
 * StackedThumbnail wraps a MediaThumbnail with stack-aware UI elements.
 *
 * - Regular stacks: shows a clickable badge (layer count) that opens
 *   StackCarouselOverlay.
 * - Live Photo stacks: shows a non-interactive Live Photo badge in the
 *   bottom-right corner. The live-photo experience is handled entirely
 *   inside the FullScreenCarousel's MediaViewer — clicking the thumbnail
 *   opens the normal carousel where the user can hover the Live Photo
 *   icon to play the motion video.
 */
const StackedThumbnail: React.FC<StackedThumbnailProps> = ({
  asset,
  thumbnailUrl,
  stackInfo,
  browseStack,
  className,
  onClick,
  isSelected,
  isSelectionMode,
}) => {
  const { t } = useI18n();
  const [stackCarouselOpen, setStackCarouselOpen] = useState(false);
  const stackCount = stackInfo.stack_size ?? 0;
  const hasStack = Boolean(stackInfo.stack_id) && stackCount > 1;
  const isLivePhotoStack = stackInfo.stack_kind === "live_photo";
  const focusAssetId = resolveStackFocusAssetId(asset, browseStack);

  return (
    <>
      <div className="relative h-full w-full">
        <MediaThumbnail
          asset={asset}
          thumbnailUrl={thumbnailUrl}
          className={className}
          onClick={onClick}
          isSelected={isSelected}
          isSelectionMode={isSelectionMode}
        />

        {/* Live Photo: non-interactive badge — experience is in the carousel */}
        {hasStack && isLivePhotoStack && !isSelectionMode && (
          <div
            className="absolute bottom-3 right-3 z-10 rounded-full border border-white/15 bg-black/65 p-1.5 shadow-lg backdrop-blur-sm"
            title={t("assets.stackDetail.livePhoto", {
              defaultValue: "Live Photo",
            })}
            aria-hidden="true"
          >
            <LivePhotos className="size-4 text-white" />
          </div>
        )}

        {/* Regular stack: clickable badge that opens the carousel overlay */}
        {hasStack && !isLivePhotoStack && !isSelectionMode && (
          <button
            type="button"
            className="absolute bottom-3 right-3 cursor-pointer z-10 rounded-full border border-white/15 bg-black/65 px-2.5 py-1.5 shadow-lg backdrop-blur-sm transition-colors hover:bg-black/80"
            onClick={(event) => {
              event.stopPropagation();
              setStackCarouselOpen(true);
            }}
            aria-label={t("assets.stackDetail.openButton", {
              count: stackCount,
              defaultValue:
                stackCount === 1 ? "View stack details" : `View ${stackCount} related assets`,
            })}
            title={t("assets.stackDetail.openButton", {
              count: stackCount,
              defaultValue:
                stackCount === 1 ? "View stack details" : `View ${stackCount} related assets`,
            })}
          >
            <div
              className="tooltip m-0 inline-flex items-center text-xs font-medium gap-1.5 text-white"
              data-tip={t("assets.stackDetail.openButton", {
                count: stackCount,
                defaultValue:
                  stackCount === 1 ? "View stack details" : `View ${stackCount} related assets`,
              })}
            >
              <Layers className="size-3.5" />
              <span>{stackCount}</span>
            </div>
          </button>
        )}
      </div>

      {/* Regular stacks open a carousel overlay; Live Photos use the main carousel */}
      {hasStack && !isLivePhotoStack && (
        <StackCarouselOverlay
          asset={asset}
          focusAssetId={focusAssetId}
          open={stackCarouselOpen}
          onClose={() => setStackCarouselOpen(false)}
        />
      )}
    </>
  );
};

export default StackedThumbnail;
