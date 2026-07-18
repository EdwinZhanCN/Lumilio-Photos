import { useCallback, useEffect, useMemo, useRef } from "react";
import { useSearchParams } from "react-router-dom";

import type { AssetUserFilter } from "../../model/filter";
import type { SortByType } from "../../types";
import {
  DEFAULT_ASSET_BROWSE_SORT,
  hasAssetBrowseParams,
  parseAssetBrowseParams,
  serializeAssetBrowseParams,
} from "../../model/browseRouteState";
import {
  clearLegacyAssetBrowseState,
  readLegacyAssetBrowseState,
} from "../../state/legacyBrowseStateMigration";

type RouteUpdateOptions = {
  replace?: boolean;
};

type UseAssetBrowseRouteStateOptions = {
  defaultSort?: SortByType;
  migrateLegacyState?: boolean;
};

export function useAssetBrowseRouteState({
  defaultSort = DEFAULT_ASSET_BROWSE_SORT,
  migrateLegacyState = false,
}: UseAssetBrowseRouteStateOptions = {}) {
  const [searchParams, setSearchParams] = useSearchParams();
  const migrationCheckedRef = useRef(false);
  const state = useMemo(
    () => parseAssetBrowseParams(searchParams, defaultSort),
    [defaultSort, searchParams],
  );

  useEffect(() => {
    if (migrationCheckedRef.current || !migrateLegacyState) return;
    migrationCheckedRef.current = true;
    if (hasAssetBrowseParams(searchParams)) return;

    const legacyState = readLegacyAssetBrowseState();
    if (!legacyState) return;
    const migratedParams = serializeAssetBrowseParams(legacyState, searchParams, defaultSort);
    setSearchParams(migratedParams, { replace: true });
    clearLegacyAssetBrowseState();
  }, [defaultSort, migrateLegacyState, searchParams, setSearchParams]);

  const updateState = useCallback(
    (
      update: (current: typeof state) => typeof state,
      { replace = false }: RouteUpdateOptions = {},
    ) => {
      const current = parseAssetBrowseParams(searchParams, defaultSort);
      const next = update(current);
      const nextParams = serializeAssetBrowseParams(next, searchParams, defaultSort);
      if (nextParams.toString() === searchParams.toString()) return;
      setSearchParams(nextParams, { replace });
    },
    [defaultSort, searchParams, setSearchParams],
  );

  const setQuery = useCallback(
    (query: string, options: RouteUpdateOptions = { replace: true }) => {
      updateState((current) => ({ ...current, query }), options);
    },
    [updateState],
  );

  const setSort = useCallback(
    (sort: SortByType, options: RouteUpdateOptions = { replace: true }) => {
      updateState((current) => ({ ...current, sort }), options);
    },
    [updateState],
  );

  const applyFilter = useCallback(
    (filter: AssetUserFilter, options: RouteUpdateOptions = { replace: false }) => {
      updateState((current) => ({ ...current, filter }), options);
    },
    [updateState],
  );

  const resetFilter = useCallback(
    (options: RouteUpdateOptions = { replace: false }) => {
      updateState((current) => ({ ...current, filter: {} }), options);
    },
    [updateState],
  );

  return {
    ...state,
    setQuery,
    setSort,
    applyFilter,
    resetFilter,
  };
}
