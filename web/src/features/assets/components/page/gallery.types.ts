import { AssetGroup } from "@/features/assets/types/assets.type";

export interface AssetGalleryProps {
  groups: AssetGroup[];
  openCarousel: (assetId: string) => void;
  onLoadMore: () => void;
  hasMore: boolean;
  isLoadingMore: boolean;
  isLoading?: boolean;
  columns?: number;
  className?: string;
}
