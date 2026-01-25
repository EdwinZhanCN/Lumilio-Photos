// src/services/assetsService.ts

import client from "@/lib/http-commons/client";
import { $api } from "@/lib/http-commons/queryClient";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";

// ============================================================================
// Type Aliases from Generated Schema
// ============================================================================

type Schemas = components["schemas"];
type Paths = paths;

export type Asset = Schemas["dto.AssetDTO"];
export type AssetListResponse = Schemas["dto.AssetListResponseDTO"];
export type AssetTypesResponse = Schemas["dto.AssetTypesResponseDTO"];
export type MessageResponse = Schemas["dto.MessageResponseDTO"];
export type AssetFilter = Schemas["dto.AssetFilterDTO"];
export type FilenameFilter = Schemas["dto.FilenameFilterDTO"];
export type DateRange = Schemas["dto.DateRangeDTO"];
export type FilterAssetsRequest = Schemas["dto.FilterAssetsRequestDTO"];
export type SearchAssetsRequest = Schemas["dto.SearchAssetsRequestDTO"];
export type FilterOptionsResponse = Schemas["dto.OptionsResponseDTO"];
export type UpdateAssetRequest = Schemas["dto.UpdateAssetRequestDTO"];
export type UpdateRatingRequest = Schemas["dto.UpdateRatingRequestDTO"];
export type UpdateLikeRequest = Schemas["dto.UpdateLikeRequestDTO"];
export type UpdateRatingAndLikeRequest = Schemas["dto.UpdateRatingAndLikeRequestDTO"];
export type UpdateDescriptionRequest = Schemas["dto.UpdateDescriptionRequestDTO"];
export type ReprocessAssetRequest = Schemas["dto.ReprocessAssetRequestDTO"];
export type ReprocessAssetResponse = Schemas["dto.ReprocessAssetResponseDTO"];

export type ListAssetsParams = NonNullable<Paths["/assets"]["get"]["parameters"]["query"]>;
export type GetAssetByIdParams = NonNullable<Paths["/assets/{id}"]["get"]["parameters"]["query"]>;
export type SearchAssetsParams = SearchAssetsRequest;
export type FilterAssetsParams = FilterAssetsRequest;

// Base URL for URL generation helpers
const baseURL = import.meta.env.VITE_API_URL || "http://localhost:8080";

// ============================================================================
// Asset Service
// ============================================================================

