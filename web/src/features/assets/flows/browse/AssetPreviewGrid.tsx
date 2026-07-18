import { useCallback, useMemo } from "react";
import type { Asset } from "@/lib/assets/types";
import { createBrowseGroupsFromAssets, flattenBrowseGroupsToAssets } from "../../model/browseItems";
import SquareGallery from "./gallery/SquareGallery/SquareGallery";
import { AssetBrowserScope } from "./selection/AssetBrowserScope";

export interface AssetPreviewGridProps {
  assets: Asset[];
  scopeId: string;
  className?: string;
  onItemClick?: (asset: Asset, index: number) => void;
}

/**
 * Small, finite asset preview for another feature's dashboard or summary.
 * It owns the browse-model adaptation and isolated selection scope so callers
 * do not depend on Assets gallery internals.
 */
export function AssetPreviewGrid({
  assets,
  scopeId,
  className,
  onItemClick,
}: AssetPreviewGridProps) {
  const browseGroups = useMemo(() => createBrowseGroupsFromAssets(assets), [assets]);
  const visibleAssets = useMemo(() => flattenBrowseGroupsToAssets(browseGroups), [browseGroups]);

  const openItem = useCallback(
    (assetId: string) => {
      const index = visibleAssets.findIndex((asset) => asset.asset_id === assetId);
      const asset = visibleAssets[index];
      if (index >= 0 && asset) onItemClick?.(asset, index);
    },
    [onItemClick, visibleAssets],
  );

  return (
    <AssetBrowserScope scopeId={scopeId}>
      <SquareGallery
        browseGroups={browseGroups}
        openCarousel={openItem}
        onLoadMore={() => {}}
        hasMore={false}
        isLoadingMore={false}
        className={className}
        render3DCard
      />
    </AssetBrowserScope>
  );
}
