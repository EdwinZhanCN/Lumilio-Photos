export { AssetsProvider } from "./AssetsProvider";
export { useAssetsContext } from "./hooks/useAssetsContext";
export { useAssetsView, useCurrentTabAssets } from "./hooks/useAssetsView";
export {
  useAsset,
  useAssets,
  useAssetExists,
  useAssetMeta,
} from "./hooks/useAsset";
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

// Export types
export type {
  AssetsState,
  AssetsContextValue,
  AssetViewDefinition,
  AssetsViewResult,
  AssetActionsResult,
  SelectionResult,
  TabType,
  GroupByType,
  ViewDefinitionOptions,
} from "./assets.types.ts";

// Export shared components
export { default as AssetsPageHeader } from "./components/shared/AssetsPageHeader";
export { default as JustifiedGallery } from "./components/page/JustifiedGallery/JustifiedGallery";

// Export utilities and selectors
export { generateViewKey } from "./reducers/views.reducer";
export {
  selectActiveFilterCount,
  selectHasActiveFilters,
  selectFilterAsAssetFilter,
} from "./reducers/filters.reducer";
export {
  selectTabTitle,
  selectTabSupportsSemanticSearch,
  selectTabAssetTypes,
} from "./reducers/ui.reducer";
