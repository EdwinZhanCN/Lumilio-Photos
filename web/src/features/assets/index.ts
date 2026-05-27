export { AssetsProvider } from "./AssetsProvider";
export { useAssetsView, useCurrentAssetsView } from "./hooks/useAssetsView";
// Export removed: useAsset and related hooks are deleted
export {
  useAssetActions,
  useAssetActionsSimple,
} from "./hooks/useAssetActions";
export {
  useSelection,
  useKeyboardSelection,
  useSelectionState,
  useBulkAssetOperations,
} from "./hooks/useSelection";
export { useAssetsNavigation } from "./hooks/useAssetsNavigation";

// Export selectors for fine-grained access
export * from "./selectors";

// Export scoped Zustand store helpers
export {
  createAssetsStore,
  useAssetsStore,
  useAssetsStoreApi,
} from "./assets.store";
export type {
  AssetsStore,
  AssetsStoreApi,
  AssetsStoreInitialState,
} from "./assets.store";

// Export types
export type {
  AssetsState,
  AssetViewDefinition,
  AssetsViewResult,
  AssetActionsResult,
  SelectionResult,
  AssetMediaType,
  SortByType,
  AssetGroup,
  BrowseItem,
  BrowseGroup,
  BrowseItemId,
  ViewDefinitionOptions,
} from "./types/assets.type";

// Re-export AssetsContextValue from types (for backwards compat)
export type { AssetsContextValue } from "./types/assets.type";

// Export shared components
export { default as AssetsPageHeader } from "./components/shared/AssetsPageHeader";
export { default as JustifiedGallery } from "./components/page/JustifiedGallery/JustifiedGallery";
export { default as SquareGallery } from "./components/page/SquareGallery/SquareGallery";
export type { AssetGalleryProps } from "./components/page/gallery.types";

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
} from "./slices/filters.slice";
