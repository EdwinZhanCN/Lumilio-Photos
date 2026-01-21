// src/services/albumService.ts

import api from "@/lib/http-commons/api.ts";
import type { AxiosRequestConfig, AxiosResponse } from "axios";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";
import type { ApiResult } from "./uploadService";
import { FilterAssetsRequest } from "./assetsService";

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
   */
  getAlbumById: async (
    id: number,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<Album>>> => {
    return api.get<ApiResult<Album>>(`/api/v1/albums/${id}`, config);
  },

  /**
   * Creates a new album for the authenticated user.
   */
  createAlbum: async (
    request: CreateAlbumRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<Album>>> => {
    return api.post<ApiResult<Album>>("/api/v1/albums", request, config);
  },

  /**
   * Updates an existing album's information.
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
   */
  getAlbumAssets: async (
    id: number,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<any>>> => {
    return api.get<ApiResult<any>>(`/api/v1/albums/${id}/assets`, config);
  },

  /**
   * Filter assets within a specific album.
   */
  filterAlbumAssets: async (
    albumId: number,
    request: FilterAssetsRequest,
    config?: AxiosRequestConfig,
  ): Promise<AxiosResponse<ApiResult<any>>> => {
    return api.post<ApiResult<any>>(
      `/api/v1/albums/${albumId}/filter`,
      request,
      config,
    );
  },

  /**
   * Adds an asset to a specific album.
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
