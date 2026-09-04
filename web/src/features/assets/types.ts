import { Asset } from "@/lib/assets/types";

import type { AssetBrowseConstraint, AssetUserFilter } from "./model/filter";

// ===== Core Types =====
export type AssetMediaType = "photos" | "videos" | "audios";
export type SortByType = "date_captured" | "recently_added";
export interface AssetGroup {
  key: string;
  assets: Asset[];
}
export type BrowseItemId = `media:${string}` | `stack:${string}`;

export type BrowseStackKind = "burst" | "manual";

/** Component facts of one logical media item (JPEG+RAW pair, Live Photo…). */
export interface MediaCompositionFacts {
  componentCount: number;
  hasRaw: boolean;
  hasJpeg: boolean;
  hasEdited: boolean;
  hasLiveMotion: boolean;
}

export interface BrowseMediaItem {
  type: "media_item";
  id: `media:${string}`;
  /** Logical media item id; equals the primary asset id on client-side grouping paths. */
  mediaItemId: string;
  /** Primary asset representing this logical media item. */
  asset: Asset;
  composition?: MediaCompositionFacts;
  /** Nearest matching video frame timestamp from semantic search (ms). */
  bestTsMs?: number;
}

/** One stack member at media-item granularity. */
export interface BrowseStackMemberRef {
  mediaItemId: string;
  primaryAssetId: string;
}

export interface BrowseStackItem {
  type: "stack";
  id: `stack:${string}`;
  stackId: string;
  stackKind: BrowseStackKind;
  /** Cover media item's primary asset. */
  representative: Asset;
  /** Loaded member primary assets (server rows only carry the cover). */
  assets: Asset[];
  members: BrowseStackMemberRef[];
  matchedMembers: BrowseStackMemberRef[];
  /** Nearest matching video frame timestamp from semantic search (ms). */
  bestTsMs?: number;
}

export type BrowseItem = BrowseMediaItem | BrowseStackItem;

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
    query?: string;
    similarAssetId?: string;
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
  error: unknown;
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
  similarAssetId?: string;
  fileQuery?: File | null;
  viewKey?: string;
}
