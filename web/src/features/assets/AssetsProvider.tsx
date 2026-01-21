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
  persist?: boolean;
  basePath?: string; // Optional base path for carousel navigation
}

function loadPersistedState(): Partial<AssetsState> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      const restored: Partial<AssetsState> = {};

      STORAGE_FIELDS.forEach((field) => {
        if (parsed[field]) {
          if (field === "selection") {
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

export const AssetsProvider = ({ children, persist = true, basePath }: AssetsProviderProps) => {
  const location = useLocation();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const params = useParams<{ assetId?: string }>();
  const { state: settingsState } = useSettingsContext();

  const initializeState = useCallback((): AssetsState => {
    const persistedState = persist ? loadPersistedState() : {};

    const currentTab: TabType = location.pathname.includes("/videos")
      ? "videos"
      : location.pathname.includes("/audios")
        ? "audios"
        : "photos";

    const preferredGroupBy =
      settingsState.ui.asset_page?.layout === "wide" ? "type" : "date";

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
    persist
  ]);

  const [state, dispatch] = useReducer(
    assetsReducer,
    undefined,
    initializeState,
  );

  useEffect(() => {
    if (persist) {
      saveStateToStorage(state);
    }
  }, [state.filters, state.selection, persist]);

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

  useEffect(() => {
    const isCarouselOpen = !!params.assetId;
    const activeAssetId = params.assetId;

    dispatch({ type: "SET_CAROUSEL_OPEN", payload: isCarouselOpen });
    dispatch({ type: "SET_ACTIVE_ASSET_ID", payload: activeAssetId });
  }, [params.assetId]);

  useEffect(() => {
    const urlGroupBy = searchParams.get("groupBy");
    const urlQuery = searchParams.get("q");
    const defaultGroupBy = settingsState.ui.asset_page?.layout === "wide" ? "type" : "date";
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

  useEffect(() => {
    const params = new URLSearchParams(searchParams);
    let hasChanges = false;
    const defaultGroupBy = settingsState.ui.asset_page?.layout === "wide" ? "type" : "date";

    if (state.ui.groupBy !== defaultGroupBy) {
      params.set("groupBy", state.ui.groupBy);
      hasChanges = true;
    } else {
      if (params.has("groupBy")) {
        params.delete("groupBy");
        hasChanges = true;
      }
    }

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

  useEffect(() => {
    const interval = setInterval(
      () => {
        if (Object.keys(state.views.views).length > 10) {
          const now = Date.now();
          const maxAge = 30 * 60 * 1000;
          const activeViewKeys = new Set(state.views.activeViewKeys);

          Object.entries(state.views.views).forEach(([key, view]) => {
            if (!activeViewKeys.has(key) && now - view.lastFetchAt > maxAge) {
              dispatch({ type: "REMOVE_VIEW", payload: { viewKey: key } });
            }
          });
        }
      },
      5 * 60 * 1000,
    );

    return () => clearInterval(interval);
  }, [state.views.views, state.views.activeViewKeys]);

  const openCarousel = useCallback(
    (assetId: string) => {
      const currentParams = new URLSearchParams(searchParams);
      let path = basePath || "/assets/photos";
      
      if (!basePath) {
        if (state.ui.currentTab === "videos") path = "/assets/videos";
        else if (state.ui.currentTab === "audios") path = "/assets/audios";
      }

      const targetUrl = `${path}/${assetId}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`;
      navigate(targetUrl);
    },
    [navigate, searchParams, state.ui.currentTab, basePath],
  );

  const closeCarousel = useCallback(() => {
    const currentParams = new URLSearchParams(searchParams);
    let path = basePath || "/assets/photos";
    
    if (!basePath) {
      if (state.ui.currentTab === "videos") path = "/assets/videos";
      else if (state.ui.currentTab === "audios") path = "/assets/audios";
    }

    const targetUrl = `${path}${currentParams.toString() ? `?${currentParams.toString()}` : ""}`;
    navigate(targetUrl);
  }, [navigate, searchParams, state.ui.currentTab, basePath]);

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

  const contextValue = useMemo<AssetsContextValue>(
    () => ({
      state,
      dispatch,
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
