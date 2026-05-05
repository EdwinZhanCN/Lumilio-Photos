import {
  createContext,
  ReactNode,
  useEffect,
  useMemo,
  useCallback,
  useRef,
  useState,
} from "react";
import {
  useLocation,
  useNavigate,
  useSearchParams,
  useParams,
} from "react-router-dom";
import { useStore } from "zustand";
import { useShallow } from "zustand/react/shallow";
import {
  AssetFilter,
  FiltersState,
  SelectionState,
  TabType,
  UIState,
} from "./types/assets.type";
import {
  AssetsStoreInitialState,
  AssetsStoreApi,
  AssetsStoreContext,
  createAssetsStore,
} from "./assets.store";
import { useSettingsContext } from "@/features/settings";
import { isGroupByType, resolveGroupByFromUrl } from "./utils/groupBy";
import { mapFilenameOperatorToMode } from "./utils/filterUtils";
import {
  ASSETS_STATE_STORAGE_KEY,
  ASSETS_STATE_STORAGE_VERSION,
  LEGACY_ASSETS_STATE_STORAGE_KEY,
} from "@/lib/settings/registry";
import {
  isRecord,
  readVersionedStorageCandidate,
  writeVersionedStorageData,
} from "@/lib/settings/storage";

export const MAIN_ASSETS_SCOPE_ID = "assets:main";

interface AssetsNavigationContextValue {
  openCarousel: (assetId: string) => void;
  closeCarousel: () => void;
  switchTab: (tab: TabType) => void;
}

export const AssetsNavigationContext = createContext<
  AssetsNavigationContextValue | undefined
>(undefined);

type PersistedAssetsState = {
  filters?: Partial<FiltersState>;
  ui?: Partial<Pick<UIState, "groupBy" | "searchQuery">>;
  selection?: Partial<Pick<SelectionState, "selectionMode">>;
};

type AssetsRouteState = {
  assetsInitialFilter?: AssetFilter;
} | null;

function toStorageSnapshot(state: {
  filters: FiltersState;
  ui: UIState;
  selection: SelectionState;
}): PersistedAssetsState {
  return {
    filters: state.filters,
    ui: {
      groupBy: state.ui.groupBy,
      searchQuery: state.ui.searchQuery,
    },
    selection: {
      selectionMode: state.selection.selectionMode,
    },
  };
}

function writeStateToStorage(state: {
  filters: FiltersState;
  ui: UIState;
  selection: SelectionState;
}) {
  writeVersionedStorageData(
    ASSETS_STATE_STORAGE_KEY,
    ASSETS_STATE_STORAGE_VERSION,
    toStorageSnapshot(state),
  );
  localStorage.removeItem(LEGACY_ASSETS_STATE_STORAGE_KEY);
}

function loadPersistedState(): PersistedAssetsState {
  try {
    if (typeof localStorage === "undefined") {
      return {};
    }

    const readResult = readVersionedStorageCandidate({
      key: ASSETS_STATE_STORAGE_KEY,
      version: ASSETS_STATE_STORAGE_VERSION,
      legacyKeys: [LEGACY_ASSETS_STATE_STORAGE_KEY],
    });

    if (readResult.candidate === null || !isRecord(readResult.candidate)) {
      return {};
    }

    const candidate = readResult.candidate;
    const restored: PersistedAssetsState = {};

    if (isRecord(candidate.filters)) {
      restored.filters = candidate.filters as Partial<FiltersState>;
    }

    if (isRecord(candidate.ui)) {
      restored.ui = {
        groupBy: isGroupByType(candidate.ui.groupBy as string | null)
          ? (candidate.ui.groupBy as UIState["groupBy"])
          : undefined,
        searchQuery:
          typeof candidate.ui.searchQuery === "string"
            ? candidate.ui.searchQuery
            : undefined,
      };
    }

    if (isRecord(candidate.selection)) {
      const mode = candidate.selection.selectionMode;
      restored.selection = {
        selectionMode:
          mode === "single" || mode === "multiple" ? mode : undefined,
      };
    }

    if (readResult.needsRewrite) {
      const snapshot = createAssetsStore(restored).getState();
      writeStateToStorage(snapshot);
    }

    return restored;
  } catch (e) {
    console.warn("[AssetsProvider] Failed to parse stored state", e);
  }
  return {};
}

function saveStateToStorage(state: {
  filters: FiltersState;
  ui: UIState;
  selection: SelectionState;
}) {
  try {
    if (typeof localStorage === "undefined") {
      return;
    }

    writeStateToStorage(state);
  } catch (e) {
    console.warn("[AssetsProvider] Failed to persist state", e);
  }
}

function getLiveSearchParams(fallback: URLSearchParams): URLSearchParams {
  if (typeof window === "undefined") {
    return new URLSearchParams(fallback);
  }

  return new URLSearchParams(window.location.search);
}

function resolveTabFromPathname(pathname: string): TabType {
  if (pathname.includes("/videos")) return "videos";
  if (pathname.includes("/audios")) return "audios";
  return "photos";
}

