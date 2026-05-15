import { useState } from "react";
import { Layers } from "lucide-react";
import { Asset, StackPreview } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import MediaThumbnail from "./MediaThumbnail";
import StackCarouselOverlay from "./StackCarouselOverlay";

interface StackedThumbnailProps {
  asset: Asset;
  thumbnailUrl?: string;
  stackInfo: StackPreview;
  className?: string;
  onClick?: (e: React.MouseEvent) => void;
  isSelected?: boolean;
  isSelectionMode?: boolean;
}

/**
 * StackedThumbnail wraps a MediaThumbnail with stack-aware UI elements:
 * - An overlay button with the stack count
 * - A fullscreen overlay that loads and displays stack members on demand
 */
const StackedThumbnail: React.FC<StackedThumbnailProps> = ({
  asset,
  thumbnailUrl,
  stackInfo,
  className,
  onClick,
  isSelected,
  isSelectionMode,
}) => {
  const { t } = useI18n();
  const [stackCarouselOpen, setStackCarouselOpen] = useState(false);
  const stackCount = stackInfo.stack_size ?? 0;
  const hasStack = Boolean(stackInfo.stack_id) && stackCount > 1;

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

        {hasStack && !isSelectionMode && (
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
                stackCount === 1
                  ? "View stack details"
                  : `View ${stackCount} related assets`,
            })}
            title={t("assets.stackDetail.openButton", {
              count: stackCount,
              defaultValue:
                stackCount === 1
                  ? "View stack details"
                  : `View ${stackCount} related assets`,
            })}
          >
            <div className="tooltip m-0 inline-flex items-center text-xs font-medium gap-1.5 text-white" data-tip={t("assets.stackDetail.openButton", {
              count: stackCount,
              defaultValue:
                stackCount === 1
                  ? "View stack details"
                  : `View ${stackCount} related assets`,
            })}>
              <Layers className="size-3.5" />
              <span>{stackCount}</span>
            </div>
          </button>
        )}
      </div>

      {hasStack && (
        <StackCarouselOverlay
          asset={asset}
          open={stackCarouselOpen}
          onClose={() => setStackCarouselOpen(false)}
        />
      )}
    </>
  );
};

export default StackedThumbnail;
