import {
  createContext,
  useReducer,
  ReactNode,
  useEffect,
  useMemo,
  useCallback,
} from "react";
import {
  useLocation,
  useNavigate,
  useSearchParams,
  useParams,
} from "react-router-dom";
import { AssetsContextValue, AssetsState, TabType } from "./types";
import { assetsReducer, initialAssetsState } from "./assets.reducer";
import { useSettingsContext } from "@/features/settings";

export const AssetsContext = createContext<AssetsContextValue | undefined>(
  undefined,
);

const STORAGE_KEY = "assets_state_v1";
const STORAGE_FIELDS = ["filters", "selection"] as const;

interface AssetsProviderProps {
  children: ReactNode;
}

function loadPersistedState(): Partial<AssetsState> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      // Only restore specific fields, not the entire state
      const restored: Partial<AssetsState> = {};

      STORAGE_FIELDS.forEach((field) => {
        if (parsed[field]) {
          if (field === "selection") {
            // Convert selectedIds array back to Set
            restored[field] = {
              ...parsed[field],
              selectedIds: new Set(parsed[field].selectedIds || []),
            } as any;
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

function saveStateToStorage(state: AssetsState) {
  try {
    const toSave: Partial<AssetsState> = {};

    STORAGE_FIELDS.forEach((field) => {
      if (field === "selection") {
        // Convert Set to array for JSON serialization
        toSave[field] = {
          ...state[field],
          selectedIds: Array.from(state[field].selectedIds),
        } as any;
      } else {
        toSave[field] = state[field];
      }
    });

    localStorage.setItem(STORAGE_KEY, JSON.stringify(toSave));
  } catch (e) {
    console.warn("[AssetsProvider] Failed to persist state", e);
  }
}

export const AssetsProvider = ({ children }: AssetsProviderProps) => {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const params = useParams<{ assetId?: string }>();
  const { state: settingsState } = useSettingsContext();

  // Initialize state with persisted data
  const initializeState = useCallback((): AssetsState => {
    const persistedState = loadPersistedState();

    // Determine current tab from URL
    const currentTab: TabType = location.pathname.includes("/videos")
      ? "videos"
      : location.pathname.includes("/audios")
        ? "audios"
        : "photos";

    // Get UI preferences from settings
    const preferredGroupBy =
      settingsState.ui.asset_page?.layout === "wide" ? "type" : "date";

    // Extract URL parameters
    const urlGroupBy = (searchParams.get("groupBy") as any) || preferredGroupBy;
    const urlQuery = searchParams.get("q") || "";
    const isCarouselOpen = !!params.assetId;

    return {
      ...initialAssetsState,
      ...persistedState,
      ui: {
        currentTab,
        groupBy: urlGroupBy,
        searchQuery: urlQuery,
        searchMode: initialAssetsState.ui.searchMode,
        isCarouselOpen,
        activeAssetId: params.assetId,
      },
    };
  }, [
    location.pathname,
    searchParams,
    params.assetId,
    settingsState.ui.asset_page?.layout,
  ]);

  const [state, dispatch] = useReducer(
    assetsReducer,
    undefined,
    initializeState,
  );

  // Persist specific state slices to localStorage
  useEffect(() => {
    saveStateToStorage(state);
  }, [state.filters, state.selection]);

  // Sync UI state with URL parameters
  useEffect(() => {
    const currentTab: TabType = location.pathname.includes("/videos")
      ? "videos"
      : location.pathname.includes("/audios")
        ? "audios"
        : "photos";

    if (currentTab !== state.ui.currentTab) {
      dispatch({ type: "SET_CURRENT_TAB", payload: currentTab });
    }
  }, [location.pathname, state.ui.currentTab]);

  // Sync carousel state with route params
  useEffect(() => {
    const isCarouselOpen = !!params.assetId;
    const activeAssetId = params.assetId;

    dispatch({ type: "SET_CAROUSEL_OPEN", payload: isCarouselOpen });
    dispatch({ type: "SET_ACTIVE_ASSET_ID", payload: activeAssetId });
  }, [params.assetId]);

  // Sync URL query parameters with UI state
  useEffect(() => {
    const urlGroupBy = searchParams.get("groupBy");
    const urlQuery = searchParams.get("q");

    // Get default groupBy based on settings
    const defaultGroupBy =
      settingsState.ui.asset_page?.layout === "wide" ? "type" : "date";

    const expectedGroupBy = urlGroupBy || defaultGroupBy;

    if (expectedGroupBy !== state.ui.groupBy) {
      dispatch({
        type: "HYDRATE_UI_FROM_URL",
        payload: { groupBy: expectedGroupBy as any },
      });
    }

    if (urlQuery !== null && urlQuery !== state.ui.searchQuery) {
      dispatch({
        type: "HYDRATE_UI_FROM_URL",
        payload: { searchQuery: urlQuery },
      });
    }
  }, [searchParams, settingsState.ui.asset_page?.layout]);

  // Write UI state changes back to URL
  useEffect(() => {
    const params = new URLSearchParams(searchParams);
    let hasChanges = false;

    // Default groupBy based on settings
    const defaultGroupBy =
      settingsState.ui.asset_page?.layout === "wide" ? "type" : "date";

    // Only set groupBy in URL if it's different from default
    if (state.ui.groupBy !== defaultGroupBy) {
      params.set("groupBy", state.ui.groupBy);
      hasChanges = true;
    } else {
      if (params.has("groupBy")) {
        params.delete("groupBy");
        hasChanges = true;
      }
    }

    // Only set search query if it's not empty
    if (state.ui.searchQuery.trim()) {
      params.set("q", state.ui.searchQuery);
      hasChanges = true;
    } else {
      if (params.has("q")) {
        params.delete("q");
        hasChanges = true;
      }
    }

    if (hasChanges) {
      setSearchParams(params, { replace: true });
    }
  }, [
    state.ui.groupBy,
    state.ui.searchQuery,
    settingsState.ui.asset_page?.layout,
    setSearchParams,
  ]);

  // Cleanup stale views periodically
  useEffect(() => {
    const interval = setInterval(
      () => {
        // Only cleanup if there are views to clean
        if (Object.keys(state.views.views).length > 10) {
          const now = Date.now();
          const maxAge = 30 * 60 * 1000; // 30 minutes
          const activeViewKeys = new Set(state.views.activeViewKeys);

          Object.entries(state.views.views).forEach(([key, view]) => {
            if (!activeViewKeys.has(key) && now - view.lastFetchAt > maxAge) {
              dispatch({ type: "REMOVE_VIEW", payload: { viewKey: key } });
            }
          });
        }
      },
      5 * 60 * 1000,
    ); // Check every 5 minutes

    return () => clearInterval(interval);
  }, [state.views.views, state.views.activeViewKeys]);

  // Navigation helpers
  const openCarousel = useCallback(
    (assetId: string) => {
      const currentParams = new URLSearchParams(searchParams);
      let basePath = "/assets/photos";

      if (state.ui.currentTab === "videos") {
        basePath = "/assets/videos";
      } else if (state.ui.currentTab === "audios") {
        basePath = "/assets/audios";
      }

      const targetUrl = `${basePath}/${assetId}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`;
      navigate(targetUrl);
    },
    [navigate, searchParams, state.ui.currentTab],
  );

  const closeCarousel = useCallback(() => {
    const currentParams = new URLSearchParams(searchParams);
    let basePath = "/assets/photos";

    if (state.ui.currentTab === "videos") {
      basePath = "/assets/videos";
    } else if (state.ui.currentTab === "audios") {
      basePath = "/assets/audios";
    }

    const targetUrl = `${basePath}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`;
    navigate(targetUrl);
  }, [navigate, searchParams, state.ui.currentTab]);

  const switchTab = useCallback(
    (tab: TabType) => {
      const currentParams = new URLSearchParams(searchParams);
      let basePath = "/assets/photos";

      if (tab === "videos") {
        basePath = "/assets/videos";
      } else if (tab === "audios") {
        basePath = "/assets/audios";
      }

      const targetUrl = `${basePath}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`;
      navigate(targetUrl);
    },
    [navigate, searchParams],
  );

  const contextValue = useMemo<AssetsContextValue>(
    () => ({
      state,
      dispatch,
      // Add navigation helpers to context for convenience
      openCarousel,
      closeCarousel,
      switchTab,
    }),
    [state, dispatch, openCarousel, closeCarousel, switchTab],
  );

  return (
    <AssetsContext.Provider value={contextValue}>
      {children}
    </AssetsContext.Provider>
  );
};