export const assetService = {
  /**
   * Fetches a paginated and filterable list of assets from the server.
   */
  async listAssets(params: ListAssetsParams) {
    return client.GET("/assets", {
      params: { query: params },
    });
  },

  /**
   * Fetches the detailed information for a single asset by its ID.
   */
  async getAssetById(id: string, params?: GetAssetByIdParams) {
    return client.GET("/assets/{id}", {
      params: { path: { id }, query: params },
    });
  },

  /**
   * Deletes an asset by its ID (soft delete).
   */
  async deleteAsset(id: string) {
    return client.DELETE("/assets/{id}", {
      params: { path: { id } },
    });
  },

  /**
   * Updates the metadata for a specific asset.
   */
  async updateAssetMetadata(id: string, request: UpdateAssetRequest) {
    return client.PUT("/assets/{id}", {
      params: { path: { id } },
      body: request,
    });
  },

  /**
   * Adds an asset to a specific album.
   */
  async addAssetToAlbum(assetId: string, albumId: number) {
    return client.POST("/assets/{id}/albums/{albumId}", {
      params: { path: { id: assetId, albumId } },
    });
  },

  /**
   * Fetches the list of all supported asset types.
   */
  async getAssetTypes() {
    return client.GET("/assets/types", {});
  },

  /**
   * Filter assets using comprehensive filtering options.
   */
  async filterAssets(request: FilterAssetsRequest) {
    return client.POST("/assets/filter", {
      body: request,
    });
  },

  /**
   * Search assets using filename or semantic search.
   */
  async searchAssets(request: SearchAssetsRequest) {
    return client.POST("/assets/search", {
      body: request,
    });
  },

  /**
   * Get available filter options (camera makes and lenses).
   */
  async getFilterOptions() {
    return client.GET("/assets/filter-options", {});
  },

  /**
   * Get assets by rating (0-5).
   */
  async getAssetsByRating(rating: number, limit: number = 20, offset: number = 0) {
    return client.GET("/assets/rating/{rating}", {
      params: { path: { rating }, query: { limit, offset } },
    });
  },

  /**
   * Get liked/favorited assets.
   */
  async getLikedAssets(limit: number = 20, offset: number = 0) {
    return client.GET("/assets/liked", {
      params: { query: { limit, offset } },
    });
  },

  /**
   * Gets the original file URL for an asset by its ID.
   */
  getOriginalFileUrl: (id: string): string => {
    return `${baseURL}/assets/${id}/original`;
  },

  /**
   * Fetches the thumbnail URL for an asset by its ID.
   */
  getThumbnailUrl: (id: string, size: "small" | "medium" | "large" = "small"): string => {
    return `${baseURL}/assets/${id}/thumbnail?size=${size}`;
  },

  /**
   * Get web-optimized video URL for an asset.
   */
  getWebVideoUrl: (id: string): string => {
    return `${baseURL}/assets/${id}/video/web`;
  },

  /**
   * Get web-optimized audio URL for an asset.
   */
  getWebAudioUrl: (id: string): string => {
    return `${baseURL}/assets/${id}/audio/web`;
  },

  /**
   * Updates the rating of a specific asset.
   */
  async updateAssetRating(id: string, rating: number) {
    return client.PUT("/assets/{id}/rating", {
      params: { path: { id } },
      body: { rating },
    });
  },

  /**
   * Updates the like status of a specific asset.
   */
  async updateAssetLike(id: string, liked: boolean) {
    return client.PUT("/assets/{id}/like", {
      params: { path: { id } },
      body: { liked },
    });
  },

  /**
   * Updates both the rating and like status of a specific asset.
   */
  async updateAssetRatingAndLike(id: string, rating: number, liked: boolean) {
    return client.PUT("/assets/{id}/rating-and-like", {
      params: { path: { id } },
      body: { rating, liked },
    });
  },

  /**
   * Updates the description of a specific asset.
   */
  async updateAssetDescription(id: string, description: string) {
    return client.PUT("/assets/{id}/description", {
      params: { path: { id } },
      body: { description },
    });
  },

  /**
   * Get albums containing a specific asset.
   */
  async getAssetAlbums(id: string) {
    return client.GET("/assets/{id}/albums", {
      params: { path: { id } },
    });
  },

  /**
   * Reprocess/retry asset processing tasks.
   */
  async reprocessAsset(id: string, request: ReprocessAssetRequest) {
    return client.POST("/assets/{id}/reprocess", {
      params: { path: { id } },
      body: request,
    });
  },
};

// ============================================================================
// React Query Hooks
// ============================================================================

/**
 * Hook for listing assets
 */
export const useAssets = (params: ListAssetsParams) =>
  $api.useQuery("get", "/assets", {
    params: { query: params },
  });

/**
 * Hook for getting a single asset
 */
export const useAsset = (id: string, params?: GetAssetByIdParams) =>
  $api.useQuery("get", "/assets/{id}", {
    params: { path: { id }, query: params },
  });

/**
 * Hook for asset types
 */
export const useAssetTypes = () =>
  $api.useQuery("get", "/assets/types", {});

/**
 * Hook for filter options
 */
export const useFilterOptions = () =>
  $api.useQuery("get", "/assets/filter-options", {});

/**
 * Hook for liked assets
 */
export const useLikedAssets = (limit: number = 20, offset: number = 0) =>
  $api.useQuery("get", "/assets/liked", {
    params: { query: { limit, offset } },
  });

/**
 * Hook for assets by rating
 */
export const useAssetsByRating = (rating: number, limit: number = 20, offset: number = 0) =>
  $api.useQuery("get", "/assets/rating/{rating}", {
    params: { path: { rating }, query: { limit, offset } },
  });

/**
 * Hook for asset albums
 */
export const useAssetAlbums = (id: string) =>
  $api.useQuery("get", "/assets/{id}/albums", {
    params: { path: { id } },
  });
