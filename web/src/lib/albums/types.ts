import type { components, paths } from "@/lib/http-commons/schema.d.ts";

type Schemas = components["schemas"];
type Paths = paths;

export type ApiResult<T = unknown> = Omit<Schemas["api.Result"], "data"> & {
  data?: T;
};

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
