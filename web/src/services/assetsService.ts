// src/services/assetsService.ts

import api from "@/lib/http-commons/api.ts";
import type { AxiosRequestConfig, AxiosResponse } from "axios";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";
import type { ApiResult } from "./uploadService";

// ============================================================================
// Type Aliases from Generated Schema
// ============================================================================

type Schemas = components["schemas"];
type Paths = paths;

/**
 * Asset data transfer object
 */
export type Asset = Schemas["dto.AssetDTO"];

/**
 * Asset list response with pagination
 */
export type AssetListResponse = Schemas["dto.AssetListResponseDTO"];

/**
 * Asset types response
 */
export type AssetTypesResponse = Schemas["dto.AssetTypesResponseDTO"];

/**
 * Message response for operations
 */
export type MessageResponse = Schemas["dto.MessageResponseDTO"];

/**
 * Asset filter criteria
 */
export type AssetFilter = Schemas["dto.AssetFilterDTO"];

/**
 * Filename filter options
 */
export type FilenameFilter = Schemas["dto.FilenameFilterDTO"];

/**
 * Date range filter
 */
export type DateRange = Schemas["dto.DateRangeDTO"];

/**
 * Filter assets request
 */
export type FilterAssetsRequest = Schemas["dto.FilterAssetsRequestDTO"];

/**
 * Search assets request
 */
export type SearchAssetsRequest = Schemas["dto.SearchAssetsRequestDTO"];

/**
 * Filter options response (camera makes and lenses)
 */
export type FilterOptionsResponse = Schemas["dto.OptionsResponseDTO"];

/**
 * Update asset request
 */
export type UpdateAssetRequest = Schemas["dto.UpdateAssetRequestDTO"];

/**
 * Update rating request
 */
export type UpdateRatingRequest = Schemas["dto.UpdateRatingRequestDTO"];

/**
 * Update like request
 */
export type UpdateLikeRequest = Schemas["dto.UpdateLikeRequestDTO"];

/**
 * Update rating and like request
 */
export type UpdateRatingAndLikeRequest =
  Schemas["dto.UpdateRatingAndLikeRequestDTO"];

/**
 * Update description request
 */
export type UpdateDescriptionRequest =
  Schemas["dto.UpdateDescriptionRequestDTO"];

/**
 * List assets query parameters - extracted directly from paths
 */
export type ListAssetsParams = NonNullable<
  Paths["/assets"]["get"]["parameters"]["query"]
>;

/**
 * Get asset by ID query parameters
 */
export type GetAssetByIdParams = NonNullable<
  Paths["/assets/{id}"]["get"]["parameters"]["query"]
>;

// ============================================================================
// Re-export commonly used types for backward compatibility
// ============================================================================

export type SearchAssetsParams = SearchAssetsRequest;
export type FilterAssetsParams = FilterAssetsRequest;

// ============================================================================
// Asset Service
// ============================================================================

/**
 * @service AssetService
 * @description A collection of functions for interacting with the asset-related API endpoints.
 */
