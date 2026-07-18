export { AssetsProvider } from "./state/AssetsProvider";
export { useAssetsView, useCurrentAssetsView } from "./api/useAssetsView";
// Export removed: useAsset and related hooks are deleted
export { useAssetActions } from "./api/useAssetActions";
export {
  useSelection,
  useKeyboardSelection,
  useSelectionState,
  useBulkAssetOperations,
} from "./hooks/useSelection";
export { useAssetsNavigation } from "./hooks/useAssetsNavigation";
export { useAssetFilterOptions } from "./api/useAssetFilterOptions";
export { useVisibleOnce } from "./hooks/useVisibleOnce";

// Export selectors for fine-grained access
export * from "./state/selectors";

// Export scoped Zustand store helpers
export { createAssetsStore, useAssetsStore, useAssetsStoreApi } from "./state/store";
export type { AssetsStore, AssetsStoreApi, AssetsStoreInitialState } from "./state/store";

// Export types
export type {
  AssetsState,
  AssetViewDefinition,
  AssetsViewResult,
  AssetActionsResult,
  SelectionResult,
  AssetMediaType,
  AssetFilter,
  SortByType,
  AssetGroup,
  BrowseItem,
  BrowseGroup,
  BrowseItemId,
  ViewDefinitionOptions,
} from "./types";

// Re-export AssetsContextValue from types (for backwards compat)
export type { AssetsContextValue } from "./types";

// Export shared components
export { default as AssetsPageHeader } from "./components/browse/AssetsPageHeader";
export { default as JustifiedGallery } from "./components/browse/JustifiedGallery/JustifiedGallery";
export { default as SquareGallery } from "./components/browse/SquareGallery/SquareGallery";
export { AssetsGalleryPage } from "./components/browse/AssetsGalleryPage";
export type { AssetGalleryProps } from "./components/browse/gallery.types";

// Export utilities and selectors from slices
export { generateViewKey } from "./utils/viewKey";
export {
  createBrowseGroupsFromAssets,
  createBrowseGroupsFromAssetGroups,
  dedupeBrowseItemsById,
  findBrowseItemById,
  findBrowseItemIndexByAssetId,
  flattenBrowseGroups,
  flattenBrowseGroupsToAssets,
  getBrowseItemAsset,
  getBrowseItemAssetId,
  resolveBrowseSelectedAssetIds,
  resolveSelectedBrowseItems,
} from "./utils/browseItems";
export {
  selectActiveFilterCount,
  selectHasActiveFilters,
  selectFilterAsAssetFilter,
} from "./state/slices/filters.slice";
