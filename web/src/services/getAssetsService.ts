// src/services/getAssetService.ts

import api from "@/https-common/";
import { AxiosRequestConfig, AxiosResponse } from "axios";
import { ApiResult } from "./uploadService";

/**
 * @interface AssetListResponse
 * @description The structure of the data object within the API response for listing assets.
 */
interface AssetListResponse {
  assets: Asset[];
  limit: number;
  offset: number;
}

/**
 * @interface MessageResponse
 * @description A generic success message response from the API.
 */
interface MessageResponse {
  message: string;
}

/**
 * @interface ListAssetsParams
 * @description Defines the shape of the query parameters object for the listAssets function.
 * All properties are optional, allowing for flexible filtering.
 */
export interface ListAssetsParams {
  type?: "PHOTO" | "VIDEO" | "AUDIO" | "DOCUMENT";
  owner_id?: number;
  q?: string;
  limit?: number;
  offset?: number;
}

/**
 * @service AssetService
 * @description A collection of functions for interacting with the asset-related API endpoints.
 */
export const getAssetService = {
  /**
   * Fetches a paginated and filterable list of assets from the server.
   * @param {ListAssetsParams} params - An object containing query parameters for filtering and pagination.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<AssetListResponse>>>} A promise that resolves to the full Axios response.
   */
  listAssets: async (
    params: ListAssetsParams,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AssetListResponse>>> => {
    // We pass the params object directly to axios.
    // Axios will automatically construct the query string,
    // ignoring any properties that are undefined.
    return api.get<ApiResult<AssetListResponse>>("/api/v1/assets", {
      ...config,
      params: params,
    });
  },

  /**
   * Fetches the detailed information for a single asset by its ID.
   * @param {string} id - The UUID of the asset.
   * @returns {Promise<AxiosResponse<ApiResult<Asset>>>} A promise resolving to the asset's details.
   */
  getAssetById: async (
    id: string,
  ): Promise<AxiosResponse<ApiResult<Asset>>> => {
    return api.get<ApiResult<Asset>>(`/api/v1/assets/${id}`);
  },

  /**
   * Deletes an asset by its ID.
   * @param {string} id - The UUID of the asset to delete.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  deleteAsset: async (
    id: string,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.delete<ApiResult<MessageResponse>>(`/api/v1/assets/${id}`);
  },

  /**
   * Updates the metadata for a specific asset.
   * @param {string} id - The UUID of the asset to update.
   * @param {JSON} metadata - The new metadata to apply.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  updateAssetMetadata: async (
    id: string,
    metadata: JSON,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    // The payload for the PUT request needs to be in the shape { metadata: { ... } }
    const payload = { metadata };
    return api.put<ApiResult<MessageResponse>>(`/api/v1/assets/${id}`, payload);
  },

  /**
   * Adds an asset to a specific album.
   * @param {string} assetId - The UUID of the asset.
   * @param {number} albumId - The ID of the album.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  addAssetToAlbum: async (
    assetId: string,
    albumId: number,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.post<ApiResult<MessageResponse>>(
      `/api/v1/assets/${assetId}/albums/${albumId}`,
    );
  },

  /**
   * Fetches the list of all supported asset types.
   * @returns {Promise<AxiosResponse<ApiResult<AssetTypesResponse>>>} A promise resolving to the list of types.
   */
  getAssetTypes: async (): Promise<AxiosResponse<ApiResult<string[]>>> => {
    return api.get<ApiResult<string[]>>(`/api/v1/assets/types`);
  },

  /**
   * Gets the original file URL for an asset by its ID.
   * This is useful for creating direct links or when you need the URL string.
   * Note: For authenticated requests, prefer using getOriginalFile() instead.
   * @param {string} id - The UUID of the asset.
   * @returns {string} The URL to fetch the original file.
   */
  getOriginalFileUrl: (id: string): string => {
    const baseURL = api.defaults.baseURL || "http://localhost:8080";
    return `${baseURL}/api/v1/assets/${id}/original`;
  },

  /**
   * Fetches the original file content for an asset by its ID.
   * This method uses the configured axios instance with proper authentication handling.
   * Recommended for programmatic file access (e.g., EXIF extraction, processing).
   * @param {string} id - The UUID of the asset.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<Blob>>} A promise resolving to the file blob.
   */
  getOriginalFile: async (
    id: string,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<Blob>> => {
    return api.get<Blob>(`/api/v1/assets/${id}/original`, {
      ...config,
      responseType: "blob",
    });
  },

  /**
   * Fetches the thumbnail URL for an asset by its ID.
   * This method returns a URL that can be used to fetch the thumbnail image.
   * @param {string} id - The UUID of the asset.
   * @param {string} [size="small"] - The size of the thumbnail. Can be "small", "medium", or "large".
   * @returns {string} The URL to fetch the thumbnail image.
   */
  getThumbnailUrl: (id: string, size: string = "small"): string => {
    const baseURL = api.defaults.baseURL || "http://localhost:8080";
    return `${baseURL}/api/v1/assets/${id}/thumbnail?size=${size}`;
  },
};
