import type { PhotoLocation } from "@/components/MapComponent";
import {
  autoConvertCoordinates,
  needsCoordinateConversion,
  type Coordinate,
} from "./coordinateConversion";

/**
 * Convert an Asset with GPS coordinates to PhotoLocation format
 * Automatically converts coordinates for Gaode Map if in China region
 */
export const assetToPhotoLocation = (
  asset: Asset,
  useGaodeMap: boolean = false,
): PhotoLocation | null => {
  const metadata = asset.specific_metadata;

  if (!metadata?.gps_latitude || !metadata?.gps_longitude) {
    return null;
  }

  // Convert coordinates if using Gaode Map and location is in China
  const convertedCoords = autoConvertCoordinates(
    metadata.gps_longitude,
    metadata.gps_latitude,
    useGaodeMap,
  );

  return {
    id: asset.asset_id || `asset-${Date.now()}`,
    position: [convertedCoords.latitude, convertedCoords.longitude],
    title: asset.original_filename || "Photo",
    description: metadata.description,
    asset: asset,
  };
};

/**
 * Convert multiple assets to photo locations, filtering out those without GPS data
 * Automatically converts coordinates for Gaode Map if in China region
 */
export const assetsToPhotoLocations = (
  assets: Asset[],
  useGaodeMap: boolean = false,
): PhotoLocation[] => {
  return assets
    .map((asset) => assetToPhotoLocation(asset, useGaodeMap))
    .filter((location): location is PhotoLocation => location !== null);
};

/**
 * Get thumbnail URL for an asset
 */
export const getAssetThumbnailUrl = (
  assetId: string,
  size: "small" | "medium" | "large" = "small",
): string => {
  return `/api/assets/${assetId}/thumbnail?size=${size}`;
};

/**
 * Calculate the center point of multiple photo locations
 */
export const calculateCenter = (
  locations: PhotoLocation[],
): [number, number] => {
  if (locations.length === 0) {
    return [0, 0];
  }

  if (locations.length === 1) {
    const pos = locations[0].position as [number, number];
    return pos;
  }

  const sum = locations.reduce(
    (acc, location) => {
      const pos = location.position as [number, number];
      return [acc[0] + pos[0], acc[1] + pos[1]];
    },
    [0, 0],
  );

  return [sum[0] / locations.length, sum[1] / locations.length];
};

/**
 * Format GPS coordinates for display
 */
export const formatGPSCoordinates = (
  latitude: number,
  longitude: number,
  precision: number = 6,
): string => {
  const lat = latitude.toFixed(precision);
  const lng = longitude.toFixed(precision);
  const latDir = latitude >= 0 ? "N" : "S";
  const lngDir = longitude >= 0 ? "E" : "W";

  return `${Math.abs(parseFloat(lat))}°${latDir}, ${Math.abs(parseFloat(lng))}°${lngDir}`;
};

/**
 * Check if coordinates are within China (approximate bounds)
 * Uses the same logic as coordinate conversion utility
 */
export const isInChina = (latitude: number, longitude: number): boolean => {
  return needsCoordinateConversion(longitude, latitude);
};

/**
 * Convert GPS coordinates to appropriate map coordinates based on region
 */
export const convertCoordinatesForMap = (
  longitude: number,
  latitude: number,
  useGaodeMap: boolean,
): Coordinate => {
  return autoConvertCoordinates(longitude, latitude, useGaodeMap);
};

/**
 * Convert PhotoLocation coordinates for specific map provider
 */
export const convertPhotoLocationCoordinates = (
  location: PhotoLocation,
  useGaodeMap: boolean,
): PhotoLocation => {
  const position = location.position as [number, number];
  const converted = convertCoordinatesForMap(
    position[1],
    position[0],
    useGaodeMap,
  );

  return {
    ...location,
    position: [converted.latitude, converted.longitude],
  };
};

/**
 * Get appropriate zoom level based on the spread of locations
 */
export const getOptimalZoom = (locations: PhotoLocation[]): number => {
  if (locations.length <= 1) {
    return 13;
  }

  // Calculate the bounding box
  const lats = locations.map((loc) => (loc.position as [number, number])[0]);
  const lngs = locations.map((loc) => (loc.position as [number, number])[1]);

  const latSpread = Math.max(...lats) - Math.min(...lats);
  const lngSpread = Math.max(...lngs) - Math.min(...lngs);
  const maxSpread = Math.max(latSpread, lngSpread);

  // Rough zoom level calculation based on spread
  if (maxSpread > 10) return 4;
  if (maxSpread > 5) return 6;
  if (maxSpread > 2) return 8;
  if (maxSpread > 1) return 10;
  if (maxSpread > 0.5) return 12;
  if (maxSpread > 0.1) return 14;
  return 16;
};
