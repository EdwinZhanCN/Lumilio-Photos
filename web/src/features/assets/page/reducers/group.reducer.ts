/**
 * Grouping slice reducer and default value
 *
 * This file now only handles the grouping strategy for the Assets page UI state.
 */
import { AssetsPageAction, GroupByType } from "../types";

/**
 * Default grouping strategy.
 */
export const DEFAULT_GROUP_BY: GroupByType = "date";

/**
 * Grouping strategy slice reducer.
 */
export function groupReducer(
  state: GroupByType = DEFAULT_GROUP_BY,
  action: AssetsPageAction,
): GroupByType {
  switch (action.type) {
    case "SET_GROUP_BY":
      return action.payload;
    case "HYDRATE_FROM_URL":
      return action.payload.groupBy ?? state;
    default:
      return state;
  }
}
