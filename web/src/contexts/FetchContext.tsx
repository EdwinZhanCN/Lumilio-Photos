/**
 * @fileoverview Assets Fetch Context Provider for managing asset browsing and filtering
 *
 * This module provides a React context for handling asset fetching operations including
 * infinite scrolling, filtering, searching, and pagination. It follows a performance-optimized
 * pattern by separating state from actions to minimize unnecessary re-renders.
 *
 * The core business logic is encapsulated within the `useFetchProcess` custom hook,
 * making this context a thin wrapper that provides the data to components.
 *
 * @author Edwin Zhan
 * @since 1.0.0
 *
 * @example
 * ```tsx
 * // Wrap your app with the AssetsProvider
 * function App() {
 *   return (
 *     <AssetsProvider>
 *       <AssetBrowser />
 *       <FilterPanel />
 *     </AssetsProvider>
 *   );
 * }
 *
 * // Use the context in your components
 * function AssetGrid() {
 *   const { assets, isLoading, fetchNextPage, hasMore } = useAssetsContext();
 *
 *   return (
 *     <div>
 *       {assets.map(asset => <AssetCard key={asset.assetId} asset={asset} />)}
 *       {hasMore && <button onClick={fetchNextPage}>Load More</button>}
 *     </div>
 *   );
 * }
 * ```
 *
 * @todo Implement caching mechanism for asset data
 * @todo Add support for real-time asset updates via WebSocket
 * @todo Consider implementing virtual scrolling for large asset lists
 */
import { createContext, useContext, useMemo, ReactNode } from "react";
import { ListAssetsParams } from "@/services/getAssetsService";
import { useFetchProcess } from "@/hooks/api-hooks/useFetchProcess";

/**
 * **Assets State Interface**
 *
 * Defines the complete state structure for asset browsing operations.
 * This state is read-only and optimized for performance.
 *
 * @interface AssetsState
 * @since 1.0.0
 *
 * @example
 * ```tsx
 * function AssetCounter() {
 *   const { assets, isLoading, hasMore } = useAssetsContext();
 *
 *   return (
 *     <div>
 *       <p>Loaded {assets.length} assets</p>
 *       {isLoading && <p>Loading...</p>}
 *       {!hasMore && <p>All assets loaded</p>}
 *     </div>
 *   );
 * }
 * ```
 */
export interface AssetsState {
  /**
   * Array of currently loaded assets.
   * This list grows as more pages are fetched via infinite scrolling.
   */
  assets: Asset[];

  /**
   * Current filter and search parameters applied to the asset list.
   *
   * @see {@link ListAssetsParams} for available filter options
   */
  filters: ListAssetsParams;
  isLoading: boolean;
  isLoadingNextPage: boolean;
  error: string | null;
  hasMore: boolean;
}

/**
 * **Assets Actions Interface**
 *
 * Defines all available actions for manipulating asset state.
 * These functions are stable and won't cause re-renders for components that only use them.
 *
 * @interface AssetsActions
 * @since 1.0.0
 *
 */
export interface AssetsActions {
  /**
   * **Fetch Assets Function**
   *
   * Fetches the first page of assets based on new parameters, replacing the current list.
   * This is typically used when filters change or initial load occurs.
   *
   * @param params - Filter and pagination parameters
   *
   * @example
   * ```tsx
   * const { fetchAssets } = useAssetsContext();
   *
   * // Fetch photos uploaded in the last week
   * await fetchAssets({
   *   type: 'PHOTO',
   *   dateRange: { start: '2024-01-01', end: '2024-01-07' },
   *   limit: 20
   * });
   * ```
   */
  fetchAssets: (params: ListAssetsParams) => Promise<void>;

  /**
   * **Fetch Next Page Function**
   *
   * Fetches the next page of assets and appends them to the current list.
   * Used for infinite scrolling implementations.
   *
   * @example
   * ```tsx
   * function InfiniteScroll() {
   *   const { fetchNextPage, hasMore, isLoadingNextPage } = useAssetsContext();
   *
   *   useEffect(() => {
   *     const handleScroll = () => {
   *       if (window.innerHeight + window.scrollY >= document.body.offsetHeight - 1000) {
   *         if (hasMore && !isLoadingNextPage) {
   *           fetchNextPage();
   *         }
   *       }
   *     };
   *
   *     window.addEventListener('scroll', handleScroll);
   *     return () => window.removeEventListener('scroll', handleScroll);
   *   }, [fetchNextPage, hasMore, isLoadingNextPage]);
   * }
   * ```
   */
  fetchNextPage: () => Promise<void>;

