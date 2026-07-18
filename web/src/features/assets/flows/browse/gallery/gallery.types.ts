import type { BrowseGroup } from "../../../types";

export interface AssetGalleryProps {
  browseGroups: BrowseGroup[];
  openCarousel: (assetId: string) => void;
  onLoadMore: () => void;
  hasMore: boolean;
  isLoadingMore: boolean;
  isLoading?: boolean;
  columns?: number;
  className?: string;
  emptyStateTitle?: string;
  emptyStateDescription?: string;
}
