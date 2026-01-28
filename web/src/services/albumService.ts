// src/services/albumService.ts

import client from "@/lib/http-commons/client";
import type { components, paths } from "@/lib/http-commons/schema.d.ts";

// ============================================================================
// Type Aliases from Generated Schema
// ============================================================================

type Schemas = components["schemas"];
type Paths = paths;

export type Album = Schemas["dto.GetAlbumResponseDTO"];
export type ListAlbumsResponse = Schemas["dto.ListAlbumsResponseDTO"];
export type CreateAlbumRequest = Schemas["dto.CreateAlbumRequestDTO"];
export type UpdateAlbumRequest = Schemas["dto.UpdateAlbumRequestDTO"];
export type AddAssetToAlbumRequest = Schemas["dto.AddAssetToAlbumRequestDTO"];
export type UpdateAssetPositionRequest = Schemas["dto.UpdateAssetPositionRequestDTO"];
export type MessageResponse = Schemas["dto.MessageResponseDTO"];
export type FilterAssetsRequest = Schemas["dto.FilterAssetsRequestDTO"];

export type ListAlbumsParams = NonNullable<
  Paths["/api/v1/albums"]["get"]["parameters"]["query"]
>;

// ============================================================================
// Album Service (Direct API calls)
// ============================================================================

export const albumService = {
  /**
   * Fetches a paginated list of albums for the authenticated user.
   */
  async listAlbums(params?: ListAlbumsParams) {
    return client.GET("/api/v1/albums", {
      params: { query: params },
    });
  },

  /**
   * Fetches a specific album by its ID.
   */
  async getAlbumById(id: number) {
    return client.GET("/api/v1/albums/{id}", {
      params: { path: { id } },
    });
  },

  /**
   * Creates a new album for the authenticated user.
   */
  async createAlbum(request: CreateAlbumRequest) {
    return client.POST("/api/v1/albums", {
      body: request,
    });
  },

  /**
   * Updates an existing album's information.
   */
  async updateAlbum(id: number, request: UpdateAlbumRequest) {
    return client.PUT("/api/v1/albums/{id}", {
      params: { path: { id } },
      body: request,
    });
  },

  /**
   * Deletes an album by its ID.
   */
  async deleteAlbum(id: number) {
    return client.DELETE("/api/v1/albums/{id}", {
      params: { path: { id } },
    });
  },

  /**
   * Retrieves all assets in a specific album.
   */
  async getAlbumAssets(id: number) {
    return client.GET("/api/v1/albums/{id}/assets", {
      params: { path: { id } },
    });
  },

  /**
   * Filter assets within a specific album.
   */
  async filterAlbumAssets(albumId: number, request: FilterAssetsRequest) {
    return client.POST("/api/v1/albums/{id}/filter", {
      params: { path: { id: albumId } },
      body: request,
    });
  },

  /**
   * Adds an asset to a specific album.
   */
  async addAssetToAlbum(albumId: number, assetId: string, request?: AddAssetToAlbumRequest) {
    return client.POST("/api/v1/albums/{id}/assets/{assetId}", {
      params: { path: { id: albumId, assetId } },
      body: request,
    });
  },

  /**
   * Removes an asset from a specific album.
   */
  async removeAssetFromAlbum(albumId: number, assetId: string) {
    return client.DELETE("/api/v1/albums/{id}/assets/{assetId}", {
      params: { path: { id: albumId, assetId } },
    });
  },

  /**
   * Updates the position of an asset within a specific album.
   */
  async updateAssetPosition(albumId: number, assetId: string, request: UpdateAssetPositionRequest) {
    return client.PUT("/api/v1/albums/{id}/assets/{assetId}/position", {
      params: { path: { id: albumId, assetId } },
      body: request,
    });
  },
};

