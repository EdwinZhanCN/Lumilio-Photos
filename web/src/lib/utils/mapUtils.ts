import type { PhotoLocation } from "@/components/MapComponent";
import { autoConvertCoordinates, type Coordinate } from "@/lib/geo/coordinateConversion";
import type { Asset } from "@/lib/http-commons";
import { isPhotoMetadata } from "@/lib/http-commons";

/**
 * Convert an Asset with GPS coordinates to PhotoLocation format
 * Automatically converts coordinates for Gaode Map if in China region
 */
export const assetToPhotoLocation = (
  asset: Asset,
  useGaodeMap: boolean = false,
): PhotoLocation | null => {
  const metadata = asset.specific_metadata;

  // Check if metadata is photo metadata with GPS data
  if (!isPhotoMetadata(asset.type, metadata)) {
    return null;
  }

  const latitude = metadata.gps_latitude;
  const longitude = metadata.gps_longitude;

  if (typeof latitude !== "number" || typeof longitude !== "number") {
    return null;
  }

  // Convert coordinates if using Gaode Map and location is in China
  const convertedCoords = autoConvertCoordinates(longitude, latitude, useGaodeMap);

  return {
    id: asset.asset_id || `asset-${Date.now()}`,
    position: [convertedCoords.latitude, convertedCoords.longitude],
    title: asset.original_filename || "Photo",
    description: metadata.description,
    asset: asset,
  };
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
