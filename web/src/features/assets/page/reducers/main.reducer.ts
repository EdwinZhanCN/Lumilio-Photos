/**
 * Assets Page Main Reducers: search, carousel, and root combiner
 *
 * This module provides the main reducers for the Assets page state:
 * - searchReducer: search query (string)
 * - carouselReducer: carousel open/close flag (boolean)
 * - assetsPageReducer: root reducer that composes all slices (group, search, carousel)
 * - initialAssetsPageState: default initial state for the page
 *
 * It mirrors the Settings architecture where multiple slice reducers are composed
 * into a single root reducer.
 */

import { AssetsPageAction, AssetsPageState } from "../types";
import { groupReducer, DEFAULT_GROUP_BY } from "./group.reducer";

/**
 * Defaults for non-group slices
 */
export const DEFAULT_SEARCH_QUERY = "";
export const DEFAULT_CAROUSEL_OPEN = false;

/**
 * Search query slice reducer
 */
export function searchReducer(
  state: string = DEFAULT_SEARCH_QUERY,
  action: AssetsPageAction,
): string {
  switch (action.type) {
    case "SET_SEARCH_QUERY":
      return action.payload;
    case "HYDRATE_FROM_URL":
      return action.payload.searchQuery ?? state;
    default:
      return state;
  }
}

/**
 * Carousel open state slice reducer
 *
 * Note: Route navigation side-effects remain in the Provider layer.
 */
export function carouselReducer(
  state: boolean = DEFAULT_CAROUSEL_OPEN,
  action: AssetsPageAction,
): boolean {
  switch (action.type) {
    case "SET_CAROUSEL_OPEN":
      return action.payload;
    default:
      return state;
  }
}

/**
 * Optional initial state if a consumer/provider wants a ready-to-use baseline.
 * Typically, the Provider initializes from URL and may override these.
 */
export const initialAssetsPageState: AssetsPageState = {
  groupBy: DEFAULT_GROUP_BY,
  searchQuery: DEFAULT_SEARCH_QUERY,
  isCarouselOpen: DEFAULT_CAROUSEL_OPEN,
};

/**
 * Root combiner that mirrors the Settings architecture:
 * - Passes the same action to each slice reducer
 * - Reassembles the AssetsPageState from their results
 */
export function assetsPageReducer(
  state: AssetsPageState = initialAssetsPageState,
  action: AssetsPageAction,
): AssetsPageState {
  return {
    groupBy: groupReducer(state.groupBy, action),
    searchQuery: searchReducer(state.searchQuery, action),
    isCarouselOpen: carouselReducer(state.isCarouselOpen, action),
  };
}
