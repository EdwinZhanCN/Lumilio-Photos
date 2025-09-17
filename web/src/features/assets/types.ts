import {
  ListAssetsParams,
  SearchAssetsParams,
  AssetFilter,
} from "@/services/assetsService";

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

  /**
   * **Perform Advanced Search Function**
   *
   * Execute advanced search using either filename matching or semantic vector search.
   * Can be combined with comprehensive filters.
   *
   * @param params - Search parameters including query, search type, and optional filters
   *
   * @example
   * ```tsx
   * // Semantic search with filters
   * await performAdvancedSearch({
   *   query: "red bird on branch",
   *   search_type: "semantic",
   *   filter: { type: "PHOTO", rating: 5 },
   *   limit: 20
   * });
   * ```
   */
  performAdvancedSearch: (params: SearchAssetsParams) => void;

  /**
   * **Apply Advanced Filter Function**
   *
   * Apply comprehensive filtering options including RAW, rating, liked status,
   * filename patterns, date ranges, camera make, and lens.
   *
   * @param filter - Advanced filter criteria
   *
   * @example
   * ```tsx
   * // Filter by camera and rating
   * await applyAdvancedFilter({
   *   camera_make: "Canon",
   *   rating: 5,
   *   raw: true,
   *   date: { from: "2024-01-01", to: "2024-01-31" }
   * });
   * ```
   */
  applyAdvancedFilter: (filter: AssetFilter) => void;

  /**
   * **Clear Search Function**
   *
   * Clears the current search context (both simple filename search and advanced semantic search),
   * reverting the data layer back to the plain list (or filtered list if an advanced filter
   * is still active).
   *
   * Typical trigger: user collapses / deactivates the Search UI button.
   *
   * @example
   * ```tsx
   * const { clearSearch } = useAssetsContext();
   *
   * function SearchToggle({ active }: { active: boolean }) {
   *   useEffect(() => {
   *     if (!active) clearSearch();
   *   }, [active, clearSearch]);
   * }
   * ```
   */
  clearSearch: () => void;
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
export type AssetsContextValue = AssetsState & AssetsActions;
