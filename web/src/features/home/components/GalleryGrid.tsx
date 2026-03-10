import React, { useCallback, useMemo } from "react";
import SquareGallery from "@/features/assets/components/page/SquareGallery/SquareGallery";
import { Asset } from "@/lib/assets/types";
import type { AssetGroup } from "@/features/assets";

export type GalleryGridProps = {
  assets?: Asset[];
  placeholderCount?: number;
  className?: string;
  onItemClick?: (asset: Asset, index: number) => void;
};

const PlaceholderGrid: React.FC<{
  count: number;
  className?: string;
}> = ({ count, className = "" }) => (
  <section className={`w-full px-4 pb-8 transition-all ${className}`}>
    <div className="grid grid-cols-2 gap-1 md:grid-cols-3 lg:grid-cols-4">
      {Array.from({ length: count }).map((_, index) => (
        <div
          key={index}
          className="relative aspect-square overflow-hidden rounded-[1.25rem] border border-base-300/70 bg-gradient-to-br from-base-200 via-base-200 to-base-300 shadow-[0_16px_40px_-30px_rgba(15,23,42,0.3)]"
        >
          <div className="absolute inset-0 animate-pulse bg-gradient-to-br from-base-100/10 via-transparent to-base-100/5" />
        </div>
      ))}
    </div>
  </section>
);

const GalleryGrid: React.FC<GalleryGridProps> = ({
  assets = [],
  placeholderCount = 8,
  className = "",
  onItemClick,
}) => {
  const groups = useMemo<AssetGroup[]>(
    () => [{ key: "flat:all", assets }],
    [assets],
  );

  const openCarousel = useCallback(
    (assetId: string) => {
      const index = assets.findIndex((asset) => asset.asset_id === assetId);
      if (index === -1) return;
      const asset = assets[index];
      if (!asset) return;
      onItemClick?.(asset, index);
    },
    [assets, onItemClick],
  );

  if (assets.length === 0) {
    return <PlaceholderGrid count={placeholderCount} className={className} />;
  }

  return (
    <SquareGallery
      groups={groups}
      openCarousel={openCarousel}
      onLoadMore={() => {}}
      hasMore={false}
      isLoadingMore={false}
      className={className}
    />
  );
};

export default GalleryGrid;
