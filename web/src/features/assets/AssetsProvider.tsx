import {
  createContext,
  ReactNode,
  useEffect,
  useMemo,
  useCallback,
  useRef,
} from "react";
import {
  useLocation,
  useNavigate,
  useSearchParams,
  useParams,
} from "react-router-dom";
import { TabType } from "./types/assets.type";
import { useSettingsContext } from "@/features/settings";
import { useAssetsStore } from "./assets.store";
import { useShallow } from "zustand/react/shallow";
import { isGroupByType, resolveGroupByFromUrl } from "./utils/groupBy";

// Context for navigation helpers which depend on router
interface AssetsNavigationContextValue {
  openCarousel: (assetId: string) => void;
  closeCarousel: () => void;
  switchTab: (tab: TabType) => void;
}

export const AssetsNavigationContext = createContext<
  AssetsNavigationContextValue | undefined
>(undefined);

const STORAGE_KEY = "assets_state_v1";
const STORAGE_FIELDS = ["filters", "selection"] as const;

interface AssetsProviderProps {
  children: ReactNode;
  persist?: boolean;
  basePath?: string; // Optional base path for carousel navigation
  defaultSelectionMode?: "single" | "multiple";
}

function loadPersistedState() {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      const restored: any = {};

      STORAGE_FIELDS.forEach((field) => {
        if (parsed[field]) {
          if (field === "selection") {
            restored[field] = {
              ...parsed[field],
              selectedIds: new Set(parsed[field].selectedIds || []),
            };
          } else {
            restored[field] = parsed[field];
          }
        }
      });

      return restored;
    }
  } catch (e) {
    console.warn("[AssetsProvider] Failed to parse stored state", e);
  }
  return {};
}

function saveStateToStorage(state: any) {
  try {
    const toSave: any = {};

    STORAGE_FIELDS.forEach((field) => {
      if (state[field]) {
        if (field === "selection") {
          toSave[field] = {
            ...state[field],
            selectedIds: Array.from(state[field].selectedIds),
          };
        } else {
          toSave[field] = state[field];
        }
      }
    });

    localStorage.setItem(STORAGE_KEY, JSON.stringify(toSave));
  } catch (e) {
    console.warn("[AssetsProvider] Failed to persist state", e);
  }
}

