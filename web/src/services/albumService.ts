// src/services/albumService.ts

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
 * Album response object
 */
export type Album = Schemas["dto.GetAlbumResponseDTO"];

/**
 * List albums response with pagination
 */
export type ListAlbumsResponse = Schemas["dto.ListAlbumsResponseDTO"];

/**
 * Create album request
 */
export type CreateAlbumRequest = Schemas["dto.CreateAlbumRequestDTO"];

/**
 * Update album request
 */
export type UpdateAlbumRequest = Schemas["dto.UpdateAlbumRequestDTO"];

/**
 * Add asset to album request
 */
export type AddAssetToAlbumRequest = Schemas["dto.AddAssetToAlbumRequestDTO"];

/**
 * Update asset position request
 */
export type UpdateAssetPositionRequest =
  Schemas["dto.UpdateAssetPositionRequestDTO"];

/**
 * Message response
 */
export type MessageResponse = Schemas["dto.MessageResponseDTO"];

/**
 * List albums query parameters - extracted directly from paths
 */
export type ListAlbumsParams = NonNullable<
  Paths["/albums"]["get"]["parameters"]["query"]
>;

// ============================================================================
// Album Service
// ============================================================================

/**
 * @service AlbumService
 * @description A collection of functions for interacting with album-related API endpoints.
 */
export const albumService = {
  /**
   * Fetches a paginated list of albums for the authenticated user.
   * @param {ListAlbumsParams} [params] - Query parameters for pagination (limit, offset)
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<ListAlbumsResponse>>>} A promise that resolves to the albums list.
   */
  listAlbums: async (
    params?: ListAlbumsParams,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<ListAlbumsResponse>>> => {
    return api.get<ApiResult<ListAlbumsResponse>>("/api/v1/albums", {
      ...config,
      params,
    });
  },

  /**
   * Fetches a specific album by its ID.
   * @param {number} id - The album ID.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<Album>>>} A promise resolving to the album details.
   */
  getAlbumById: async (
    id: number,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<Album>>> => {
    return api.get<ApiResult<Album>>(`/api/v1/albums/${id}`, config);
  },

  /**
   * Creates a new album for the authenticated user.
   * @param {CreateAlbumRequest} request - The album creation data.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<Album>>>} A promise resolving to the created album.
   */
  createAlbum: async (
    request: CreateAlbumRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<Album>>> => {
    return api.post<ApiResult<Album>>("/api/v1/albums", request, config);
  },

  /**
   * Updates an existing album's information.
   * @param {number} id - The album ID.
   * @param {UpdateAlbumRequest} request - The album update data.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<Album>>>} A promise resolving to the updated album.
   */
  updateAlbum: async (
    id: number,
    request: UpdateAlbumRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<Album>>> => {
    return api.put<ApiResult<Album>>(`/api/v1/albums/${id}`, request, config);
  },

  /**
   * Deletes an album by its ID.
   * @param {number} id - The album ID.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  deleteAlbum: async (
    id: number,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.delete<ApiResult<MessageResponse>>(
      `/api/v1/albums/${id}`,
      config,
    );
  },

  /**
   * Retrieves all assets in a specific album.
   * @param {number} id - The album ID.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<any>>>} A promise resolving to the album's assets.
   */
  getAlbumAssets: async (
    id: number,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<any>>> => {
    return api.get<ApiResult<any>>(`/api/v1/albums/${id}/assets`, config);
  },

  /**
   * Adds an asset to a specific album.
   * @param {number} albumId - The album ID.
   * @param {string} assetId - The asset ID (UUID format).
   * @param {AddAssetToAlbumRequest} [request] - Optional position data.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  addAssetToAlbum: async (
    albumId: number,
    assetId: string,
    request?: AddAssetToAlbumRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.post<ApiResult<MessageResponse>>(
      `/api/v1/albums/${albumId}/assets/${assetId}`,
      request,
      config,
    );
  },

  /**
   * Removes an asset from a specific album.
   * @param {number} albumId - The album ID.
   * @param {string} assetId - The asset ID (UUID format).
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  removeAssetFromAlbum: async (
    albumId: number,
    assetId: string,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.delete<ApiResult<MessageResponse>>(
      `/api/v1/albums/${albumId}/assets/${assetId}`,
      config,
    );
  },

  /**
   * Updates the position of an asset within a specific album.
   * @param {number} albumId - The album ID.
   * @param {string} assetId - The asset ID (UUID format).
   * @param {UpdateAssetPositionRequest} request - The new position data.
   * @param {AxiosRequestConfig} [config] - Optional additional Axios request configuration.
   * @returns {Promise<AxiosResponse<ApiResult<MessageResponse>>>} A promise resolving to a success message.
   */
  updateAssetPosition: async (
    albumId: number,
    assetId: string,
    request: UpdateAssetPositionRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<MessageResponse>>> => {
    return api.put<ApiResult<MessageResponse>>(
      `/api/v1/albums/${albumId}/assets/${assetId}/position`,
      request,
      config,
    );
  },
};
