import { AssetFilter } from "@/services/assetsService";

// ===== Core Types =====
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
    field: "taken_time" | "rating" | "upload_time";
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
export interface AssetsState {
  entities: EntitiesState;
  views: ViewsState;
  ui: UIState;
  filters: FiltersState;
  selection: SelectionState;
}

// ===== Actions =====
export type AssetsAction =
  // Entity Actions
  | {
      type: "SET_ENTITY";
      payload: { assetId: string; asset: Asset; meta?: Partial<EntityMeta> };
    }
  | {
      type: "UPDATE_ENTITY";
      payload: {
        assetId: string;
        updates: Partial<Asset>;
        meta?: Partial<EntityMeta>;
      };
    }
  | { type: "DELETE_ENTITY"; payload: { assetId: string } }
  | {
      type: "BATCH_SET_ENTITIES";
      payload: { assets: Asset[]; meta?: Record<string, Partial<EntityMeta>> };
    }

  // View Actions
  | {
      type: "CREATE_VIEW";
      payload: { viewKey: string; definition: AssetViewDefinition };
    }
  | { type: "SET_VIEW_LOADING"; payload: { viewKey: string; loading: boolean } }
  | {
      type: "SET_VIEW_ASSETS";
      payload: {
        viewKey: string;
        assetIds: string[];
        hasMore: boolean;
        pageInfo: ViewState["pageInfo"];
        replace?: boolean;
      };
    }
  | {
      type: "APPEND_VIEW_ASSETS";
      payload: {
        viewKey: string;
        assetIds: string[];
        hasMore: boolean;
        pageInfo: ViewState["pageInfo"];
      };
    }
  | {
      type: "SET_VIEW_ERROR";
      payload: { viewKey: string; error: string | null };
    }
  | {
      type: "SET_VIEW_LOADING_MORE";
      payload: { viewKey: string; loading: boolean };
    }
  | { type: "REMOVE_VIEW"; payload: { viewKey: string } }
  | { type: "REMOVE_ASSET_FROM_VIEWS"; payload: { assetId: string } }

  // UI Actions
  | { type: "SET_CURRENT_TAB"; payload: TabType }
  | { type: "SET_GROUP_BY"; payload: GroupByType }
  | { type: "SET_SEARCH_QUERY"; payload: string }
  | { type: "SET_SEARCH_MODE"; payload: "filename" | "semantic" }
  | { type: "SET_CAROUSEL_OPEN"; payload: boolean }
  | { type: "SET_ACTIVE_ASSET_ID"; payload: string | undefined }
  | {
      type: "HYDRATE_UI_FROM_URL";
      payload: Partial<Pick<UIState, "groupBy" | "searchQuery">>;
    }

  // Filter Actions
  | { type: "SET_FILTERS_ENABLED"; payload: boolean }
  | { type: "SET_FILTER_RAW"; payload: boolean | undefined }
  | { type: "SET_FILTER_RATING"; payload: number | undefined }
  | { type: "SET_FILTER_LIKED"; payload: boolean | undefined }
  | { type: "SET_FILTER_FILENAME"; payload: FiltersState["filename"] }
  | { type: "SET_FILTER_DATE"; payload: FiltersState["date"] }
  | { type: "SET_FILTER_CAMERA_MAKE"; payload: string | undefined }
  | { type: "SET_FILTER_LENS"; payload: string | undefined }
  | { type: "RESET_FILTERS" }
  | { type: "BATCH_UPDATE_FILTERS"; payload: Partial<FiltersState> }

  // Selection Actions
  | { type: "SET_SELECTION_ENABLED"; payload: boolean }
  | { type: "TOGGLE_ASSET_SELECTION"; payload: { assetId: string } }
  | { type: "SELECT_ASSET"; payload: { assetId: string } }
  | { type: "DESELECT_ASSET"; payload: { assetId: string } }
  | { type: "SELECT_ALL"; payload: { assetIds: string[] } }
  | { type: "CLEAR_SELECTION" }
  | { type: "SET_SELECTION_MODE"; payload: "single" | "multiple" };

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
  dispatch: React.Dispatch<AssetsAction>;
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
