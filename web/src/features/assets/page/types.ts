/**
 * Assets Page Types
 *
 * These types define the page-level UI state and actions for the Assets feature,
 * mirroring the Settings architecture:
 * - Centralized State interface
 * - Discriminated union Action type
 * - Context value exposing { state, dispatch }
 *
 * Domain data fetching, pagination, etc. remain in AssetsProvider (data layer).
 * This page state focuses purely on UI concerns (grouping, searching, and carousel).
 */

/**
 * Grouping strategy for assets on the page.
 */
export type GroupByType = "date" | "type" | "album";

/**
 * Assets Page UI State
 *
 * Note:
 * - `isCarouselOpen` is stored for ease of effect management and to align with
 *   the Settings-like state + reducer flow. The actual assetId lives in the route,
 *   and navigation is handled at the provider level.
 */
export interface AssetsPageState {
  groupBy: GroupByType;
  searchQuery: string;
  isCarouselOpen: boolean;
}

/**
 * Assets Page Actions
 *
 * HYDRATE_FROM_URL:
 * - Used to initialize/sync state from URL query params on mount or when URL changes.
 * - Only includes URL-driven fields (groupBy, searchQuery).
 *
 * SET_CAROUSEL_OPEN:
 * - Controls the UI open/close state of the carousel.
 * - Actual route navigation lives in the provider (side-effect), not the reducer.
 */
export type AssetsPageAction =
  | { type: "SET_GROUP_BY"; payload: GroupByType }
  | { type: "SET_SEARCH_QUERY"; payload: string }
  | { type: "SET_CAROUSEL_OPEN"; payload: boolean }
  | {
      type: "HYDRATE_FROM_URL";
      payload: Partial<Pick<AssetsPageState, "groupBy" | "searchQuery">>;
    };

/**
 * Context value for the Assets Page state.
 *
 * Follows the Settings pattern: consumers dispatch actions to mutate state.
 * Side-effects (URL sync, navigation, debounced search bridging to data layer) are
 * implemented inside the Provider, not in consumers.
 */
export interface AssetsPageContextValue {
  state: AssetsPageState;
  dispatch: React.Dispatch<AssetsPageAction>;
}
