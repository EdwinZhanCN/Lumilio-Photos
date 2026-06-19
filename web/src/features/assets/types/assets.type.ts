import { Asset } from "@/lib/assets/types";

import React from "react";
import type { components } from "@/lib/http-commons";

// ===== Core Types =====
export type AssetFilter = components["schemas"]["dto.AssetFilterDTO"];
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
}

export interface BrowseStackItem {
  type: "stack";
  id: `stack:${string}`;
  stackId: string;
  representative: Asset;
  assets: Asset[];
  memberAssetIds?: string[];
  matchedMemberIds?: string[];
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
  filter?: AssetFilter;
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

// ===== Entities State (DEPRECATED) =====
// Kept for compatibility, but effectively unused
export interface EntityMeta {
  lastUpdated: number;
  isOptimistic?: boolean;
  fetchOrigin?: string;
  signature?: string;
}

export interface EntitiesState {
  assets: Record<string, Asset>;
  meta: Record<string, EntityMeta>;
}

export interface AssetsPageInfo {
  cursor?: string;
  page?: number;
  total?: number;
}

// ===== UI State =====
export interface UIState {
  sortBy: SortByType;
  searchQuery: string;
  isCarouselOpen: boolean;
  activeAssetId?: string;
}

// ===== Filters State =====
export interface FiltersState {
  enabled: boolean;
  type?: "PHOTO" | "VIDEO";
  raw?: boolean;
  rating?: number;
  liked?: boolean;
  filename?: {
    mode: "contains" | "matches" | "startswith" | "endswith";
    value: string;
  };
  date?: {
    from?: string;
    to?: string;
  };
  camera_model?: string;
  lens?: string;
  tag_names?: string[];
  location?: {
    north: number;
    south: number;
    east: number;
    west: number;
  };
}

// ===== Selection State =====
export interface SelectionState {
  enabled: boolean;
  selectedIds: Set<string>;
  lastSelectedId?: string;
  selectionMode: "single" | "multiple";
}

// ===== Main State =====
export interface AssetsState extends SelectionState {
  entities: EntitiesState;
  ui: UIState;
  filters: FiltersState;
  selection: SelectionState;
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

export interface SelectionResult {
  enabled: boolean;
  selectedIds: Set<string>;
  selectedCount: number;
  isSelected: (assetId: string) => boolean;
  toggle: (assetId: string) => void;
  select: (assetId: string) => void;
  deselect: (assetId: string) => void;
  selectAll: (assetIds?: string[]) => void;
  clear: () => void;
  setEnabled: (enabled: boolean) => void;
  selectionMode: "single" | "multiple";
  setSelectionMode: (mode: "single" | "multiple") => void;

  // Extended operations
  selectRange: (
    fromAssetId: string,
    toAssetId: string,
    assetIds: string[],
  ) => void;
  toggleRange: (
    fromAssetId: string,
    toAssetId: string,
    assetIds: string[],
  ) => void;
  selectFiltered: (
    assetIds: string[],
    predicate: (assetId: string) => boolean,
  ) => void;
  deselectFiltered: (
    assetIds: string[],
    predicate: (assetId: string) => boolean,
  ) => void;
  invertSelection: (assetIds: string[]) => void;

  // Computed properties
  hasSelection: boolean;
  selectedAsArray: string[];
}

// ===== Context Value =====
export interface AssetsContextValue {
  state: AssetsState;
  dispatch: React.Dispatch<any>;
  // Navigation helpers
  openCarousel: (assetId: string) => void;
  closeCarousel: () => void;
}

// ===== Utility Types =====
export interface ViewDefinitionOptions {
  autoFetch?: boolean;
  disabled?: boolean;
  withGroups?: boolean;
  baseFilter?: AssetFilter;
  viewKey?: string;
}
