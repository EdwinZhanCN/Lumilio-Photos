import type { components, paths } from "@/lib/http-commons/schema.d.ts";

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
export type UpdateRatingAndLikeRequest =
  Schemas["dto.UpdateRatingAndLikeRequestDTO"];
export type UpdateDescriptionRequest =
  Schemas["dto.UpdateDescriptionRequestDTO"];
export type ReprocessAssetRequest = Schemas["dto.ReprocessAssetRequestDTO"];
export type ReprocessAssetResponse = Schemas["dto.ReprocessAssetResponseDTO"];

export type ListAssetsParams =
  NonNullable<Paths["/api/v1/assets"]["get"]["parameters"]["query"]>;
export type GetAssetByIdParams =
  NonNullable<Paths["/api/v1/assets/{id}"]["get"]["parameters"]["query"]>;
export type SearchAssetsParams = SearchAssetsRequest;
export type FilterAssetsParams = FilterAssetsRequest;
