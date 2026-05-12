import React, { useState } from "react";
import { Layers, ChevronDown, ChevronUp } from "lucide-react";
import { Asset, StackPreview } from "@/lib/assets/types";
import MediaThumbnail from "./MediaThumbnail";

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
 * - A stack badge showing the count of assets in the stack
 * - Click-to-expand behavior to reveal all stack members
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
  const [expanded, setExpanded] = useState(false);

  // If this asset is part of a stack and is the cover, show stack badge
  if (
    stackInfo.stack_cover &&
    stackInfo.stack_size &&
    stackInfo.stack_size > 1
  ) {
    const otherCount = stackInfo.stack_size - 1;

    return (
      <div className="relative group">
        {/* Main thumbnail (cover) */}
        <div className="relative">
          <MediaThumbnail
            asset={asset}
            thumbnailUrl={thumbnailUrl}
            className={className}
            onClick={(e) => {
              if (isSelectionMode) {
                onClick?.(e);
              } else {
                setExpanded(!expanded);
              }
            }}
            isSelected={isSelected}
            isSelectionMode={isSelectionMode}
          />

          {/* Stack badge */}
          {!isSelectionMode && (
            <div
              className="absolute bottom-2 right-2 z-20 flex items-center gap-1 rounded-full border border-white/15 bg-black/65 px-2.5 py-1 text-xs text-white shadow-lg backdrop-blur-sm cursor-pointer hover:bg-black/80 transition-colors"
              onClick={(e) => {
                e.stopPropagation();
                setExpanded(!expanded);
              }}
              title={`${otherCount} more in stack`}
            >
              <Layers className="w-3 h-3" />
              <span>+{otherCount}</span>
              {expanded ? (
                <ChevronUp className="w-3 h-3" />
              ) : (
                <ChevronDown className="w-3 h-3" />
              )}
            </div>
          )}
        </div>

        {/* Expanded members (placeholder for now - loads full stack members) */}
        {expanded && (
          <div className="mt-2 grid grid-cols-1 gap-2 pl-4 border-l-2 border-primary/30">
            <div className="text-xs text-base-content/60 py-1">
              Stack contains {otherCount} related asset
              {otherCount !== 1 ? "s" : ""}
            </div>
            {/* Individual stack members would be fetched and rendered here */}
            {/* For now, show a placeholder indicating this is a stack */}
            <div
              className="text-xs text-base-content/40 py-2 px-3 bg-base-200 rounded cursor-pointer hover:bg-base-300 transition-colors"
              onClick={(e) => {
                e.stopPropagation();
                // Navigate to stack detail view
                if (stackInfo.stack_id) {
                  window.location.href = `/assets/${asset.asset_id}/stack`;
                }
              }}
            >
              Click to view all stack members →
            </div>
          </div>
        )}
      </div>
    );
  }

  // Not a stack or not a cover - render plain thumbnail
  return (
    <MediaThumbnail
      asset={asset}
      thumbnailUrl={thumbnailUrl}
      className={className}
      onClick={onClick}
      isSelected={isSelected}
      isSelectionMode={isSelectionMode}
    />
  );
};

export default StackedThumbnail;
