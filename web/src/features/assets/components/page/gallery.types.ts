import { Asset } from "@/lib/assets/types";

export interface AssetGalleryProps {
  groupedPhotos: Record<string, Asset[]>;
  openCarousel: (assetId: string) => void;
  onLoadMore: () => void;
  hasMore: boolean;
  isLoadingMore: boolean;
  isLoading?: boolean;
  columns?: number;
  className?: string;
}

export const DEFAULT_GROUP_LABELS = new Set(["All Results", "All Assets"]);
