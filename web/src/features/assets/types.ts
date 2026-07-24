import { Asset } from "@/lib/assets/types";

import type { AssetBrowseConstraint, AssetUserFilter } from "./model/filter";

// ===== Core Types =====
export type AssetMediaType = "photos" | "videos" | "audios";
export type SortByType = "date_captured" | "recently_added";
export interface AssetGroup {
  key: string;
  assets: Asset[];
}
export type BrowseItemId = `asset:${string}` | `stack:${string}`;

export interface BrowseAssetItem {
  type: "asset";
  id: `asset:${string}`;
  asset: Asset;
  /** Nearest matching video frame timestamp from semantic search (ms). */
  bestTsMs?: number;
}

export interface BrowseStackItem {
  type: "stack";
  id: `stack:${string}`;
  stackId: string;
  representative: Asset;
  assets: Asset[];
  memberAssetIds?: string[];
  matchedMemberIds?: string[];
  /** Nearest matching video frame timestamp from semantic search (ms). */
  bestTsMs?: number;
}

export type BrowseItem = BrowseAssetItem | BrowseStackItem;

export interface BrowseGroup {
  key: string;
  items: BrowseItem[];
}

// ===== Asset View Definition =====
export interface AssetViewDefinition {
  /** Asset types to include */
  types?: AssetMediaType[];
  /** Filter conditions */
  filter?: AssetBrowseConstraint;
  /** @deprecated Filters are scoped explicitly by the caller. */
  inheritGlobalFilter?: boolean;
  /** Search configuration */
  search?: {
    query: string;
  };
  /** Sorting strategy */
  sortBy?: SortByType;
  /** Page size for pagination */
  pageSize?: number;
  /** Pagination mode */
  pagination?: "cursor" | "offset";
  /** Manual stable key for view caching */
  key?: string;
}

export interface AssetsPageInfo {
  cursor?: string;
  page?: number;
  total?: number;
}

// ===== Hook Return Types =====
export interface AssetsViewResult {
  assets: Asset[];
  groups?: AssetGroup[];
  browseGroups: BrowseGroup[];
  browseItems: BrowseItem[];
  browseAssets: Asset[];
  isLoading: boolean;
  isLoadingMore: boolean;
  isFetched: boolean;
  error: string | null;
  fetchMore: () => Promise<void>;
  refetch: () => Promise<void>;
  hasMore: boolean;
  viewKey: string;
  pageInfo: AssetsPageInfo;
}

export interface AssetActionsResult {
  updateRating: (assetId: string, rating: number) => Promise<void>;
  toggleLike: (assetId: string, isLiked: boolean) => Promise<void>;
  updateDescription: (assetId: string, description: string) => Promise<void>;
  deleteAsset: (assetId: string) => Promise<void>;
  batchUpdateAssets: (
    updates: Array<{
      assetId: string;
      updates: Partial<Asset>;
    }>,
  ) => Promise<void>;
  refreshAsset: () => Promise<void>;
}

// ===== Utility Types =====
export interface ViewDefinitionOptions {
  autoFetch?: boolean;
  disabled?: boolean;
  withGroups?: boolean;
  constraint?: AssetBrowseConstraint;
  userFilter?: AssetUserFilter;
  searchQuery?: string;
  viewKey?: string;
}
