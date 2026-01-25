export { AssetsProvider } from "./AssetsProvider";
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
export { useAssetsNavigation } from "./hooks/useAssetsNavigation";

// Export selectors for fine-grained access
export * from "./selectors";

// Export Zustand store
export { useAssetsStore } from "./assets.store";

// Export types
export type {
  AssetsState,
  AssetViewDefinition,
  AssetsViewResult,
  AssetActionsResult,
  SelectionResult,
  TabType,
  GroupByType,
  ViewDefinitionOptions,
} from "./types/assets.type";

// Re-export AssetsContextValue from types (for backwards compat)
export type { AssetsContextValue } from "./types/assets.type";

// Export shared components
export { default as AssetsPageHeader } from "./components/shared/AssetsPageHeader";
export { default as JustifiedGallery } from "./components/page/JustifiedGallery/JustifiedGallery";

// Export utilities and selectors from slices
export { generateViewKey } from "./slices/views.slice";
export {
  selectActiveFilterCount,
  selectHasActiveFilters,
  selectFilterAsAssetFilter,
} from "./slices/filters.slice";
export {
  selectTabTitle,
  selectTabSupportsSemanticSearch,
  selectTabAssetTypes,
} from "./slices/ui.slice";

