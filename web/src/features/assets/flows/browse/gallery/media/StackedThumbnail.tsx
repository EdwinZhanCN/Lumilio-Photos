import { useState } from "react";
import { GalleryHorizontalEnd, Layers } from "lucide-react";
import { Asset, StackPreview } from "@/lib/assets/types";
import { useI18n } from "@/lib/i18n";
import MediaThumbnail from "./MediaThumbnail";
import StackCarouselOverlay from "./StackCarouselOverlay";
import { resolveStackFocusAssetId } from "../../../../model/browseItems";
import type { BrowseStackItem, BrowseStackKind, MediaCompositionFacts } from "../../../../types";

interface StackedThumbnailProps {
  asset: Asset;
  thumbnailUrl?: string;
  stackInfo: StackPreview;
  browseStack?: BrowseStackItem;
  composition?: MediaCompositionFacts;
  className?: string;
  onClick?: (e: React.MouseEvent) => void;
  isSelected?: boolean;
  isSelectionMode?: boolean;
}

const STACK_KIND_ICONS: Record<BrowseStackKind, typeof Layers> = {
  burst: GalleryHorizontalEnd,
  manual: Layers,
};

/**
 * StackedThumbnail wraps a MediaThumbnail with stack-aware UI elements.
 *
 * Burst and manual presentation stacks show a clickable logical-item count.
 * RAW/JPEG and Live Photo components are represented by a media item and do
 * not enter this component as stacks.
 */
const StackedThumbnail: React.FC<StackedThumbnailProps> = ({
  asset,
  thumbnailUrl,
  stackInfo,
  browseStack,
  composition,
  className,
  onClick,
  isSelected,
  isSelectionMode,
}) => {
  const { t } = useI18n();
  const [stackCarouselOpen, setStackCarouselOpen] = useState(false);
  const stackCount = stackInfo.stack_size ?? 0;
  const hasStack = Boolean(stackInfo.stack_id) && stackCount > 1;
  const focusAssetId = resolveStackFocusAssetId(asset, browseStack);

  const stackKind: BrowseStackKind = browseStack?.stackKind ?? stackInfo.stack_kind ?? "manual";
  const KindIcon = STACK_KIND_ICONS[stackKind];
  // The filter can match only part of a stack; the collapsed row then says "2 / 7".
  const matchedCount = browseStack?.matchedMembers.length ?? 0;
  const memberCount = browseStack?.members.length ?? stackCount;
  const isPartialMatch = Boolean(browseStack) && matchedCount > 0 && matchedCount < memberCount;
  const countLabel = isPartialMatch ? `${matchedCount} / ${memberCount}` : String(stackCount);

  const kindLabel =
    stackKind === "burst"
      ? t("assets.stackDetail.burstKind", "Burst")
      : t("assets.stackDetail.manualKind", "Manual stack");
  const stackAriaLabel = isPartialMatch
    ? t("assets.stackDetail.partialMatch", {
        kind: kindLabel,
        matched: matchedCount,
        total: memberCount,
        defaultValue: "{{kind}}: {{matched}} of {{total}} match the current filter",
      })
    : t("assets.stackDetail.openButton", {
        count: stackCount,
        defaultValue: stackCount === 1 ? "View stack details" : `View ${stackCount} related assets`,
      });

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
          composition={composition}
        />

        {hasStack && !isSelectionMode && (
          <button
            type="button"
            className="btn btn-sm btn-neutral absolute bottom-3 right-3 shadow-lg"
            onClick={(event) => {
              event.stopPropagation();
              setStackCarouselOpen(true);
            }}
            aria-label={stackAriaLabel}
            title={stackAriaLabel}
          >
            <div
              className="tooltip m-0 inline-flex items-center text-xs font-medium gap-1.5 text-white"
              data-tip={stackAriaLabel}
            >
              <KindIcon className="size-3.5" />
              <span>{countLabel}</span>
            </div>
          </button>
        )}
      </div>

      {hasStack && (
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