export const AssetsProvider = ({
  children,
  persist = true,
  basePath,
  defaultSelectionMode = "multiple",
}: AssetsProviderProps) => {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const params = useParams<{ assetId?: string }>();
  const { state: settingsState } = useSettingsContext();

  // Store actions
  const {
    setCurrentTab,
    setCarouselOpen,
    setActiveAssetId,
    hydrateUIFromURL,
    setSelectionMode,
    batchUpdateFilters,
    setSelectionEnabled,
    selectAll,
  } = useAssetsStore(
    useShallow((state) => ({
      setCurrentTab: state.setCurrentTab,
      setCarouselOpen: state.setCarouselOpen,
      setActiveAssetId: state.setActiveAssetId,
      hydrateUIFromURL: state.hydrateUIFromURL,
      setSelectionMode: state.setSelectionMode,
      batchUpdateFilters: state.batchUpdateFilters,
      setSelectionEnabled: state.setSelectionEnabled,
      selectAll: state.selectAll,
    })),
  );

  // Store state for effects
  const uiState = useAssetsStore(useShallow((state) => state.ui));
  const filtersState = useAssetsStore(useShallow((state) => state.filters));
  const selectionState = useAssetsStore(useShallow((state) => state.selection));

  // Initialize state from storage and URL
  const initialized = useRef(false);
  useEffect(() => {
    if (initialized.current) return;
    initialized.current = true;

    // Load persisted state
    if (persist) {
      const persistedState = loadPersistedState();
      if (persistedState.filters) {
        batchUpdateFilters(persistedState.filters);
      }
      if (persistedState.selection) {
        // We need to manually reconstruct the selection state
        // because the slice actions expect specific calls
        if (persistedState.selection.enabled) {
          setSelectionEnabled(true);
        }
        if (persistedState.selection.selectionMode) {
          setSelectionMode(persistedState.selection.selectionMode);
        }
        if (persistedState.selection.selectedIds?.size > 0) {
          selectAll(Array.from(persistedState.selection.selectedIds));
        }
      }
    }

    // Set default selection mode
    if (!persist || !loadPersistedState().selection) {
      setSelectionMode(defaultSelectionMode);
    }
  }, [
    persist,
    defaultSelectionMode,
    batchUpdateFilters,
    setSelectionEnabled,
    setSelectionMode,
    selectAll,
  ]);

  // Persist state changes
  useEffect(() => {
    if (persist) {
      saveStateToStorage({
        filters: filtersState,
        selection: selectionState,
      });
    }
  }, [filtersState, selectionState, persist]);

  // Sync Tab with URL
  useEffect(() => {
    const currentTab: TabType = location.pathname.includes("/videos")
      ? "videos"
      : location.pathname.includes("/audios")
        ? "audios"
        : "photos";

    if (currentTab !== uiState.currentTab) {
      setCurrentTab(currentTab);
    }
  }, [location.pathname, uiState.currentTab, setCurrentTab]);

  // Sync Carousel with URL
  useEffect(() => {
    const isCarouselOpen = !!params.assetId;
    const activeAssetId = params.assetId;

    setCarouselOpen(isCarouselOpen);
    setActiveAssetId(activeAssetId);
  }, [params.assetId, setCarouselOpen, setActiveAssetId]);

  // Sync UI state from URL params
  useEffect(() => {
    const urlGroupBy = searchParams.get("groupBy");
    const urlQuery = searchParams.get("q");
    const resolvedGroupBy = resolveGroupByFromUrl(
      urlGroupBy,
      settingsState.ui.asset_page?.layout,
    );
    const resolvedQuery = urlQuery ?? "";

    if (resolvedGroupBy !== uiState.groupBy) {
      hydrateUIFromURL({ groupBy: resolvedGroupBy as any });
    }

    if (resolvedQuery !== uiState.searchQuery) {
      hydrateUIFromURL({ searchQuery: resolvedQuery });
    }

    if (!isGroupByType(urlGroupBy)) {
      const params = new URLSearchParams(searchParams);
      params.set("groupBy", resolvedGroupBy);
      setSearchParams(params, { replace: true });
    }
  }, [
    searchParams,
    settingsState.ui.asset_page?.layout,
    uiState.groupBy,
    uiState.searchQuery,
    hydrateUIFromURL,
    setSearchParams,
  ]);


  const openCarousel = useCallback(
    (assetId: string) => {
      const currentParams = new URLSearchParams(searchParams);
      let path = basePath || "/assets/photos";

      if (!basePath) {
        if (uiState.currentTab === "videos") path = "/assets/videos";
        else if (uiState.currentTab === "audios") path = "/assets/audios";
      }

      const targetUrl = `${path}/${assetId}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`;
      navigate(targetUrl);
    },
    [navigate, searchParams, uiState.currentTab, basePath],
  );

  const closeCarousel = useCallback(() => {
    const currentParams = new URLSearchParams(searchParams);
    let path = basePath || "/assets/photos";

    if (!basePath) {
      if (uiState.currentTab === "videos") path = "/assets/videos";
      else if (uiState.currentTab === "audios") path = "/assets/audios";
    }

    const targetUrl = `${path}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`;
    navigate(targetUrl);
  }, [navigate, searchParams, uiState.currentTab, basePath]);

  const switchTab = useCallback(
    (tab: TabType) => {
      const currentParams = new URLSearchParams(searchParams);
      let path = "/assets/photos";
      if (tab === "videos") path = "/assets/videos";
      else if (tab === "audios") path = "/assets/audios";
      const targetUrl = `${path}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`;
      navigate(targetUrl);
    },
    [navigate, searchParams],
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
    <AssetsNavigationContext.Provider value={contextValue}>
      {children}
    </AssetsNavigationContext.Provider>
  );
};