export const assetService = {
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
    return api.get<ApiResult<AssetListResponse>>("/api/v1/assets", {
      ...config,
      params: params,
    });
  },

  /**
   * Fetches the detailed information for a single asset by its ID.
   * @param {string} id - The UUID of the asset.
   * @param {GetAssetByIdParams} [params] - Optional query parameters (include_thumbnails, include_tags, etc.)
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<Asset>>>} A promise resolving to the asset's details.
   */
  getAssetById: async (
    id: string,
    params?: GetAssetByIdParams,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<Asset>>> => {
    return api.get<ApiResult<Asset>>(`/api/v1/assets/${id}`, {
      ...config,
      params,
    });
  },

  /**
   * Deletes an asset by its ID (soft delete).
   * @param {string} id - The UUID of the asset to delete.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  deleteAsset: async (
    id: string,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.delete<ApiResult<MessageResponse>>(
      `/api/v1/assets/${id}`,
      config,
    );
  },

  /**
   * Updates the metadata for a specific asset.
   * @param {string} id - The UUID of the asset to update.
   * @param {UpdateAssetRequest} request - The metadata update request.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  updateAssetMetadata: async (
    id: string,
    request: UpdateAssetRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.put<ApiResult<MessageResponse>>(
      `/api/v1/assets/${id}`,
      request,
      config,
    );
  },

  /**
   * Adds an asset to a specific album.
   * @param {string} assetId - The UUID of the asset.
   * @param {number} albumId - The ID of the album.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  addAssetToAlbum: async (
    assetId: string,
    albumId: number,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.post<ApiResult<MessageResponse>>(
      `/api/v1/assets/${assetId}/albums/${albumId}`,
      undefined,
      config,
    );
  },

  /**
   * Fetches the list of all supported asset types.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<AssetTypesResponse>>>} A promise resolving to the list of types.
   */
  getAssetTypes: async (
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AssetTypesResponse>>> => {
    return api.get<ApiResult<AssetTypesResponse>>(
      `/api/v1/assets/types`,
      config,
    );
  },

  /**
   * Filter assets using comprehensive filtering options.
   * @param {FilterAssetsRequest} request - Filter criteria and pagination
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration
   * @returns {Promise<AxiosResponse<ApiResult<AssetListResponse>>>} A promise that resolves to the filtered assets
   */
  filterAssets: async (
    request: FilterAssetsRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AssetListResponse>>> => {
    return api.post<ApiResult<AssetListResponse>>(
      "/api/v1/assets/filter",
      request,
      config,
    );
  },

  /**
   * Search assets using filename or semantic search.
   * @param {SearchAssetsRequest} request - Search parameters
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration
   * @returns {Promise<AxiosResponse<ApiResult<AssetListResponse>>>} A promise that resolves to the search results
   */
  searchAssets: async (
    request: SearchAssetsRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AssetListResponse>>> => {
    return api.post<ApiResult<AssetListResponse>>(
      "/api/v1/assets/search",
      request,
      config,
    );
  },

  /**
   * Get available filter options (camera makes and lenses).
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration
   * @returns {Promise<AxiosResponse<ApiResult<FilterOptionsResponse>>>} A promise that resolves to available filter options
   */
  getFilterOptions: async (
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<FilterOptionsResponse>>> => {
    return api.get<ApiResult<FilterOptionsResponse>>(
      "/api/v1/assets/filter-options",
      config,
    );
  },

  /**
   * Get assets by rating (0-5).
   * @param {number} rating - The rating to filter by (0-5)
   * @param {number} [limit=20] - Maximum number of results
   * @param {number} [offset=0] - Number of results to skip
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration
   * @returns {Promise<AxiosResponse<ApiResult<AssetListResponse>>>} A promise that resolves to the assets
   */
  getAssetsByRating: async (
    rating: number,
    limit: number = 20,
    offset: number = 0,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AssetListResponse>>> => {
    return api.get<ApiResult<AssetListResponse>>(
      `/api/v1/assets/rating/${rating}`,
      {
        ...config,
        params: { limit, offset },
      },
    );
  },

  /**
   * Get liked/favorited assets.
   * @param {number} [limit=20] - Maximum number of results
   * @param {number} [offset=0] - Number of results to skip
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration
   * @returns {Promise<AxiosResponse<ApiResult<AssetListResponse>>>} A promise that resolves to the liked assets
   */
  getLikedAssets: async (
    limit: number = 20,
    offset: number = 0,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<AssetListResponse>>> => {
    return api.get<ApiResult<AssetListResponse>>("/api/v1/assets/liked", {
      ...config,
      params: { limit, offset },
    });
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
   * @param {"small" | "medium" | "large"} [size="small"] - The size of the thumbnail.
   * @returns {string} The URL to fetch the thumbnail image.
   */
  getThumbnailUrl: (
    id: string,
    size: "small" | "medium" | "large" = "small",
  ): string => {
    const baseURL = api.defaults.baseURL || "http://localhost:8080";
    return `${baseURL}/api/v1/assets/${id}/thumbnail?size=${size}`;
  },

  /**
   * Get web-optimized video URL for an asset.
   * @param {string} id - The UUID of the asset.
   * @returns {string} The URL to fetch the web-optimized video.
   */
  getWebVideoUrl: (id: string): string => {
    const baseURL = api.defaults.baseURL || "http://localhost:8080";
    return `${baseURL}/api/v1/assets/${id}/video/web`;
  },

  /**
   * Get web-optimized audio URL for an asset.
   * @param {string} id - The UUID of the asset.
   * @returns {string} The URL to fetch the web-optimized audio.
   */
  getWebAudioUrl: (id: string): string => {
    const baseURL = api.defaults.baseURL || "http://localhost:8080";
    return `${baseURL}/api/v1/assets/${id}/audio/web`;
  },

  /**
   * Updates the rating of a specific asset.
   * @param {string} id - The UUID of the asset.
   * @param {number} rating - The rating (0-5).
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  updateAssetRating: async (
    id: string,
    rating: number,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    const payload: UpdateRatingRequest = { rating };
    return api.put<ApiResult<MessageResponse>>(
      `/api/v1/assets/${id}/rating`,
      payload,
      config,
    );
  },

  /**
   * Updates the like status of a specific asset.
   * @param {string} id - The UUID of the asset.
   * @param {boolean} liked - The like status.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  updateAssetLike: async (
    id: string,
    liked: boolean,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    const payload: UpdateLikeRequest = { liked };
    return api.put<ApiResult<MessageResponse>>(
      `/api/v1/assets/${id}/like`,
      payload,
      config,
    );
  },

  /**
   * Updates both the rating and like status of a specific asset.
   * @param {string} id - The UUID of the asset.
   * @param {number} rating - The rating (0-5).
   * @param {boolean} liked - The like status.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  updateAssetRatingAndLike: async (
    id: string,
    rating: number,
    liked: boolean,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    const payload: UpdateRatingAndLikeRequest = { rating, liked };
    return api.put<ApiResult<MessageResponse>>(
      `/api/v1/assets/${id}/rating-and-like`,
      payload,
      config,
    );
  },

  /**
   * Updates the description of a specific asset.
   * @param {string} id - The UUID of the asset.
   * @param {string} description - The new description text.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  updateAssetDescription: async (
    id: string,
    description: string,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    const payload: UpdateDescriptionRequest = { description };
    return api.put<ApiResult<MessageResponse>>(
      `/api/v1/assets/${id}/description`,
      payload,
      config,
    );
  },

  /**
   * Get albums containing a specific asset.
   * @param {string} id - The UUID of the asset.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<any>>>} A promise resolving to the list of albums.
   */
  getAssetAlbums: async (
    id: string,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<any>>> => {
    return api.get<ApiResult<any>>(`/api/v1/assets/${id}/albums`, config);
  },
};