function assetFilterToFiltersState(filter?: AssetFilter): Partial<FiltersState> {
  if (!filter) return {};

  const filters: Partial<FiltersState> = {
    enabled: Object.keys(filter).length > 0,
  };

  if (filter.raw !== undefined) filters.raw = filter.raw;
  if (filter.rating !== undefined) filters.rating = filter.rating;
  if (filter.liked !== undefined) filters.liked = filter.liked;
  if (filter.filename) {
    filters.filename = {
      mode: mapFilenameOperatorToMode(filter.filename.operator) ?? "contains",
      value: filter.filename.value ?? "",
    };
  }
  if (filter.date) {
    filters.date = {
      from: filter.date.from,
      to: filter.date.to,
    };
  }
  if (filter.camera_model) filters.camera_model = filter.camera_model;
  if (filter.lens) filters.lens = filter.lens;

  return filters;
}

interface AssetsProviderProps {
  children: ReactNode;
  scopeId: string;
  persist?: boolean;
  syncUrl?: boolean;
  basePath?: string;
  defaultSelectionMode?: "single" | "multiple";
  initialState?: AssetsStoreInitialState;
}

export const AssetsProvider = ({
  children,
  scopeId,
  persist = false,
  syncUrl = false,
  basePath,
  defaultSelectionMode = "multiple",
  initialState,
}: AssetsProviderProps) => {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const params = useParams<{ assetId?: string }>();
  const { state: settingsState } = useSettingsContext();
  const [isHydrated, setIsHydrated] = useState(false);
  const storeRef = useRef<AssetsStoreApi | null>(null);
  const isMainPersistentScope = persist && scopeId === MAIN_ASSETS_SCOPE_ID;

  if (storeRef.current === null) {
    storeRef.current = createAssetsStore({
      ...initialState,
      selection: {
        selectionMode: defaultSelectionMode,
        ...initialState?.selection,
      },
    });
  }

  const store = storeRef.current;
  const {
    setCurrentTab,
    setCarouselOpen,
    setActiveAssetId,
    hydrateUIFromURL,
    setSelectionMode,
    batchUpdateFilters,
  } = useStore(
    store,
    useShallow((state) => ({
      setCurrentTab: state.setCurrentTab,
      setCarouselOpen: state.setCarouselOpen,
      setActiveAssetId: state.setActiveAssetId,
      hydrateUIFromURL: state.hydrateUIFromURL,
      setSelectionMode: state.setSelectionMode,
      batchUpdateFilters: state.batchUpdateFilters,
    })),
  );

  const uiState = useStore(store, useShallow((state) => state.ui));
  const filtersState = useStore(store, useShallow((state) => state.filters));
  const selectionState = useStore(
    store,
    useShallow((state) => state.selection),
  );

  const initialized = useRef(false);
  useEffect(() => {
    if (initialized.current) return;
    initialized.current = true;

    const liveSearchParams = getLiveSearchParams(searchParams);
    const urlGroupBy = liveSearchParams.get("groupBy");
    const urlQuery = liveSearchParams.get("q");
    const resolvedGroupBy = resolveGroupByFromUrl(
      urlGroupBy,
      settingsState.ui.asset_page?.layout,
    );
    const persistedState = isMainPersistentScope ? loadPersistedState() : {};
    const initialGroupBy =
      syncUrl && isGroupByType(urlGroupBy)
        ? resolvedGroupBy
        : persistedState.ui?.groupBy ?? resolvedGroupBy;
    const initialSearchQuery =
      syncUrl && urlQuery !== null
        ? urlQuery
        : persistedState.ui?.searchQuery ?? "";
    const routeState = location.state as AssetsRouteState;
    const routeFilter = assetFilterToFiltersState(
      routeState?.assetsInitialFilter,
    );

    if (syncUrl) {
      setCurrentTab(resolveTabFromPathname(location.pathname));
    }

    if (persistedState.filters) {
      batchUpdateFilters(persistedState.filters);
    }

    if (Object.keys(routeFilter).length > 0) {
      batchUpdateFilters(routeFilter);
    }

    if (persistedState.selection?.selectionMode) {
      setSelectionMode(persistedState.selection.selectionMode);
    } else {
      setSelectionMode(defaultSelectionMode);
    }

    hydrateUIFromURL({
      groupBy: syncUrl ? initialGroupBy : persistedState.ui?.groupBy ?? uiState.groupBy,
      searchQuery: syncUrl
        ? initialSearchQuery
        : persistedState.ui?.searchQuery ?? uiState.searchQuery,
    });

    if (syncUrl && !isGroupByType(urlGroupBy)) {
      const params = new URLSearchParams(liveSearchParams);
      params.set("groupBy", initialGroupBy);
      setSearchParams(params, { replace: true });
    }

    if (routeState?.assetsInitialFilter) {
      navigate(`${location.pathname}${location.search}`, {
        replace: true,
        state: null,
      });
    }

    setIsHydrated(true);
  }, [
    batchUpdateFilters,
    defaultSelectionMode,
    hydrateUIFromURL,
    isMainPersistentScope,
    location.pathname,
    location.search,
    location.state,
    navigate,
    searchParams,
    setCurrentTab,
    setSearchParams,
    setSelectionMode,
    settingsState.ui.asset_page?.layout,
    syncUrl,
    uiState.groupBy,
    uiState.searchQuery,
  ]);

  useEffect(() => {
    if (!isMainPersistentScope || !isHydrated) return;

    saveStateToStorage({
      filters: filtersState,
      ui: uiState,
      selection: selectionState,
    });
  }, [filtersState, isHydrated, isMainPersistentScope, selectionState, uiState]);

  useEffect(() => {
    if (!syncUrl || !isHydrated) return;

    const currentTab = resolveTabFromPathname(location.pathname);
    if (currentTab !== uiState.currentTab) {
      setCurrentTab(currentTab);
    }
  }, [
    isHydrated,
    location.pathname,
    setCurrentTab,
    syncUrl,
    uiState.currentTab,
  ]);

  useEffect(() => {
    if (!isHydrated) return;

    const isCarouselOpen = !!params.assetId;
    const activeAssetId = params.assetId;

    setCarouselOpen(isCarouselOpen);
    setActiveAssetId(activeAssetId);
  }, [isHydrated, params.assetId, setActiveAssetId, setCarouselOpen]);

  useEffect(() => {
    if (!syncUrl || !isHydrated) return;

    const liveSearchParams = getLiveSearchParams(searchParams);
    const urlGroupBy = liveSearchParams.get("groupBy");
    const urlQuery = liveSearchParams.get("q");
    const resolvedGroupBy = resolveGroupByFromUrl(
      urlGroupBy,
      settingsState.ui.asset_page?.layout,
    );
    const resolvedQuery = urlQuery ?? "";

    if (urlGroupBy !== null && resolvedGroupBy !== uiState.groupBy) {
      hydrateUIFromURL({ groupBy: resolvedGroupBy });
    }

    if (urlQuery !== null && resolvedQuery !== uiState.searchQuery) {
      hydrateUIFromURL({ searchQuery: resolvedQuery });
    }
  }, [
    hydrateUIFromURL,
    isHydrated,
    searchParams,
    settingsState.ui.asset_page?.layout,
    syncUrl,
    uiState.groupBy,
    uiState.searchQuery,
  ]);

  useEffect(() => {
    if (!syncUrl || !isHydrated) return;

    const liveSearchParams = getLiveSearchParams(searchParams);
    const nextParams = new URLSearchParams(liveSearchParams);
    nextParams.set("groupBy", uiState.groupBy);
    if (uiState.searchQuery.trim()) {
      nextParams.set("q", uiState.searchQuery.trim());
    } else {
      nextParams.delete("q");
    }

    if (nextParams.toString() !== liveSearchParams.toString()) {
      setSearchParams(nextParams, { replace: true });
    }
  }, [
    isHydrated,
    searchParams,
    setSearchParams,
    syncUrl,
    uiState.groupBy,
    uiState.searchQuery,
  ]);

  const openCarousel = useCallback(
    (assetId: string) => {
      const currentParams = syncUrl ? getLiveSearchParams(searchParams) : null;
      let path = basePath || "/assets/photos";

      if (!basePath) {
        if (uiState.currentTab === "videos") path = "/assets/videos";
        else if (uiState.currentTab === "audios") path = "/assets/audios";
      }

      const query = currentParams?.toString();
      navigate(`${path}/${assetId}${query ? `?${query}` : ""}`);
    },
    [basePath, navigate, searchParams, syncUrl, uiState.currentTab],
  );

  const closeCarousel = useCallback(() => {
    const currentParams = syncUrl ? getLiveSearchParams(searchParams) : null;
    let path = basePath || "/assets/photos";

    if (!basePath) {
      if (uiState.currentTab === "videos") path = "/assets/videos";
      else if (uiState.currentTab === "audios") path = "/assets/audios";
    }

    const query = currentParams?.toString();
    navigate(`${path}${query ? `?${query}` : ""}`);
  }, [basePath, navigate, searchParams, syncUrl, uiState.currentTab]);

  const switchTab = useCallback(
    (tab: TabType) => {
      const currentParams = syncUrl ? getLiveSearchParams(searchParams) : null;
      let path = "/assets/photos";
      if (tab === "videos") path = "/assets/videos";
      else if (tab === "audios") path = "/assets/audios";
      const query = currentParams?.toString();
      navigate(`${path}${query ? `?${query}` : ""}`);
    },
    [navigate, searchParams, syncUrl],
  );

  const contextValue = useMemo<AssetsNavigationContextValue>(
    () => ({
      openCarousel,
      closeCarousel,
      switchTab,
    }),
    [openCarousel, closeCarousel, switchTab],
  );

  return (
    <AssetsStoreContext.Provider value={store}>
      <AssetsNavigationContext.Provider value={contextValue}>
        {isHydrated ? children : null}
      </AssetsNavigationContext.Provider>
    </AssetsStoreContext.Provider>
  );
};
