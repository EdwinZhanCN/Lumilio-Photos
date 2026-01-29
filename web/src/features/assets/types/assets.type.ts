import { Asset } from "@/lib/assets/types";

import React from "react";
import type { components } from "@/lib/http-commons";

// ===== Core Types =====
export type AssetFilter = components["schemas"]["dto.AssetFilterDTO"];
export type TabType = "photos" | "videos" | "audios";
export type GroupByType = "date" | "type" | "album" | "flat";

// ===== Asset View Definition =====
export interface AssetViewDefinition {
  /** Asset types to include */
  types?: TabType[];
  /** Filter conditions */
  filter?: AssetFilter;
  /** Whether to inherit global filters */
  inheritGlobalFilter?: boolean;
  /** Search configuration */
  search?: {
    query: string;
    mode: "filename" | "semantic";
  };
  /** Grouping strategy */
  groupBy?: GroupByType;
  /** Sorting configuration */
  sort?: {
    direction: "desc" | "asc";
  };
  /** Page size for pagination */
  pageSize?: number;
  /** Pagination mode */
  pagination?: "cursor" | "offset";
  /** Manual stable key for view caching */
  key?: string;
}

// ===== Entities State =====
export interface EntityMeta {
  lastUpdated: number;
  isOptimistic?: boolean;
  fetchOrigin?: string;
}

export interface EntitiesState {
  assets: Record<string, Asset>;
  meta: Record<string, EntityMeta>;
}

// ===== Views State =====
export interface ViewState {
  assetIds: string[];
  isLoading: boolean;
  isLoadingMore: boolean;
  hasMore: boolean;
  error: string | null;
  pageInfo: {
    cursor?: string;
    page?: number;
    total?: number;
  };
  definitionHash: string;
  lastFetchAt: number;
}

export interface ViewsState {
  views: Record<string, ViewState>;
  activeViewKeys: string[];
}

// ===== UI State =====
export interface UIState {
  currentTab: TabType;
  groupBy: GroupByType;
  searchQuery: string;
  searchMode: "filename" | "semantic";
  isCarouselOpen: boolean;
  activeAssetId?: string;
}

// ===== Filters State =====
export interface FiltersState {
  enabled: boolean;
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
  camera_make?: string;
  lens?: string;
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
  views: ViewsState;
  ui: UIState;
  filters: FiltersState;
  selection: SelectionState;
}

// ===== Hook Return Types =====
export interface AssetsViewResult {
  assets: Asset[];
  groups?: Record<string, Asset[]>;
  isLoading: boolean;
  isLoadingMore: boolean;
  error: string | null;
  fetchMore: () => Promise<void>;
  refetch: () => Promise<void>;
  hasMore: boolean;
  viewKey: string;
  pageInfo: ViewState["pageInfo"];
}

export interface AssetActionsResult {
  updateRating: (assetId: string, rating: number) => Promise<void>;
  toggleLike: (assetId: string) => Promise<void>;
  updateDescription: (assetId: string, description: string) => Promise<void>;
  deleteAsset: (assetId: string) => Promise<void>;
  batchUpdateAssets: (
    updates: Array<{
      assetId: string;
      updates: Partial<Asset>;
    }>,
  ) => Promise<void>;
  refreshAsset: (assetId: string) => Promise<void>;
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
  switchTab: (tab: TabType) => void;
}

// ===== Utility Types =====
export interface ViewDefinitionOptions {
  autoFetch?: boolean;
  disabled?: boolean;
  withGroups?: boolean;
}
