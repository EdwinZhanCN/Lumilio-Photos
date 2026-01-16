// src/services/index.ts
// Central export point for all service modules

export * from "./uploadService";
export {
  assetService,
  type Asset,
  type AssetListResponse,
  type AssetTypesResponse,
  type AssetFilter,
  type FilenameFilter,
  type DateRange,
  type FilterAssetsRequest,
  type SearchAssetsRequest,
  type FilterOptionsResponse,
  type UpdateAssetRequest,
  type UpdateRatingRequest,
  type UpdateLikeRequest,
  type UpdateRatingAndLikeRequest,
  type UpdateDescriptionRequest,
  type ListAssetsParams,
  type GetAssetByIdParams,
  type SearchAssetsParams,
  type FilterAssetsParams,
} from "./assetsService";
export {
  albumService,
  type Album,
  type ListAlbumsResponse,
  type CreateAlbumRequest,
  type UpdateAlbumRequest,
  type AddAssetToAlbumRequest,
  type UpdateAssetPositionRequest,
  type ListAlbumsParams,
} from "./albumService";
export {
  authService,
  type User,
  type AuthResponse,
  type LoginRequest,
  type RegisterRequest,
  type RefreshTokenRequest,
} from "./authService";
export * from "./healthService";
export * from "./geoService";
export * from "./justifiedLayoutService";
export {
  statsService,
  type FocalLengthBucket,
  type FocalLengthDistributionResponse,
  type CameraLensCombination,
  type CameraLensStatsResponse,
  type TimeBucket,
  type TimeDistributionResponse,
  type TimeDistributionType,
  type HeatmapValue,
  type HeatmapResponse,
  type AvailableYearsResponse,
} from "./statsService";

// Re-export health service functions
export {
  checkHealth,
  isServerOnline,
  pollHealth,
  fetchHealth,
  HEALTH_ENDPOINT,
  MIN_HEALTH_INTERVAL_SEC,
  MAX_HEALTH_INTERVAL_SEC,
  DEFAULT_HEALTH_INTERVAL_SEC,
} from "./healthService";
