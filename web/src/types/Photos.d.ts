/**
 * @fileoverview This file contains the type definitions for the Photos page.
 *
 * @author Edwin Zhan
 * @since 1.0.0
 */

import { Asset } from "@/lib/http-commons/schema-extensions";

/**
 * Defines the shape of the state for the Photos page.
 *
 * @property assets - An array of all assets.
 * @property groupedAssets - A map of assets grouped by date.
 * @property selectedAsset - The currently selected asset.
 * @property isCarouselOpen - A boolean indicating whether the carousel is open.
 * @property isLoading - A boolean indicating whether the assets are being loaded.
 * @property error - An error message if an error occurred.
 */
export interface PhotosState {
  assets: Asset[];
  groupedAssets: Record<string, Asset[]>;
  selectedAsset: Asset | null;
  isCarouselOpen: boolean;
  isLoading: boolean;
  error: string | null;
}

/**
 * Defines the actions that can be dispatched to the Photos reducer.
 *
 * @property type - The type of the action.
 * @property payload - The payload of the action.
 */
export type PhotosAction =
  | { type: "FETCH_SUCCESS"; payload: Asset[] }
  | { type: "FETCH_FAILURE"; payload: string }
  | { type: "GROUP_ASSETS" }
  | { type: "SELECT_ASSET"; payload: Asset }
  | { type: "CLEAR_SELECTED_ASSET" }
  | { type: "OPEN_CAROUSEL" }
  | { type: "CLOSE_CAROUSEL" };
