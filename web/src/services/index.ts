// src/services/index.ts
// Central export point for all service modules

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