  /**
   * **Apply Filter Function**
   *
   * Higher-level function to apply a new filter and refetch the asset list from the start.
   * Automatically resets pagination when filters change.
   *
   * @param key - The filter parameter to update
   * @param value - The new value for the filter parameter
   *
   * @example
   * ```tsx
   * // Filter by asset type
   * applyFilter('type', 'PHOTO');
   *
   * // Filter by date range
   * applyFilter('dateRange', { start: '2024-01-01', end: '2024-01-31' });
   * ```
   */
  applyFilter: (key: keyof ListAssetsParams, value: any) => void;

  /**
   * **Set Search Query Function**
   *
   * Higher-level function to apply a new search query and refetch the asset list.
   * Typically searches across filenames, descriptions, and tags.
   *
   * @param query - The search query string
   *
   * @example
   * ```tsx
   * function SearchBar() {
   *   const { setSearchQuery } = useAssetsContext();
   *   const [query, setQuery] = useState('');
   *
   *   const handleSearch = useMemo(
   *     () => debounce((searchQuery: string) => {
   *       setSearchQuery(searchQuery);
   *     }, 300),
   *     [setSearchQuery]
   *   );
   *
   *   useEffect(() => {
   *     handleSearch(query);
   *   }, [query, handleSearch]);
   *
   *   return (
   *     <input
   *       value={query}
   *       onChange={(e) => setQuery(e.target.value)}
   *       placeholder="Search assets..."
   *     />
   *   );
   * }
   * ```
   */
  setSearchQuery: (query: string) => void;

  /**
   * **Reset Filters Function**
   *
   * Resets all filters to their default state and refetches the complete asset list.
   * Useful for "Clear All" functionality.
   *
   * @example
   * ```tsx
   * <button onClick={resetFilters}>
   *   Clear All Filters
   * </button>
   * ```
   */
  resetFilters: () => void;
}

/**
 * **Assets Context Value Type**
 *
 * Combined type representing the complete API provided by the assets context.
 * Merges state and actions into a single interface for easier consumption.
 *
 * @since 1.0.0
 * @see {@link AssetsState} for state properties
 * @see {@link AssetsActions} for available actions
 */
type AssetsContextValue = AssetsState & AssetsActions;

/**
 * **Assets State Context**
 *
 * React context for sharing asset state across components.
 * Separated from actions context for performance optimization.
 *
 * @internal
 */
const AssetsStateContext = createContext<AssetsState | undefined>(undefined);

/**
 * **Assets Actions Context**
 *
 * React context for sharing asset actions across components.
 * Separated from state context to prevent unnecessary re-renders.
 *
 * @internal
 */
const AssetsActionsContext = createContext<AssetsActions | undefined>(
  undefined,
);

/**
 * **Assets Provider Props**
 *
 * Props interface for the AssetsProvider component.
 *
 * @interface AssetsProviderProps
 */
interface AssetsProviderProps {
  /** Child components that will have access to the assets context */
  children: ReactNode;
}

/**
 * **Assets Provider Component**
 *
 * Main provider component that manages asset fetching state and provides context to child components.
 * Uses a performance-optimized pattern with separate state and actions contexts.
 *
 * @param props - Provider props containing children
 * @returns JSX element wrapping children with assets context
 *
 * @since 1.0.0
 */
export default function AssetsProvider({ children }: AssetsProviderProps) {
  const { state, actions } = useFetchProcess();
  const actionsValue = useMemo(() => actions, [actions]);
  const stateValue = useMemo(() => state, [state]);

  return (
    <AssetsActionsContext.Provider value={actionsValue}>
      <AssetsStateContext.Provider value={stateValue}>
        {children}
      </AssetsStateContext.Provider>
    </AssetsActionsContext.Provider>
  );
}

/**
 * **Assets Context Hook**
 *
 * Primary hook for components to interact with the assets context.
 * Provides type-safe access to both state and actions with automatic error handling.
 *
 * @returns Combined assets state and actions
 * @throws Error if used outside of AssetsProvider
 *
 * @since 1.0.0
 * @see {@link AssetsProvider} for the context provider
 * @see {@link AssetsContextValue} for the complete API reference
 */
export function useAssetsContext(): AssetsContextValue {
  const state = useContext(AssetsStateContext);
  const actions = useContext(AssetsActionsContext);

  if (state === undefined || actions === undefined) {
    throw new Error("useAssetsContext must be used within an AssetsProvider");
  }

  return { ...state, ...actions };
}
