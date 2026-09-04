import { useMemo } from "react";

import { useBrowseScope } from "@/features/repositories";

import {
  DEFAULT_ASSET_TYPES,
  DEFAULT_TOP_RESULTS_META,
  type AssetBrowserViewResult,
} from "../../api/assetViewModel";
import { useAssetsList } from "../../api/useAssetsList";
import { useAssetsSearch } from "../../api/useAssetsSearch";
import { mergeAssetFilters } from "../../model/filter";
import type { AssetViewDefinition, SortByType, ViewDefinitionOptions } from "../../types";

export type AssetBrowserOptions = ViewDefinitionOptions & {
  sortBy?: SortByType;
  pageSize?: number;
};

const EMPTY_SEARCH_FIELDS = {
  topResults: [],
  resultAssets: [],
  resultGroups: [],
  topResultsBrowseGroups: [],
  resultBrowseGroups: [],
  topResultsMeta: DEFAULT_TOP_RESULTS_META,
} satisfies Pick<
  AssetBrowserViewResult,
  | "topResults"
  | "resultAssets"
  | "resultGroups"
  | "topResultsBrowseGroups"
  | "resultBrowseGroups"
  | "topResultsMeta"
>;

export function useAssetBrowser(options: AssetBrowserOptions = {}): AssetBrowserViewResult {
  const {
    sortBy = "date_captured",
    pageSize = 50,
    constraint,
    userFilter = {},
    searchQuery = "",
    similarAssetId = "",
    fileQuery = null,
    viewKey,
    ...viewOptions
  } = options;
  const { scopedRepositoryId } = useBrowseScope();
  const scopedConstraint = useMemo(
    () =>
      constraint?.repository_id || !scopedRepositoryId
        ? constraint
        : { ...constraint, repository_id: scopedRepositoryId },
    [constraint, scopedRepositoryId],
  );
  const effectiveFilter = useMemo(
    () => mergeAssetFilters(userFilter, scopedConstraint),
    [scopedConstraint, userFilter],
  );
  const normalizedQuery = searchQuery.trim();
  const normalizedSimilar = similarAssetId.trim();
  const fileIdentity = fileQuery
    ? `${fileQuery.name}:${fileQuery.size}:${fileQuery.lastModified}`
    : "";
  const searchActive =
    normalizedQuery.length > 0 || normalizedSimilar.length > 0 || Boolean(fileQuery);
  const definition = useMemo<AssetViewDefinition>(
    () => ({
      types: DEFAULT_ASSET_TYPES,
      filter: effectiveFilter,
      sortBy,
      pageSize,
      key: fileIdentity ? `${viewKey ?? ""}:file:${fileIdentity}` : viewKey,
      search: searchActive
        ? {
            query: normalizedQuery || undefined,
            similarAssetId: normalizedSimilar || undefined,
          }
        : undefined,
    }),
    [
      effectiveFilter,
      fileIdentity,
      normalizedQuery,
      normalizedSimilar,
      pageSize,
      searchActive,
      sortBy,
      viewKey,
    ],
  );

  const listView = useAssetsList(definition, {
    ...viewOptions,
    withGroups: true,
    disabled: viewOptions.disabled || searchActive,
  });
  const searchView = useAssetsSearch(definition, {
    ...viewOptions,
    fileQuery,
    withGroups: viewOptions.withGroups ?? true,
    disabled: viewOptions.disabled || !searchActive,
  });

  return searchActive
    ? searchView
    : {
        ...EMPTY_SEARCH_FIELDS,
        ...listView,
      };
}
