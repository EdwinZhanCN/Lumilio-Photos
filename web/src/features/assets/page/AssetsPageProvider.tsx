import React, {
  createContext,
  useEffect,
  useMemo,
  useReducer,
  useRef,
  ReactNode,
} from "react";
import {
  useLocation,
  useNavigate,
  useParams,
  useSearchParams,
} from "react-router-dom";
import { useAssetsContext } from "../hooks/useAssetsContext";
import { AssetsPageContextValue, AssetsPageState, GroupByType } from "./types";
import { DEFAULT_GROUP_BY } from "./reducers/group.reducer";
import {
  assetsPageReducer,
  initialAssetsPageState,
  DEFAULT_SEARCH_QUERY,
} from "./reducers/main.reducer";
import { useSettingsContext } from "@/features/settings";

/**
 * React Context for Assets Page UI State
 * Follows the Settings architecture: expose { state, dispatch } only.
 */
export const AssetsPageContext = createContext<
  AssetsPageContextValue | undefined
>(undefined);

interface AssetsPageProviderProps {
  children: ReactNode;
}

/**
 * AssetsPageProvider
 *
 * Mirrors the SettingsProvider pattern:
 * - Centralized reducer-based page UI state
 * - Side-effects (URL sync, debounced search bridging to data layer)
 * - State hydration from URL; continuous two-way sync
 *
 * Domain data fetching/pagination stay in AssetsProvider. This provider only
 * handles page-level UI concerns (groupBy, searchQuery, carousel flag).
 */
export function AssetsPageProvider({ children }: AssetsPageProviderProps) {
  const params = useParams<{ assetId?: string }>();
  const [searchParams, setSearchParams] = useSearchParams();

  // Bridge to domain layer (AssetsProvider) for search
  const { setSearchQuery: setContextSearchQuery } = useAssetsContext();

  // Read UI preferences from Settings (e.g., asset page layout) to decide defaults
  const { state: settingsState } = useSettingsContext();
  const preferredGroupBy: GroupByType = (() => {
    const layout = settingsState.ui.asset_page?.layout;
    switch (layout) {
      case "wide":
        return "type";
      default:
        return DEFAULT_GROUP_BY;
    }
  })();

  // Initialize state from URL and route params
  const computeInitial = (): AssetsPageState => {
    const urlGroupBy =
      (searchParams.get("groupBy") as GroupByType) ?? preferredGroupBy;
    const urlQuery = searchParams.get("q") ?? DEFAULT_SEARCH_QUERY;

    return {
      ...initialAssetsPageState,
      groupBy: urlGroupBy,
      searchQuery: urlQuery,
      isCarouselOpen: !!params.assetId,
    };
  };

  const [state, dispatch] = useReducer(
    assetsPageReducer,
    undefined,
    computeInitial,
  );

  // Keep isCarouselOpen in sync with the presence of :assetId param
  useEffect(() => {
    const open = !!params.assetId;
    console.log("AssetsPageProvider: assetId param changed", {
      assetId: params.assetId,
      open,
      currentIsCarouselOpen: state.isCarouselOpen,
    });
    if (open !== state.isCarouselOpen) {
      console.log("AssetsPageProvider: dispatching SET_CAROUSEL_OPEN", open);
      dispatch({ type: "SET_CAROUSEL_OPEN", payload: open });
    }
  }, [params.assetId, state.isCarouselOpen]);

  // Two-way sync: When URL query changes externally, hydrate state
  useEffect(() => {
    const urlGroupBy =
      (searchParams.get("groupBy") as GroupByType) ?? preferredGroupBy;
    const urlQuery = searchParams.get("q") ?? DEFAULT_SEARCH_QUERY;

    if (urlGroupBy !== state.groupBy || urlQuery !== state.searchQuery) {
      const hydratePayload: Partial<
        Pick<AssetsPageState, "groupBy" | "searchQuery">
      > = {
        groupBy: urlGroupBy,
        searchQuery: urlQuery,
      };
      dispatch({ type: "HYDRATE_FROM_URL", payload: hydratePayload });
    }
  }, [searchParams, preferredGroupBy]);

  // Write state changes back to URL, keeping only non-defaults; replace to avoid history spam
  useEffect(() => {
    const params = new URLSearchParams();
    if (state.groupBy !== preferredGroupBy)
      params.set("groupBy", state.groupBy);
    if (state.searchQuery) params.set("q", state.searchQuery);

    setSearchParams(params, { replace: true });
  }, [state.groupBy, state.searchQuery, preferredGroupBy, setSearchParams]);

  // Debounce search bridging: when page searchQuery changes, call data-layer setSearchQuery
  const isFirstSearchEffect = useRef(true);
  useEffect(() => {
    if (isFirstSearchEffect.current) {
      isFirstSearchEffect.current = false;
      // Optionally sync the initial query to data layer:
      // setContextSearchQuery(state.searchQuery);
      return;
    }
    const t = setTimeout(() => {
      setContextSearchQuery(state.searchQuery);
    }, 300);
    return () => clearTimeout(t);
  }, [state.searchQuery, setContextSearchQuery]);

  // Optional helpers (not exposed in context to keep parity with Settings):
  // Navigate to open/close carousel while preserving current query string.
  // Consumers can import and use these via a dedicated hook if needed later.

  // Memoize context value
  const value = useMemo<AssetsPageContextValue>(
    () => ({
      state,
      dispatch,
    }),
    [state, dispatch],
  );

  return (
    <AssetsPageContext.Provider value={value}>
      {/* Expose helpers via render-prop pattern if ever needed */}
      {children}
    </AssetsPageContext.Provider>
  );
}

/**
 * Hook to consume AssetsPageContext safely
 */
export function useAssetsPageContext(): AssetsPageContextValue {
  const ctx = React.useContext(AssetsPageContext);
  if (!ctx) {
    throw new Error(
      "useAssetsPageContext must be used within an AssetsPageProvider",
    );
  }
  return ctx;
}

// Optional: export helpers via separate hook to keep context value aligned with Settings
export function useAssetsPageNavigation() {
  const navigate = useNavigate();
  const location = useLocation();
  const [searchParams] = useSearchParams();

  const open = React.useCallback(
    (assetId: string) => {
      console.log("useAssetsPageNavigation.open called with assetId:", assetId);
      const currentParams = new URLSearchParams(searchParams);
      const path = location.pathname;

      let basePath = "/assets/photos";
      if (path.includes("/videos")) basePath = "/assets/videos";
      else if (path.includes("/audios")) basePath = "/assets/audios";

      const targetUrl = `${basePath}/${assetId}?${currentParams.toString()}`;
      console.log("Navigating to:", targetUrl);
      navigate(targetUrl);
    },
    [location.pathname, navigate, searchParams],
  );

  const close = React.useCallback(() => {
    console.log("useAssetsPageNavigation.close called");
    const currentParams = new URLSearchParams(searchParams);
    const path = location.pathname;

    let basePath = "/assets/photos";
    if (path.includes("/videos")) basePath = "/assets/videos";
    else if (path.includes("/audios")) basePath = "/assets/audios";

    const targetUrl = `${basePath}?${currentParams.toString()}`;
    console.log("Closing carousel, navigating to:", targetUrl);
    navigate(targetUrl);
  }, [location.pathname, navigate, searchParams]);

  return { openCarousel: open, closeCarousel: close };
}
