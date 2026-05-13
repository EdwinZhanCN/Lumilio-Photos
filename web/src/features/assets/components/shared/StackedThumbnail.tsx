import React, { useState } from "react";
import { Layers } from "lucide-react";
import { Asset, StackPreview } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import MediaThumbnail from "./MediaThumbnail";
import StackDetailModal from "./StackDetailModal";

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
 * - A modal that loads and displays stack details on demand
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
  const [detailsOpen, setDetailsOpen] = useState(false);
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
            className="absolute bottom-3 right-3 z-30 inline-flex items-center gap-1.5 rounded-full border border-white/15 bg-black/65 px-2.5 py-1.5 text-xs font-medium text-white shadow-lg backdrop-blur-sm transition-colors hover:bg-black/80"
            onClick={(event) => {
              event.stopPropagation();
              setDetailsOpen(true);
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
            <Layers className="size-3.5" />
            <span>{stackCount}</span>
          </button>
        )}
      </div>

      {hasStack && (
        <StackDetailModal
          asset={asset}
          open={detailsOpen}
          stackSize={stackCount}
          onClose={() => setDetailsOpen(false)}
        />
      )}
    </>
  );
};

export default StackedThumbnail;
